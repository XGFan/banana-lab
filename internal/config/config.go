package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Model describes a single image-generation model registered in config.yaml.
type Model struct {
	ID                  string   `yaml:"id"`
	Label               string   `yaml:"label"`
	Enabled             bool     `yaml:"enabled"`
	AllowedSizes        []string `yaml:"allowed_sizes"`
	AllowedAspectRatios []string `yaml:"allowed_aspect_ratios"`
	DefaultSize         string   `yaml:"default_size"`
}

// Config holds runtime configuration loaded from environment variables and config.yaml.
type Config struct {
	ServerPort           int           `yaml:"server_port"`
	StorageRoot          string        `yaml:"storage_root"`
	DBPath               string        `yaml:"db_path"`
	ProxyBaseURL         string        `yaml:"proxy_base_url"`
	ProxyAuthHeaderName  string        `yaml:"proxy_auth_header_name"`
	ProxyAuthHeaderValue string        `yaml:"proxy_auth_header_value"`
	Models               []Model       `yaml:"models"`
	GenerateTimeout      time.Duration `yaml:"generate_timeout"`
	MaxConcurrent        int           `yaml:"max_concurrent"`
	LibraryPageSize      int           `yaml:"library_page_size"`
}

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		ServerPort:          8080,
		StorageRoot:         os.Getenv("STORAGE_ROOT"),
		DBPath:              os.Getenv("DB_PATH"),
		ProxyAuthHeaderName: "Authorization",
		MaxConcurrent:       1,
		LibraryPageSize:     20,
		GenerateTimeout:     120 * time.Second,
	}

	if fileCfg, ok, err := loadFromFile("config.yaml"); err != nil {
		return Config{}, err
	} else if ok {
		applyConfig(&cfg, fileCfg)
	}

	var err error

	cfg.ProxyBaseURL, err = requiredValue("PROXY_BASE_URL", cfg.ProxyBaseURL)
	if err != nil {
		return Config{}, err
	}

	cfg.ProxyAuthHeaderValue, err = requiredValue("PROXY_AUTH_HEADER_VALUE", cfg.ProxyAuthHeaderValue)
	if err != nil {
		return Config{}, err
	}

	if err := validateModels(cfg.Models); err != nil {
		return Config{}, err
	}

	if v, ok := optionalEnv("PROXY_AUTH_HEADER_NAME"); ok {
		cfg.ProxyAuthHeaderName = v
	}

	if v, ok := optionalEnv("STORAGE_ROOT"); ok {
		cfg.StorageRoot = v
	}

	if v, ok := optionalEnv("DB_PATH"); ok {
		cfg.DBPath = v
	}

	cfg.ServerPort, err = optionalIntEnv("SERVER_PORT", cfg.ServerPort)
	if err != nil {
		return Config{}, err
	}

	cfg.MaxConcurrent, err = optionalIntEnv("MAX_CONCURRENT", cfg.MaxConcurrent)
	if err != nil {
		return Config{}, err
	}

	cfg.LibraryPageSize, err = optionalIntEnv("LIBRARY_PAGE_SIZE", cfg.LibraryPageSize)
	if err != nil {
		return Config{}, err
	}

	cfg.GenerateTimeout, err = optionalDurationEnv("GENERATE_TIMEOUT", cfg.GenerateTimeout)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadFromFile(path string) (Config, bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("读取配置文件失败: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cfg, true, nil
}

func applyConfig(dst *Config, src Config) {
	if src.ServerPort != 0 {
		dst.ServerPort = src.ServerPort
	}
	if strings.TrimSpace(src.StorageRoot) != "" {
		dst.StorageRoot = src.StorageRoot
	}
	if strings.TrimSpace(src.DBPath) != "" {
		dst.DBPath = src.DBPath
	}
	if strings.TrimSpace(src.ProxyBaseURL) != "" {
		dst.ProxyBaseURL = src.ProxyBaseURL
	}
	if strings.TrimSpace(src.ProxyAuthHeaderName) != "" {
		dst.ProxyAuthHeaderName = src.ProxyAuthHeaderName
	}
	if strings.TrimSpace(src.ProxyAuthHeaderValue) != "" {
		dst.ProxyAuthHeaderValue = src.ProxyAuthHeaderValue
	}
	if src.Models != nil && len(src.Models) > 0 {
		dst.Models = append([]Model(nil), src.Models...)
	}
	if src.GenerateTimeout != 0 {
		dst.GenerateTimeout = src.GenerateTimeout
	}
	if src.MaxConcurrent != 0 {
		dst.MaxConcurrent = src.MaxConcurrent
	}
	if src.LibraryPageSize != 0 {
		dst.LibraryPageSize = src.LibraryPageSize
	}
}

func requiredEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("缺少环境变量 %s", key)
	}

	return value, nil
}

func requiredValue(key string, current string) (string, error) {
	if value, ok := optionalEnv(key); ok {
		return value, nil
	}
	if strings.TrimSpace(current) != "" {
		return current, nil
	}
	return "", fmt.Errorf("缺少配置 %s", key)
}

func optionalEnv(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}

	return value, true
}

func optionalIntEnv(key string, defaultValue int) (int, error) {
	value, ok := optionalEnv(key)
	if !ok {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("环境变量 %s 解析为整数失败: %w", key, err)
	}

	return parsed, nil
}

func optionalDurationEnv(key string, defaultValue time.Duration) (time.Duration, error) {
	value, ok := optionalEnv(key)
	if !ok {
		return defaultValue, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("环境变量 %s 解析为时长失败: %w", key, err)
	}

	return parsed, nil
}

// validateModels enforces invariants over the model registry loaded from
// config.yaml. The registry has no environment-variable bridge; config.yaml
// is the sole source of truth.
func validateModels(models []Model) error {
	if len(models) == 0 {
		return fmt.Errorf("配置 models 不能为空: 至少需要一个模型")
	}
	if !models[0].Enabled {
		return fmt.Errorf("配置 models[0] (%s) 必须 enabled=true (默认模型必须可用)", models[0].ID)
	}
	seen := make(map[string]struct{}, len(models))
	for i, m := range models {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			return fmt.Errorf("配置 models[%d].id 不能为空", i)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("配置 models 中存在重复 id: %s", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
