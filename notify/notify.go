package notify

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"subscription-manager/models"
)

const dateFmt = "2006-01-02"

// DaysUntil returns days remaining until t (negative if past).
func DaysUntil(t time.Time) int {
	return int(time.Until(t).Hours() / 24)
}

// GetTypeName 获取类型名称
func GetTypeName(t int) string {
	switch t {
	case 0:
		return "国内"
	case 1:
		return "国外"
	default:
		return "未知"
	}
}

// GetPeriodName 获取周期名称
func GetPeriodName(p int) string {
	switch p {
	case 1:
		return "1个月"
	case 3:
		return "3个月"
	case 6:
		return "6个月"
	case 12:
		return "12个月"
	case 24:
		return "24个月"
	case 36:
		return "36个月"
	default:
		return fmt.Sprintf("%d个月", p)
	}
}

// Config YAML配置结构
type Config struct {
	Password   string           `yaml:"password"`
	JWTSecret  string           `yaml:"jwt_secret"`
	ServerChan ServerChanConfig `yaml:"serverchan"`
	SMTP       SMTPConfig       `yaml:"smtp"`
	Schedule   ScheduleConfig   `yaml:"schedule"`
}

// ServerChanConfig Server酱配置
type ServerChanConfig struct {
	Key string `yaml:"key"`
}

// SMTPConfig SMTP配置
type SMTPConfig struct {
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	AuthCode string `yaml:"auth_code"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
}

// ScheduleConfig 定时任务配置
type ScheduleConfig struct {
	Hour   int `yaml:"hour"`
	Minute int `yaml:"minute"`
}

// AppConfig 全局配置
var AppConfig Config

var (
	// Server酱配置
	ServerChanKey = ""
	SCTimeout     = 30 * time.Second

	// 邮件配置
	SMTPServer   = ""
	SMTPPort     = 465
	SMTPAuthCode = ""
	SMTPFrom     = ""
	SMTPTo       = ""

	// 通知配置
	NotifyDays = 0 // 提前N天通知，0表示当天通知

	// 定时任务配置
	ScheduleHour   = 10 // 默认10点
	ScheduleMinute = 30 // 默认30分
)

func LoadConfig() error {
	if key := os.Getenv("SERVER_CHAN_KEY"); key != "" {
		ServerChanKey = key
	}
	if notifyDays := os.Getenv("NOTIFY_DAYS"); notifyDays != "" {
		fmt.Sscanf(notifyDays, "%d", &NotifyDays)
	}
	return nil
}

var passwordHashData []byte
var jwtSecretData string

func InitConfig() error {
	data, err := os.ReadFile("config/config.yaml")
	if err != nil {
		return fmt.Errorf("配置文件不存在或读取失败，请运行初始化")
	}

	if err := yaml.Unmarshal(data, &AppConfig); err != nil {
		return fmt.Errorf("解析配置文件失败")
	}

	if AppConfig.Password == "" || AppConfig.JWTSecret == "" {
		return fmt.Errorf("配置不完整，请重新运行初始化")
	}

	passwordHashData = []byte(AppConfig.Password)
	jwtSecretData = AppConfig.JWTSecret

	if AppConfig.ServerChan.Key != "" {
		ServerChanKey = AppConfig.ServerChan.Key
	}
	if AppConfig.SMTP.Server != "" {
		SMTPServer = AppConfig.SMTP.Server
		SMTPPort = AppConfig.SMTP.Port
		SMTPAuthCode = AppConfig.SMTP.AuthCode
		SMTPFrom = AppConfig.SMTP.From
		SMTPTo = AppConfig.SMTP.To
	}

	if AppConfig.Schedule.Hour >= 0 && AppConfig.Schedule.Hour <= 23 {
		ScheduleHour = AppConfig.Schedule.Hour
	}
	if AppConfig.Schedule.Minute >= 0 && AppConfig.Schedule.Minute <= 59 {
		ScheduleMinute = AppConfig.Schedule.Minute
	}

	return nil
}

func GetPasswordHash() (string, error) {
	if len(passwordHashData) > 0 {
		return string(passwordHashData), nil
	}
	return "", fmt.Errorf("密码未配置")
}

func GetJWTSecret() (string, error) {
	if jwtSecretData != "" {
		return jwtSecretData, nil
	}
	return "", fmt.Errorf("JWT Secret 未配置")
}

func CheckConfig() error {
	if len(passwordHashData) == 0 || jwtSecretData == "" {
		return fmt.Errorf("配置未初始化，请运行 go run ./cmd/init")
	}
	return nil
}

// SendWeChatNotification 发送微信提醒
func SendWeChatNotification(sub models.Subscription) error {
	if ServerChanKey == "" {
		return fmt.Errorf("Server酱 key 未配置")
	}

	msg := fmt.Sprintf("【续费提醒】%s 即将到期\n服务: %s\n到期时间: %s\n剩余: %d天\n费用: %.2f元",
		sub.Name,
		sub.Name,
		sub.ExpireDate.Format(dateFmt),
		DaysUntil(sub.ExpireDate),
		sub.Price)

	form := map[string]string{
		"text": msg,
		"desp": fmt.Sprintf("类型: %s\n周期: %s", GetTypeName(sub.Type), GetPeriodName(sub.Period)),
	}
	formData, err := json.Marshal(form)
	if err != nil {
		return err
	}

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr, Timeout: SCTimeout}

	req, _ := http.NewRequest("POST", fmt.Sprintf("https://sctapi.ftqq.com/%s.send", ServerChanKey), bytes.NewReader(formData))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("发送失败: HTTP %d", resp.StatusCode)
	}
	return nil
}

// SendEmailNotification 发送邮件提醒
func SendEmailNotification(sub models.Subscription) error {
	if SMTPAuthCode == "" || SMTPTo == "" {
		return fmt.Errorf("邮件配置未完成")
	}

	subject := fmt.Sprintf("【续费提醒】%s 即将到期", sub.Name)
	body := fmt.Sprintf("%s 即将到期\n\n服务: %s\n类型: %s\n周期: %s\n费用: %.2f元\n到期时间: %s\n剩余: %d天",
		sub.Name, sub.Name, GetTypeName(sub.Type), GetPeriodName(sub.Period), sub.Price, sub.ExpireDate.Format(dateFmt), DaysUntil(sub.ExpireDate))

	from := mail.Address{Name: "订阅管理", Address: SMTPFrom}
	to := mail.Address{Name: "", Address: SMTPTo}

	return sendMail(from, to, subject, body)
}

func sendMail(from, to mail.Address, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		from.Address, to.Address, subject, body)

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", SMTPServer, SMTPPort), &tls.Config{ServerName: SMTPServer})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, SMTPServer)
	if err != nil {
		return err
	}

	if err := client.Auth(smtp.PlainAuth("", SMTPFrom, SMTPAuthCode, SMTPServer)); err != nil {
		return err
	}
	if err := client.Mail(SMTPFrom); err != nil {
		return err
	}
	if err := client.Rcpt(SMTPTo); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	w.Close()
	return client.Quit()
}

// SendWeChatBatchNotification 批量发送微信提醒
func SendWeChatBatchNotification(subs []models.Subscription) error {
	if ServerChanKey == "" {
		return fmt.Errorf("Server酱 key 未配置")
	}

	count := len(subs)
	days := "即将到期"
	if count == 1 {
		days = fmt.Sprintf("剩余 %d 天", DaysUntil(subs[0].ExpireDate))
	}

	msg := fmt.Sprintf("【续费提醒】您有 %d 个订阅 %s", count, days)

	// 构建详情
	var desp strings.Builder
	for _, sub := range subs {
		left := DaysUntil(sub.ExpireDate)
		desp.WriteString(fmt.Sprintf("\n---\n服务: %s\n类型: %s\n周期: %s\n费用: %.2f元\n到期: %s\n剩余: %d天",
			sub.Name, GetTypeName(sub.Type), GetPeriodName(sub.Period),
			sub.Price, sub.ExpireDate.Format(dateFmt), left))
	}

	form := map[string]string{
		"text": msg,
		"desp": desp.String(),
	}
	formData, err := json.Marshal(form)
	if err != nil {
		return err
	}

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr, Timeout: SCTimeout}

	req, _ := http.NewRequest("POST", fmt.Sprintf("https://sctapi.ftqq.com/%s.send", ServerChanKey), bytes.NewReader(formData))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("发送失败: HTTP %d", resp.StatusCode)
	}
	return nil
}

// SendEmailBatchNotification 批量发送邮件提醒
func SendEmailBatchNotification(subs []models.Subscription) error {
	if SMTPAuthCode == "" || SMTPTo == "" {
		return fmt.Errorf("邮件配置未完成")
	}

	count := len(subs)
	subject := fmt.Sprintf("【续费提醒】您有 %d 个订阅即将到期", count)

	var body strings.Builder
	body.WriteString(fmt.Sprintf("您有 %d 个订阅即将到期\n\n", count))
	for _, sub := range subs {
		left := DaysUntil(sub.ExpireDate)
		body.WriteString(fmt.Sprintf("---\n服务: %s\n类型: %s\n周期: %s\n费用: %.2f元\n到期时间: %s\n剩余: %d天\n\n",
			sub.Name, GetTypeName(sub.Type), GetPeriodName(sub.Period),
			sub.Price, sub.ExpireDate.Format(dateFmt), left))
	}

	from := mail.Address{Name: "订阅管理", Address: SMTPFrom}
	to := mail.Address{Name: "", Address: SMTPTo}

	return sendMail(from, to, subject, body.String())
}
