package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DataDir   string        `yaml:"data_dir"`
	DBPath    string        `yaml:"db_path"`
	ExportDir string        `yaml:"export_dir"`
	Interval  time.Duration `yaml:"interval"`
	Workers   int           `yaml:"workers"`
	Sources   Sources       `yaml:"sources"`
	Notify    Notify        `yaml:"notify"`
	LogLevel  string        `yaml:"log_level"`
	LogFile   string        `yaml:"log_file"`
	Mode      string        `yaml:"mode"`
}

type Sources struct {
	Arkadiyt         bool `yaml:"arkadiyt"`
	ProjectDiscovery bool `yaml:"projectdiscovery"`
	Diodb            bool `yaml:"diodb"`
	Rix4uni          bool `yaml:"rix4uni"`
	OrgsData         bool `yaml:"orgsdata"`
	Dotgov           bool `yaml:"dotgov"`
}

type Notify struct {
	Enabled  bool     `yaml:"enabled"`
	Telegram Telegram `yaml:"telegram"`
	Webhook  Webhook  `yaml:"webhook"`
}

type Telegram struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type Webhook struct {
	Enabled bool              `yaml:"enabled"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
}

func Default() *Config {
	return &Config{
		DataDir: "./data", DBPath: "./data/bbmonitor.db", ExportDir: "./data/exports",
		Interval: 10 * time.Minute, Workers: 6,
		Sources: Sources{
			Arkadiyt: true, ProjectDiscovery: true, Diodb: true,
			Rix4uni: true, OrgsData: true, Dotgov: true,
		},
		Notify: Notify{Enabled: true, Telegram: Telegram{Enabled: false}, Webhook: Webhook{Enabled: false, Headers: map[string]string{}}},
		LogLevel: "info", Mode: "tui",
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read config: %w", err)
			}
		} else if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}
	applyEnv(cfg)
	if cfg.Workers < 1 {
		cfg.Workers = 4
	}
	if cfg.Interval < 30*time.Second {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Mode != "tui" && cfg.Mode != "daemon" {
		cfg.Mode = "tui"
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("BB_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("BB_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("BB_EXPORT_DIR"); v != "" {
		cfg.ExportDir = v
	}
	if v := os.Getenv("BB_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Interval = d
		}
	}
	if v := os.Getenv("BB_MODE"); v != "" {
		cfg.Mode = strings.ToLower(v)
	}
	if v := os.Getenv("BB_TELEGRAM_BOT_TOKEN"); v != "" {
		cfg.Notify.Telegram.BotToken = v
		cfg.Notify.Telegram.Enabled = true
	}
	if v := os.Getenv("BB_TELEGRAM_CHAT_ID"); v != "" {
		cfg.Notify.Telegram.ChatID = v
		cfg.Notify.Telegram.Enabled = true
	}
	if v := os.Getenv("BB_WEBHOOK_URL"); v != "" {
		cfg.Notify.Webhook.URL = v
		cfg.Notify.Webhook.Enabled = true
	}
	if v := os.Getenv("BB_SOURCE_DOTGOV"); v != "" {
		cfg.Sources.Dotgov = truthy(v)
	}
}

func truthy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes" || s == "on"
}
