package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用程序配置
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`

	Server   ServerConfig   `mapstructure:"server"`
	OAuth2   OAuth2Config   `mapstructure:"oauth2"`
	Admin    AdminConfig    `mapstructure:"admin"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Security SecurityConfig `mapstructure:"security"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
	Debug       bool   `mapstructure:"debug"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port           int      `mapstructure:"port"`
	Host           string   `mapstructure:"host"`
	AllowedOrigins []string `mapstructure:"allowed_origins"` // CORS允许的来源列表
}

// OAuth2Config OAuth2配置
type OAuth2Config struct {
	// 模式切换: "builtin"(内置) 或 "keycloak"(外部)
	Mode             string `mapstructure:"mode"`
	JWTSigningSecret string `mapstructure:"jwt_signing_secret"`
	JWTTokenExpiry   int    `mapstructure:"jwt_token_expiry"`
	JWTIssuer        string `mapstructure:"jwt_issuer"`

	// Keycloak 模式配置
	ClientID               string `mapstructure:"client_id"`
	ClientSecret           string `mapstructure:"client_secret"`
	AuthURL                string `mapstructure:"auth_url"`
	TokenURL               string `mapstructure:"token_url"`
	UserInfoURL            string `mapstructure:"userinfo_url"`
	RedirectURI            string `mapstructure:"redirect_uri"`
	LogoutURL              string `mapstructure:"logout_url"`
	LogoutRedirectURIParam string `mapstructure:"logout_redirect_uri_param"`
}

// IsBuiltinMode 是否为内置认证模式
func (c *OAuth2Config) IsBuiltinMode() bool {
	return c.Mode == "builtin"
}

// AdminConfig 管理控制台配置
type AdminConfig struct {
	ConsoleURL string `mapstructure:"console_url"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	UploadDir        string `mapstructure:"upload_dir"`
	MaxSize          int64  `mapstructure:"max_size"`
	MaxDocumentPages int    `mapstructure:"max_document_pages"` // PDF/DOCX 等文档的最大页数限制
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	FileAccessSecret string `mapstructure:"file_access_secret"` // 文件访问凭证签名密钥
	UploadTokenTTL   int    `mapstructure:"upload_token_ttl"`   // 上传凭证有效期（秒）
	DownloadTokenTTL int    `mapstructure:"download_token_ttl"` // 下载凭证有效期（秒）
}

// Load 加载配置
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/fly-print-cloud")

	// 设置环境变量前缀
	viper.SetEnvPrefix("FLY_PRINT")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config file error: %w", err)
		}
		// 配置文件不存在时使用默认值和环境变量
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unmarshal config error: %w", err)
	}

	// 环境变量 FLY_PRINT_SERVER_ALLOWED_ORIGINS 为单字符串（或逗号分隔），Viper 不会自动转为 []string
	if raw := viper.Get("server.allowed_origins"); raw != nil {
		if v, ok := raw.(string); ok && v != "" {
			config.Server.AllowedOrigins = splitTrim(v, ",")
		}
	}

	return &config, nil
}

// splitTrim 按 sep 分割并去除每段首尾空格
func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	// 验证应用配置
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}

	// 验证数据库配置
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		return fmt.Errorf("database.port must be between 1 and 65535")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if c.Database.DBName == "" {
		return fmt.Errorf("database.dbname is required")
	}

	// 验证服务器配置
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}

	// 验证 OAuth2 配置
	if c.OAuth2.Mode != "builtin" && c.OAuth2.Mode != "keycloak" {
		return fmt.Errorf("oauth2.mode must be 'builtin' or 'keycloak', got: %s", c.OAuth2.Mode)
	}

	// Keycloak 模式需要额外配置
	if !c.OAuth2.IsBuiltinMode() {
		if c.OAuth2.ClientID == "" {
			return fmt.Errorf("oauth2.client_id is required for keycloak mode")
		}
		if c.OAuth2.ClientSecret == "" {
			return fmt.Errorf("oauth2.client_secret is required for keycloak mode")
		}
		if c.OAuth2.AuthURL == "" {
			return fmt.Errorf("oauth2.auth_url is required for keycloak mode")
		}
		if c.OAuth2.TokenURL == "" {
			return fmt.Errorf("oauth2.token_url is required for keycloak mode")
		}
		if c.OAuth2.UserInfoURL == "" {
			return fmt.Errorf("oauth2.userinfo_url is required for keycloak mode")
		}
	}

	// 警告：生产环境不应使用默认密钥
	if !c.App.Debug {
		if c.OAuth2.JWTSigningSecret == "fly-print-jwt-secret-dev-only" {
			return fmt.Errorf("SECURITY WARNING: jwt_signing_secret must be changed in production")
		}
		if c.Security.FileAccessSecret == "fly-print-file-access-secret-dev-only" {
			return fmt.Errorf("SECURITY WARNING: file_access_secret must be changed in production")
		}
	}

	// 验证JWT密钥强度（至少256位 / 32字节）
	if len(c.OAuth2.JWTSigningSecret) < 32 {
		return fmt.Errorf("SECURITY WARNING: jwt_signing_secret must be at least 32 characters long")
	}

	// 验证文件访问密钥强度（至少256位 / 32字节）
	if len(c.Security.FileAccessSecret) < 32 {
		return fmt.Errorf("SECURITY WARNING: file_access_secret must be at least 32 characters long")
	}

	// 验证存储配置
	if c.Storage.MaxSize <= 0 {
		return fmt.Errorf("storage.max_size must be greater than 0")
	}

	// 验证安全配置
	if c.Security.UploadTokenTTL <= 0 {
		return fmt.Errorf("security.upload_token_ttl must be greater than 0")
	}
	if c.Security.DownloadTokenTTL <= 0 {
		return fmt.Errorf("security.download_token_ttl must be greater than 0")
	}

	return nil
}

// GetOAuth2UserInfoURL 获取 OAuth2 UserInfo URL
func GetOAuth2UserInfoURL() string {
	return viper.GetString("oauth2.userinfo_url")
}

// setDefaults 设置默认配置值
func setDefaults() {
	// App 默认值
	viper.SetDefault("app.name", "fly-print-cloud")
	viper.SetDefault("app.version", "0.1.0")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.debug", true)

	// Database 默认值
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.dbname", "fly_print_cloud")
	viper.SetDefault("database.sslmode", "disable")

	// Server 默认值
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.allowed_origins", []string{
		"https://admin.fly-print.local",
		"https://kiosk.fly-print.local",
		"http://localhost:3000",
		"http://localhost:8080",
		"http://localhost:8012", // 与 .env.example 默认 HTTP_PORT 一致，便于一键启动
	})

	// OAuth2 默认值
	viper.SetDefault("oauth2.mode", "builtin")
	viper.SetDefault("oauth2.jwt_signing_secret", "fly-print-jwt-secret-dev-only")
	viper.SetDefault("oauth2.jwt_token_expiry", 3600)
	viper.SetDefault("oauth2.jwt_issuer", "fly-print-cloud")
	viper.SetDefault("oauth2.client_id", "")
	viper.SetDefault("oauth2.client_secret", "")
	viper.SetDefault("oauth2.auth_url", "")
	viper.SetDefault("oauth2.token_url", "")
	viper.SetDefault("oauth2.userinfo_url", "")
	viper.SetDefault("oauth2.redirect_uri", "")
	viper.SetDefault("oauth2.logout_url", "")
	viper.SetDefault("oauth2.logout_redirect_uri_param", "post_logout_redirect_uri")
	viper.SetDefault("admin.console_url", "http://localhost:3000")

	// Admin 创建配置
	viper.SetDefault("create_default_admin", "false")
	viper.SetDefault("default_admin_password", "")

	// Storage 默认值
	viper.SetDefault("storage.upload_dir", "./uploads")
	viper.SetDefault("storage.max_size", 10485760) // 10MB
	viper.SetDefault("storage.max_document_pages", 5)

	// Security 默认值
	viper.SetDefault("security.file_access_secret", "fly-print-file-access-secret-dev-only")
	viper.SetDefault("security.upload_token_ttl", 180)   // 3分钟
	viper.SetDefault("security.download_token_ttl", 180) // 3分钟
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// GetServerAddr 获取服务器地址
func (c *ServerConfig) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
