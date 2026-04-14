package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Postgres PostgresConfig `yaml:"postgres"`
	Telegram TelegramConfig `yaml:"telegram"`
	Vaults   []VaultConfig  `yaml:"vaults"`
}

type AppConfig struct {
	Timezone           string  `yaml:"timezone"`
	DefaultVaultID     int64   `yaml:"default_vault_id"`
	DebounceWindow     string  `yaml:"debounce_window"`
	TaskMatchThreshold float64 `yaml:"task_match_threshold"`
	DailyNotesDir      string  `yaml:"daily_notes_dir"`
	WeeklyGoalsDir     string  `yaml:"weekly_goals_dir"`
	DailySummaryHeader string  `yaml:"daily_summary_header"`
}

type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode"`
}

type TelegramConfig struct {
	Token          string  `yaml:"token"`
	AllowedChatIDs []int64 `yaml:"allowed_chat_ids"`
}

type VaultConfig struct {
	ID       int64  `yaml:"id"`
	Name     string `yaml:"name"`
	RootPath string `yaml:"root_path"`
	Timezone string `yaml:"-"`
}

func Load(path string) (Config, error) {
	envPath := os.Getenv("APP_ENV_FILE")
	if envPath == "" {
		envPath = ".env"
	}
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return Config{}, fmt.Errorf("load %s: %w", envPath, err)
		}
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("stat %s: %w", envPath, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return Config{}, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return Config{}, fmt.Errorf("read %s: expected file, got directory", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	data = replaceEnvVars(data)

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}

	if cfg.App.Timezone == "" {
		cfg.App.Timezone = "UTC+3"
	}
	if cfg.App.DefaultVaultID == 0 && len(cfg.Vaults) > 0 {
		cfg.App.DefaultVaultID = cfg.Vaults[0].ID
	}
	if cfg.App.DebounceWindow == "" {
		cfg.App.DebounceWindow = "10s"
	}
	if cfg.App.TaskMatchThreshold == 0 {
		cfg.App.TaskMatchThreshold = 0.72
	}
	if cfg.App.DailyNotesDir == "" {
		cfg.App.DailyNotesDir = "Daily"
	}
	if cfg.App.WeeklyGoalsDir == "" {
		cfg.App.WeeklyGoalsDir = "Weakly goals"
	}
	if cfg.App.DailySummaryHeader == "" {
		cfg.App.DailySummaryHeader = "Today's summary"
	}
	if cfg.Postgres.Port == 0 {
		cfg.Postgres.Port = 5432
	}
	if cfg.Postgres.SSLMode == "" {
		cfg.Postgres.SSLMode = "disable"
	}
	for index := range cfg.Vaults {
		cfg.Vaults[index].Timezone = cfg.App.Timezone
	}

	return cfg, nil
}

func (a AppConfig) DebounceDuration() (time.Duration, error) {
	return time.ParseDuration(a.DebounceWindow)
}

func LoadLocation(name string) (*time.Location, error) {
	if offset, ok := parseFixedOffset(name); ok {
		return time.FixedZone(name, offset), nil
	}
	return time.LoadLocation(name)
}

func parseFixedOffset(name string) (int, bool) {
	if !strings.HasPrefix(strings.ToUpper(name), "UTC") {
		return 0, false
	}
	if len(name) == 3 {
		return 0, true
	}
	sign := name[3]
	if sign != '+' && sign != '-' {
		return 0, false
	}
	rest := name[4:]
	parts := strings.Split(rest, ":")
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	minutes := 0
	if len(parts) > 1 {
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, false
		}
	}
	offset := hours*3600 + minutes*60
	if sign == '-' {
		offset = -offset
	}
	return offset, true
}

func (p PostgresConfig) DSN() string {
	query := url.Values{}
	query.Set("sslmode", p.SSLMode)
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?%s",
		url.QueryEscape(p.User),
		url.QueryEscape(p.Password),
		p.Host,
		p.Port,
		url.PathEscape(p.Name),
		query.Encode(),
	)
}

func replaceEnvVars(input []byte) []byte {
	envVarRegexp := regexp.MustCompile(`\$\{(\w+)\}`)
	return envVarRegexp.ReplaceAllFunc(input, func(match []byte) []byte {
		key := string(match[2 : len(match)-1])
		value := strconv.Quote(os.Getenv(key))
		return []byte(value[1 : len(value)-1])
	})
}
