package mcpserver

import (
	"encoding/json"
	"os"
)

const (
	defaultTimeoutSecs = 30
	defaultMaxOutput   = 64 * 1024
)

type Config struct {
	AllowedHosts       []string `json:"allowed_hosts"`
	ConfirmDestructive bool     `json:"confirm_destructive"`
	DefaultTimeoutSecs int      `json:"default_timeout_secs"`
	MaxOutputBytes     int      `json:"max_output_bytes"`
	AuditLog           string   `json:"audit_log"`
}

func defaultConfig() Config {
	return Config{
		AllowedHosts:       []string{"*"},
		DefaultTimeoutSecs: defaultTimeoutSecs,
		MaxOutputBytes:     defaultMaxOutput,
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.DefaultTimeoutSecs <= 0 {
		cfg.DefaultTimeoutSecs = defaultTimeoutSecs
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = defaultMaxOutput
	}
	return cfg, nil
}
