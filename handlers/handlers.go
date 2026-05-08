package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"subscription-manager/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	notify "subscription-manager/notify"
)

type Handler struct {
	db *sql.DB
}

var jwtSecret []byte

func InitJWTSecret(secret string) {
	jwtSecret = []byte(secret)
}

func New(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func generateToken(exp int64) (string, error) {
	if len(jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": exp,
		"iat": time.Now().Unix(),
	})
	return token.SignedString(jwtSecret)
}

func validateToken(tokenStr string) bool {
	if len(jwtSecret) == 0 {
		return false
	}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	return err == nil && token.Valid
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" || !validateToken(tokenStr) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}
		c.Next()
	}
}

var loginAttempts = make(map[string]int)
var lastAttemptTime = make(map[string]time.Time)
var rateLimitMutex sync.Mutex

func StartRateLimitCleaner() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rateLimitMutex.Lock()
			cutoff := time.Now().Add(-15 * time.Minute)
			for ip, t := range lastAttemptTime {
				if t.Before(cutoff) {
					delete(loginAttempts, ip)
					delete(lastAttemptTime, ip)
				}
			}
			rateLimitMutex.Unlock()
		}
	}()
}

func (h *Handler) Login(c *gin.Context) {
	clientIP := c.ClientIP()

	rateLimitMutex.Lock()
	attempts := loginAttempts[clientIP]
	lastTime := lastAttemptTime[clientIP]

	if time.Since(lastTime) > 15*time.Minute {
		loginAttempts[clientIP] = 0
		attempts = 0
	}

	if attempts >= 5 {
		rateLimitMutex.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "尝试次数过多，请15分钟后重试"})
		return
	}
	rateLimitMutex.Unlock()

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	storedHash, _ := notify.GetPasswordHash()
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)); err != nil {
		rateLimitMutex.Lock()
		loginAttempts[clientIP]++
		lastAttemptTime[clientIP] = time.Now()
		rateLimitMutex.Unlock()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	rateLimitMutex.Lock()
	delete(loginAttempts, clientIP)
	delete(lastAttemptTime, clientIP)
	rateLimitMutex.Unlock()

	exp := time.Now().Add(24 * time.Hour).Unix()
	tokenStr, err := generateToken(exp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenStr})
}

func (h *Handler) GetStats(c *gin.Context) {
	stats, err := models.GetStats(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) GetSubscriptions(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "10")
	name := c.Query("name")
	subType := c.Query("type")
	expireDate := c.Query("expire_date")
	status := c.Query("status")

	pageNum := 1
	pageSizeNum := 10
	fmt.Sscanf(page, "%d", &pageNum)
	fmt.Sscanf(pageSize, "%d", &pageSizeNum)
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSizeNum < 1 || pageSizeNum > 100 {
		pageSizeNum = 10
	}

	var subs []models.Subscription
	var total int64
	var err error

	typeInt := -1
	if subType != "" {
		fmt.Sscanf(subType, "%d", &typeInt)
	}

	if name == "" && typeInt == -1 && expireDate == "" && status == "" {
		subs, total, err = models.GetAllPaged(h.db, pageNum, pageSizeNum)
	} else {
		subs, total, err = models.SearchPaged(h.db, name, typeInt, expireDate, status, pageNum, pageSizeNum)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	for i := range subs {
		subs[i].Status = models.ComputeStatus(subs[i].ExpireDate)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  subs,
		"total": total,
		"page":  pageNum,
		"size":  pageSizeNum,
	})
}

func (h *Handler) CreateSubscription(c *gin.Context) {
	var req struct {
		Name       string  `json:"name"`
		Remark     string  `json:"remark"`
		Type       int     `json:"type"`
		Period     int     `json:"period"`
		Price      float64 `json:"price"`
		StartDate  string  `json:"start_date"`
		ExpireDate string  `json:"expire_date"`
		NotifyDays int     `json:"notify_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Price < 0 || req.Price > 1000000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "价格必须在 0-1000000 范围内"})
		return
	}
	validPeriods := map[int]bool{1: true, 3: true, 6: true, 12: true, 24: true, 36: true}
	if !validPeriods[req.Period] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订阅周期无效，必须是 1/3/6/12/24/36 月"})
		return
	}
	if len(strings.TrimSpace(req.Name)) == 0 || len(req.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称必须为 1-100 个字符"})
		return
	}
	if req.NotifyDays < 0 || req.NotifyDays > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "提醒天数必须在 0-365 范围内"})
		return
	}
	if utf8.RuneCountInString(req.Remark) > 20 {
		runes := []rune(req.Remark)
		req.Remark = string(runes[:20])
	}
	startDate, _ := time.Parse("2006-01-02", req.StartDate)
	expireDate, _ := time.Parse("2006-01-02", req.ExpireDate)

	sub := models.Subscription{
		Name:       req.Name,
		Remark:     req.Remark,
		Type:       req.Type,
		Period:     req.Period,
		Price:      req.Price,
		StartDate:  startDate,
		ExpireDate: expireDate,
		NotifyDays: req.NotifyDays,
		IsActive:   true,
		Notified:   false,
	}

	sub.Status = models.ComputeStatus(sub.ExpireDate)

	if err := models.Create(h.db, &sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *Handler) UpdateSubscription(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name       string  `json:"name"`
		Remark     string  `json:"remark"`
		Type       int     `json:"type"`
		Period     int     `json:"period"`
		Price      float64 `json:"price"`
		StartDate  string  `json:"start_date"`
		ExpireDate string  `json:"expire_date"`
		NotifyDays int     `json:"notify_days"`
		IsActive   bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Price < 0 || req.Price > 1000000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "价格必须在 0-1000000 范围内"})
		return
	}
	validPeriods := map[int]bool{1: true, 3: true, 6: true, 12: true, 24: true, 36: true}
	if !validPeriods[req.Period] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订阅周期无效，必须是 1/3/6/12/24/36 月"})
		return
	}
	if len(strings.TrimSpace(req.Name)) == 0 || len(req.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称必须为 1-100 个字符"})
		return
	}
	if req.NotifyDays < 0 || req.NotifyDays > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "提醒天数必须在 0-365 范围内"})
		return
	}
	if utf8.RuneCountInString(req.Remark) > 20 {
		runes := []rune(req.Remark)
		req.Remark = string(runes[:20])
	}
	subID, _ := parseUint(id)
	startDate, _ := time.Parse("2006-01-02", req.StartDate)
	expireDate, _ := time.Parse("2006-01-02", req.ExpireDate)

	sub := models.Subscription{
		ID:         subID,
		Name:       req.Name,
		Remark:     req.Remark,
		Type:       req.Type,
		Period:     req.Period,
		Price:      req.Price,
		StartDate:  startDate,
		ExpireDate: expireDate,
		NotifyDays: req.NotifyDays,
		IsActive:   req.IsActive,
	}

	sub.Status = models.ComputeStatus(sub.ExpireDate)

	if err := models.Update(h.db, &sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *Handler) DeleteSubscription(c *gin.Context) {
	id := c.Param("id")
	uid, _ := parseUint(id)
	if err := models.Delete(h.db, uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *Handler) ToggleSubscription(c *gin.Context) {
	id := c.Param("id")
	uid, _ := parseUint(id)
	sub, err := models.GetByID(h.db, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}
	sub.IsActive = !sub.IsActive
	if err := models.Update(h.db, sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *Handler) RenewSubscription(c *gin.Context) {
	id := c.Param("id")
	uid, _ := parseUint(id)

	var req struct {
		Period int `json:"period"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	validPeriods := map[int]bool{1: true, 3: true, 6: true, 12: true, 24: true, 36: true}
	if !validPeriods[req.Period] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "续订周期无效，必须是 1/3/6/12/24/36 月"})
		return
	}

	sub, err := models.GetByID(h.db, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅不存在"})
		return
	}

	// 计算新到期日期：从当前到期日或今天开始，加上续订周期
	newStart := sub.ExpireDate
	if newStart.Before(time.Now()) {
		newStart = time.Now()
	}
	sub.StartDate = newStart
	sub.ExpireDate = newStart.AddDate(0, req.Period, 0)
	sub.Notified = false // 重置通知状态
	sub.Status = models.ComputeStatus(sub.ExpireDate)

	if err := models.Update(h.db, sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func parseUint(s string) (uint, error) {
	var n uint
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func (h *Handler) GetNotificationLogs(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")

	pageNum := 1
	pageSizeNum := 20
	fmt.Sscanf(page, "%d", &pageNum)
	fmt.Sscanf(pageSize, "%d", &pageSizeNum)
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSizeNum < 1 || pageSizeNum > 100 {
		pageSizeNum = 20
	}

	logs, total, err := models.GetNotificationLogs(h.db, pageNum, pageSizeNum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  logs,
		"total": total,
		"page":  pageNum,
		"size":  pageSizeNum,
	})
}
