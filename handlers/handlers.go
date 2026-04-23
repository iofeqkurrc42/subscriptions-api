package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"subscription-manager/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	notify "subscription-manager/notify"
)

type Handler struct {
	db *sql.DB
}

var jwtSecretData []byte

func InitJWTSecret(secret string) {
	jwtSecretData = []byte(secret)
}

func New(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func generateToken(exp int64) string {
	if len(jwtSecretData) == 0 {
		return ""
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	signature := hmac.New(sha256.New, jwtSecretData)
	signature.Write([]byte(header + "." + payload))
	sig := base64.RawURLEncoding.EncodeToString(signature.Sum(nil))
	return header + "." + payload + "." + sig
}

func validateToken(tokenStr string) bool {
	if len(jwtSecretData) == 0 {
		return false
	}
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return false
	}
	signature := hmac.New(sha256.New, jwtSecretData)
	signature.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(signature.Sum(nil))
	if parts[2] != expectedSig {
		return false
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	if string(headerBytes) != `{"alg":"HS256","typ":"JWT"}` {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var payload struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return false
	}
	return payload.Exp > time.Now().Unix()
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			tokenStr = c.Query("token")
		}
		if tokenStr == "" || !validateToken(tokenStr) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	storedHash, _ := notify.GetPasswordHash()
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	exp := time.Now().Add(24 * time.Hour).Unix()
	tokenStr := generateToken(exp)

	c.JSON(http.StatusOK, gin.H{"token": tokenStr})
}

func (h *Handler) GetStats(c *gin.Context) {
	stats, err := models.GetStats(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *Handler) DeleteSubscription(c *gin.Context) {
	id := c.Param("id")
	uid, _ := parseUint(id)
	if err := models.Delete(h.db, uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *Handler) ToggleSubscription(c *gin.Context) {
	id := c.Param("id")
	uid, _ := parseUint(id)
	sub, err := models.GetByID(h.db, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sub.IsActive = !sub.IsActive
	if err := models.Update(h.db, sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误", "result:": err.Error()})
		return
	}

	sub, err := models.GetByID(h.db, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	logs, total, err := models.GetNotificationLogs(h.db, pageNum, pageSizeNum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  logs,
		"total": total,
		"page":  pageNum,
		"size":  pageSizeNum,
	})
}
