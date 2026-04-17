package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subscription-manager/config"
	"subscription-manager/handlers"
	"subscription-manager/models"
	"subscription-manager/notify"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 检查是否是 init 命令
	initCmd := flag.Bool("init", false, "初始化配置")
	flag.Parse()

	if *initCmd {
		fmt.Println("请使用独立命令: go run ./cmd/init")
		return
	}

	// 正常启动服务
	startServer()
}

func startServer() {
	// 加载配置
	notify.LoadConfig()
	if err := notify.InitConfig(); err != nil {
		log.Fatalf("配置初始化失败: %v\n请运行: go run ./cmd/init", err)
	}

	// 初始化 JWT Secret
	jwtSecret, _ := notify.GetJWTSecret()
	handlers.InitJWTSecret(jwtSecret)

	// 初始化数据库
	db, err := config.InitDB()
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 自动建表
	if err := models.AutoMigrate(db); err != nil {
		log.Fatalf("建表失败: %v", err)
	}
	if err := models.AutoMigrateNotificationLogs(db); err != nil {
		log.Fatalf("通知日志表建表失败: %v", err)
	}

	// 启动定时检查任务
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				checkAndNotify(db)
			case <-stop:
				return
			}
		}
	}()

	// 启动时检查一次
	go checkAndNotify(db)

	// 启动 Gin 服务器
	r := gin.Default()
	setupRoutes(r, db)
	srv := &http.Server{Addr: ":8080", Handler: r}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待信号以优雅关闭
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	close(stop)
	srv.Shutdown(context.Background())
}

func checkAndNotify(db *sql.DB) {
	// 逻辑1: 提前通知N天的订阅
	expiringSubs, err := models.GetExpiring(db)
	if err != nil {
		log.Printf("获取即将过期的订阅失败: %v", err)
		return
	}

	// 逻辑2: 已过期但未通知的订阅
	expiredSubs, err := models.GetExpired(db)
	if err != nil {
		log.Printf("获取已过期订阅失败: %v", err)
		return
	}

	// 合并两个列表
	allSubs := append(expiringSubs, expiredSubs...)
	if len(allSubs) == 0 {
		return
	}

	notifyAndMarkNotified(db, allSubs)
}

func notifyAndMarkNotified(db *sql.DB, subs []models.Subscription) {
	now := time.Now()

	// 批量发送微信通知
	if err := notify.SendWeChatBatchNotification(subs); err != nil {
		log.Printf("发送微信提醒失败: %v", err)
	} else {
		logNotificationsBatch(db, subs, "wechat", now)
	}

	// 批量发送邮件通知
	if err := notify.SendEmailBatchNotification(subs); err != nil {
		log.Printf("发送邮件提醒失败: %v", err)
	} else {
		logNotificationsBatch(db, subs, "email", now)
	}

	// 标记已通知
	for _, sub := range subs {
		models.MarkNotified(db, sub.ID)
	}
}

func logNotificationsBatch(db *sql.DB, subs []models.Subscription, channel string, sentAt time.Time) {
	logs := make([]models.NotificationLog, len(subs))
	for i, sub := range subs {
		logs[i] = models.NotificationLog{
			SubscriptionID:   sub.ID,
			SubscriptionName: sub.Name,
			Channel:          channel,
			SentAt:           sentAt,
		}
	}
	if err := models.CreateNotificationLogsBatch(db, logs); err != nil {
		log.Printf("批量记录通知日志失败: %v", err)
	}
}

func setupRoutes(r *gin.Engine, db *sql.DB) {
	h := handlers.New(db)

	api := r.Group("/api")
	{
		api.POST("/login", h.Login)

		protected := api.Group("")
		protected.Use(handlers.AuthMiddleware())
		{
			protected.GET("/stats", h.GetStats)
			protected.GET("/subscriptions", h.GetSubscriptions)
			protected.POST("/subscriptions", h.CreateSubscription)
			protected.PUT("/subscriptions/:id", h.UpdateSubscription)
			protected.PUT("/subscriptions/:id/toggle", h.ToggleSubscription)
			protected.PUT("/subscriptions/:id/renew", h.RenewSubscription)
			protected.DELETE("/subscriptions/:id", h.DeleteSubscription)
			protected.GET("/notifications", h.GetNotificationLogs)
		}
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
