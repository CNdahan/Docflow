package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Storage      StorageConfig      `mapstructure:"storage"`
	Auth         AuthConfig         `mapstructure:"auth"`
	InitialSuper InitialSuperConfig `mapstructure:"initial_super"`
	Log          LogConfig          `mapstructure:"log"`
	CORS         CORSConfig         `mapstructure:"cors"`
}

type ServerConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

type StorageConfig struct {
	Root                        string `mapstructure:"root"`
	TmpDir                      string `mapstructure:"tmp_dir"`
	MaxFileSize                 int64  `mapstructure:"max_file_size"`
	MaxInlineImageSize          int64  `mapstructure:"max_inline_image_size"`
	MaxAttachmentsPerDocument   int    `mapstructure:"max_attachments_per_document"`
	MaxTemplatesPerDocument     int    `mapstructure:"max_templates_per_document"`
	MaxAttachmentsPerSubmission int    `mapstructure:"max_attachments_per_submission"`
	MaxInlineImagesPerDocument  int    `mapstructure:"max_inline_images_per_document"`
}

type AuthConfig struct {
	JWTSecret       string `mapstructure:"jwt_secret"`
	AccessTokenTTL  int    `mapstructure:"access_token_ttl"`
	RefreshTokenTTL int    `mapstructure:"refresh_token_ttl"`
	BcryptCost      int    `mapstructure:"bcrypt_cost"`
}

type InitialSuperConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	RealName string `mapstructure:"real_name"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type CORSConfig struct {
	AllowOrigins []string `mapstructure:"allow_origins"`
}

// Load 读取 YAML 配置,环境变量优先覆盖。
// 支持 DOCFLOW_DB_PASSWORD / DOCFLOW_JWT_SECRET / DOCFLOW_INITIAL_PASSWORD。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("DOCFLOW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if pw := os.Getenv("DOCFLOW_DB_PASSWORD"); pw != "" {
		cfg.Database.Password = pw
	}
	if s := os.Getenv("DOCFLOW_JWT_SECRET"); s != "" {
		cfg.Auth.JWTSecret = s
	}
	if pw := os.Getenv("DOCFLOW_INITIAL_PASSWORD"); pw != "" {
		cfg.InitialSuper.Password = pw
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret is required (set DOCFLOW_JWT_SECRET)")
	}
	if len(c.Auth.JWTSecret) < 16 {
		return fmt.Errorf("auth.jwt_secret too short, need >= 16 chars")
	}
	if c.Storage.Root == "" {
		return fmt.Errorf("storage.root is required")
	}
	if c.Auth.BcryptCost < 4 || c.Auth.BcryptCost > 14 {
		return fmt.Errorf("auth.bcrypt_cost must be 4..14")
	}
	return nil
}
