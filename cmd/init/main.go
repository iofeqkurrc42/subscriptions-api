package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// Config YAML配置结构
type Config struct {
	Password   string           `yaml:"password"`
	JWTSecret  string           `yaml:"jwt_secret"`
	ServerChan ServerChanConfig `yaml:"serverchan"`
	SMTP       SMTPConfig       `yaml:"smtp"`
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

func main() {
	fmt.Println("=== 订阅管理系统初始化 ===")
	fmt.Println()

	config := Config{}

	// 检查配置文件是否已存在
	if _, err := os.Stat("config/config.yaml"); err == nil {
		fmt.Println("配置文件已存在，是否覆盖? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.TrimSpace(input) != "y" && strings.TrimSpace(input) != "Y" {
			fmt.Println("取消初始化")
			return
		}
	}

	// 输入密码
	fmt.Print("请设置登录密码: ")
	password := readPassword()
	fmt.Print("请确认密码: ")
	password2 := readPassword()
	if password != password2 {
		fmt.Println("两次密码输入不一致")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("密码加密失败: %v\n", err)
		os.Exit(1)
	}
	config.Password = string(hash)

	// 输入 JWT Secret
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("请输入 JWT Secret (用于token验证): ")
	jwtSecret, _ := reader.ReadString('\n')
	jwtSecret = strings.TrimSpace(jwtSecret)
	if jwtSecret == "" {
		fmt.Println("JWT Secret 不能为空")
		os.Exit(1)
	}
	config.JWTSecret = jwtSecret

	// 输入 Server酱 key
	reader = bufio.NewReader(os.Stdin)
	fmt.Print("请输入 Server酱 key (直接回车跳过): ")
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)
	if key != "" {
		config.ServerChan.Key = key
	}

	// 输入 SMTP 配置
	fmt.Println("\n=== SMTP 配置 (直接回车跳过) ===")
	fmt.Print("SMTP 服务器地址: ")
	server, _ := reader.ReadString('\n')
	server = strings.TrimSpace(server)
	if server != "" {
		config.SMTP.Server = server

		fmt.Print("SMTP 端口 (默认 587): ")
		portStr, _ := reader.ReadString('\n')
		portStr = strings.TrimSpace(portStr)
		if portStr == "" {
			config.SMTP.Port = 587
		} else {
			fmt.Sscanf(portStr, "%d", &config.SMTP.Port)
		}

		fmt.Print("SMTP 授权码: ")
		config.SMTP.AuthCode = readPassword()

		fmt.Print("SMTP 发件人: ")
		config.SMTP.From, _ = reader.ReadString('\n')
		config.SMTP.From = strings.TrimSpace(config.SMTP.From)

		fmt.Print("SMTP 收件人 (多个用逗号分隔): ")
		config.SMTP.To, _ = reader.ReadString('\n')
		config.SMTP.To = strings.TrimSpace(config.SMTP.To)
	}

	// 写入配置文件
	if err := os.MkdirAll("config", 0755); err != nil {
		fmt.Printf("创建配置目录失败: %v\n", err)
		os.Exit(1)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		fmt.Printf("序列化配置失败: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("config/config.yaml", data, 0644); err != nil {
		fmt.Printf("写入配置文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("初始化完成! 配置文件已保存到 config/config.yaml")
}

func readPassword() string {
	// 使用 Unix 终端控制码隐藏输入
	fmt.Print("\033[8m")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	// 恢复显示
	fmt.Print("\033[0m")
	return strings.TrimSpace(line)
}
