package models

import (
	"database/sql"
	"math"
	"time"
)

const dateFmt = "2006-01-02"
const dateTimeFmt = "2006-01-02 15:04:05"

const (
	StatusActive   = "active"
	StatusExpired  = "expired"
	StatusExpiring = "expiring"
)

// ComputeStatus returns subscription status based on days until expiry.
func ComputeStatus(expireDate time.Time) string {
	days := int(time.Until(expireDate).Hours() / 24)
	if days < 0 {
		return StatusExpired
	}
	if days <= 7 {
		return StatusExpiring
	}
	return StatusActive
}

func parseDates(s *Subscription, start, expire, created, updated string) {
	s.StartDate, _ = time.Parse(dateFmt, start)
	s.ExpireDate, _ = time.Parse(dateFmt, expire)
	s.CreatedAt, _ = time.Parse(dateTimeFmt, created)
	s.UpdatedAt, _ = time.Parse(dateTimeFmt, updated)
}

type Subscription struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Remark     string    `json:"remark"` // 备注，最多20字
	Type       int       `json:"type"`   // 0=国内, 1=国外
	Period     int       `json:"period"` // 月数：1,3,6,12,24,36
	Price      float64   `json:"price"`
	StartDate  time.Time `json:"start_date"`
	ExpireDate time.Time `json:"expire_date"`
	NotifyDays int       `json:"notify_days"` // 提前N天通知
	Status     string    `json:"status"`      // active, expired
	IsActive   bool      `json:"is_active"`
	Notified   bool      `json:"notified"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func AutoMigrate(db *sql.DB) error {
	// 创建表
	query := `
	CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		remark TEXT,
		type INTEGER NOT NULL DEFAULT 0,
		period INTEGER NOT NULL DEFAULT 1,
		price REAL NOT NULL,
		start_date TEXT NOT NULL,
		expire_date TEXT NOT NULL,
		notify_days INTEGER NOT NULL DEFAULT 3,
		status TEXT NOT NULL DEFAULT 'active',
		is_active INTEGER NOT NULL DEFAULT 1,
		notified INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`
	if _, err := db.Exec(query); err != nil {
		return err
	}

	return nil
}

func GetAll(db *sql.DB) ([]Subscription, error) {
	query := `SELECT id, name, remark, type, period, price, start_date, expire_date, notify_days, status, is_active, notified, created_at, updated_at FROM subscriptions ORDER BY expire_date ASC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		var startDate, expireDate, createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Remark, &s.Type, &s.Period, &s.Price, &startDate, &expireDate, &s.NotifyDays, &s.Status, &s.IsActive, &s.Notified, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		parseDates(&s, startDate, expireDate, createdAt, updatedAt)
		subs = append(subs, s)
	}
	return subs, nil
}

func GetByID(db *sql.DB, id uint) (*Subscription, error) {
	query := `SELECT id, name, remark, type, period, price, start_date, expire_date, notify_days, status, is_active, notified, created_at, updated_at FROM subscriptions WHERE id = ?`
	var s Subscription
	var startDate, expireDate, createdAt, updatedAt string
	err := db.QueryRow(query, id).Scan(&s.ID, &s.Name, &s.Remark, &s.Type, &s.Period, &s.Price, &startDate, &expireDate, &s.NotifyDays, &s.Status, &s.IsActive, &s.Notified, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	parseDates(&s, startDate, expireDate, createdAt, updatedAt)
	return &s, nil
}

func Create(db *sql.DB, s *Subscription) error {
	now := time.Now().Format(dateTimeFmt)
	start := s.StartDate.Format(dateFmt)
	expire := s.ExpireDate.Format(dateFmt)
	if s.Period == 0 {
		s.Period = 1
	}

	query := `INSERT INTO subscriptions (name, remark, type, period, price, start_date, expire_date, notify_days, status, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`
	result, err := db.Exec(query, s.Name, s.Remark, s.Type, s.Period, s.Price, start, expire, s.NotifyDays, s.Status, now, now)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	s.ID = uint(id)
	return nil
}

func Update(db *sql.DB, s *Subscription) error {
	now := time.Now().Format(dateTimeFmt)
	start := s.StartDate.Format(dateFmt)
	expire := s.ExpireDate.Format(dateFmt)

	query := `UPDATE subscriptions SET name=?, remark=?, type=?, period=?, price=?, start_date=?, expire_date=?, notify_days=?, status=?, is_active=?, updated_at=? WHERE id=?`
	_, err := db.Exec(query, s.Name, s.Remark, s.Type, s.Period, s.Price, start, expire, s.NotifyDays, s.Status, s.IsActive, now, s.ID)
	return err
}

func Delete(db *sql.DB, id uint) error {
	query := `DELETE FROM subscriptions WHERE id = ?`
	_, err := db.Exec(query, id)
	return err
}

func Search(db *sql.DB, name string, subType string, expireDate string, status string) ([]Subscription, error) {
	query := `SELECT id, name, type, period, price, start_date, expire_date, status, is_active, notified, created_at, updated_at FROM subscriptions WHERE 1=1`
	args := []any{}

	if name != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+name+"%")
	}
	if subType != "" {
		query += " AND type = ?"
		args = append(args, subType)
	}
	if expireDate != "" {
		query += " AND expire_date = ?"
		args = append(args, expireDate)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY expire_date ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		var startDate, expireDateStr, createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Type, &s.Period, &s.Price, &startDate, &expireDateStr, &s.Status, &s.IsActive, &s.Notified, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		parseDates(&s, startDate, expireDateStr, createdAt, updatedAt)
		subs = append(subs, s)
	}
	return subs, nil
}

func GetExpiring(db *sql.DB) ([]Subscription, error) {
	// 查询即将到期的订阅（根据每个订阅的 notify_days）
	query := `SELECT id, name, type, period, price, start_date, expire_date, notify_days, status, is_active, notified, created_at, updated_at FROM subscriptions WHERE is_active = 1 AND expire_date <= date('now', 'localtime', '+' || notify_days || ' days') AND expire_date >= date('now', 'localtime') AND notified = 0`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		var startDate, expireDateStr, createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Type, &s.Period, &s.Price, &startDate, &expireDateStr, &s.NotifyDays, &s.Status, &s.IsActive, &s.Notified, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		parseDates(&s, startDate, expireDateStr, createdAt, updatedAt)
		subs = append(subs, s)
	}
	return subs, nil
}

func GetExpired(db *sql.DB) ([]Subscription, error) {
	// 查询已过期但未通知的订阅
	query := `SELECT id, name, type, period, price, start_date, expire_date, notify_days, status, is_active, notified, created_at, updated_at FROM subscriptions WHERE is_active = 1 AND expire_date < date('now', 'localtime') AND notified = 0`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		var startDate, expireDateStr, createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Type, &s.Period, &s.Price, &startDate, &expireDateStr, &s.NotifyDays, &s.Status, &s.IsActive, &s.Notified, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		parseDates(&s, startDate, expireDateStr, createdAt, updatedAt)
		subs = append(subs, s)
	}
	return subs, nil
}

func MarkNotified(db *sql.DB, id uint) error {
	query := `UPDATE subscriptions SET notified = 1 WHERE id = ?`
	_, err := db.Exec(query, id)
	return err
}

// Stats 统计信息
type Stats struct {
	ActiveCount  int     `json:"active_count"`
	MonthlyTotal float64 `json:"monthly_total"`
}

// GetStats 获取统计信息
func GetStats(db *sql.DB) (*Stats, error) {
	stats := &Stats{}
	var monthlyTotal float64
	err := db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(price / period), 0)
		FROM subscriptions WHERE is_active = 1 AND period > 0
	`).Scan(&stats.ActiveCount, &monthlyTotal)
	if err != nil {
		return nil, err
	}
	stats.MonthlyTotal = math.Round(monthlyTotal*10) / 10
	return stats, nil
}

// GetAllPaged 获取分页列表
func GetAllPaged(db *sql.DB, page, pageSize int) ([]Subscription, int64, error) {
	// 获取总数
	var total int64
	err := db.QueryRow("SELECT COUNT(*) FROM subscriptions").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `SELECT id, name, remark, type, period, price, start_date, expire_date, notify_days, status, is_active, notified, created_at, updated_at FROM subscriptions ORDER BY expire_date ASC LIMIT ? OFFSET ?`
	rows, err := db.Query(query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		var startDate, expireDate, createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Remark, &s.Type, &s.Period, &s.Price, &startDate, &expireDate, &s.NotifyDays, &s.Status, &s.IsActive, &s.Notified, &createdAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		parseDates(&s, startDate, expireDate, createdAt, updatedAt)
		subs = append(subs, s)
	}
	return subs, total, nil
}

// NotificationLog 通知日志
type NotificationLog struct {
	ID               uint      `json:"id"`
	SubscriptionID   uint      `json:"subscription_id"`
	SubscriptionName string    `json:"subscription_name"`
	Channel          string    `json:"channel"` // wechat, email
	Content          string    `json:"content"`
	SentAt           time.Time `json:"sent_at"`
}

func AutoMigrateNotificationLogs(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS notification_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subscription_id INTEGER NOT NULL,
		subscription_name TEXT NOT NULL,
		channel TEXT NOT NULL,
		content TEXT,
		sent_at TEXT NOT NULL
	);
	`
	_, err := db.Exec(query)
	return err
}

func CreateNotificationLog(db *sql.DB, log *NotificationLog) error {
	query := `INSERT INTO notification_logs (subscription_id, subscription_name, channel, content, sent_at) VALUES (?, ?, ?, ?, ?)`
	_, err := db.Exec(query, log.SubscriptionID, log.SubscriptionName, log.Channel, log.Content, log.SentAt.Format(dateTimeFmt))
	return err
}

func CreateNotificationLogsBatch(db *sql.DB, logs []NotificationLog) error {
	if len(logs) == 0 {
		return nil
	}
	query := `INSERT INTO notification_logs (subscription_id, subscription_name, channel, content, sent_at) VALUES `
	args := make([]any, 0, len(logs)*5)
	for _, log := range logs {
		query += `(?, ?, ?, ?, ?),`
		args = append(args, log.SubscriptionID, log.SubscriptionName, log.Channel, log.Content, log.SentAt.Format(dateTimeFmt))
	}
	query = query[:len(query)-1] // remove trailing comma
	_, err := db.Exec(query, args...)
	return err
}

func GetNotificationLogs(db *sql.DB, page, pageSize int) ([]NotificationLog, int64, error) {
	var total int64
	err := db.QueryRow("SELECT COUNT(*) FROM notification_logs").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `SELECT id, subscription_id, subscription_name, channel, content, sent_at FROM notification_logs ORDER BY sent_at DESC LIMIT ? OFFSET ?`
	rows, err := db.Query(query, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []NotificationLog
	for rows.Next() {
		var l NotificationLog
		var sentAt string
		if err := rows.Scan(&l.ID, &l.SubscriptionID, &l.SubscriptionName, &l.Channel, &l.Content, &sentAt); err != nil {
			return nil, 0, err
		}
		l.SentAt, _ = time.Parse(dateTimeFmt, sentAt)
		logs = append(logs, l)
	}
	return logs, total, nil
}

// SearchPaged 搜索分页
func SearchPaged(db *sql.DB, name string, subType int, expireDate, status string, page, pageSize int) ([]Subscription, int64, error) {
	query := `SELECT id, name, remark, type, period, price, start_date, expire_date, notify_days, status, is_active, notified, created_at, updated_at FROM subscriptions WHERE 1=1`
	args := []any{}

	if name != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+name+"%")
	}
	if subType >= 0 {
		query += " AND type = ?"
		args = append(args, subType)
	}
	if expireDate != "" {
		query += " AND expire_date = ?"
		args = append(args, expireDate)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	// 获取总数
	countQuery := "SELECT COUNT(*) FROM subscriptions WHERE 1=1"
	countArgs := []any{}
	if name != "" {
		countQuery += " AND name LIKE ?"
		countArgs = append(countArgs, "%"+name+"%")
	}
	if subType >= 0 {
		countQuery += " AND type = ?"
		countArgs = append(countArgs, subType)
	}
	if expireDate != "" {
		countQuery += " AND expire_date = ?"
		countArgs = append(countArgs, expireDate)
	}
	if status != "" {
		countQuery += " AND status = ?"
		countArgs = append(countArgs, status)
	}

	var total int64
	err := db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query += " ORDER BY expire_date ASC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		var startDate, expireDateStr, createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Remark, &s.Type, &s.Period, &s.Price, &startDate, &expireDateStr, &s.NotifyDays, &s.Status, &s.IsActive, &s.Notified, &createdAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		parseDates(&s, startDate, expireDateStr, createdAt, updatedAt)
		subs = append(subs, s)
	}
	return subs, total, nil
}
