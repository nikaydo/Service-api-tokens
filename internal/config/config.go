// Package config загружает и проверяет конфигурацию сервиса.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// placeholder — значение-заглушка из .env.example.
const placeholder = "CHANGE_ME"

// Config содержит конфигурацию сервиса.
type Config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`

	// gRPC.
	Host string `env:"HOST" envDefault:"0.0.0.0"`
	Port int    `env:"PORT" envDefault:"50052"`

	MaxMessageBytes  int           `env:"MAX_MESSAGE_BYTES" envDefault:"1048576"`
	MaxTokensPerUser int           `env:"MAX_TOKENS_PER_USER" envDefault:"10"`
	ShutdownTimeout  time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"30s"`
}

// Addr возвращает адрес gRPC-сервера.
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Load читает конфигурацию и проверяет её.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(".env"); statErr == nil {
			return Config{}, fmt.Errorf("не удалось прочитать .env: %w", err)
		}
	}

	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("не удалось разобрать конфигурацию: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate проверяет значения, которые env-парсер не может проверить сам.
func (c Config) Validate() error {
	var problems []string

	switch {
	case c.DatabaseURL == "":
		problems = append(problems, "DATABASE_URL не задан")
	case c.DatabaseURL == placeholder:
		problems = append(problems, "DATABASE_URL остался со значением-заглушкой "+placeholder)
	default:
		for _, template := range []string{"USER:PASSWORD@", "user:password@", "appuser:password@"} {
			if strings.Contains(c.DatabaseURL, template) {
				problems = append(problems, "DATABASE_URL остался шаблоном из .env.example")
				break
			}
		}
	}

	if c.Port < 1 || c.Port > 65535 {
		problems = append(problems, fmt.Sprintf("PORT=%d вне диапазона 1-65535", c.Port))
	}
	if c.MaxTokensPerUser < 1 {
		problems = append(problems, "MAX_TOKENS_PER_USER должен быть не меньше 1")
	}

	if len(problems) > 0 {
		return fmt.Errorf("некорректная конфигурация:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}
