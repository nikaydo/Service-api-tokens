package config

import (
	"strings"
	"testing"
)

func TestLoadAcceptsValidConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load вернул ошибку: %v", err)
	}
	if cfg.Port != 50052 {
		t.Errorf("Port = %d, ожидалось 50052", cfg.Port)
	}
	if cfg.MaxTokensPerUser != 10 {
		t.Errorf("MaxTokensPerUser = %d, ожидалось 10", cfg.MaxTokensPerUser)
	}
	if cfg.Addr() != "0.0.0.0:50052" {
		t.Errorf("Addr = %q", cfg.Addr())
	}
}

func TestValidateRejectsPlaceholderDatabaseURL(t *testing.T) {
	for _, url := range []string{
		"CHANGE_ME",
		"postgres://USER:PASSWORD@localhost:5432/db",
		"postgres://appuser:password@db:5432/user",
	} {
		t.Run(url, func(t *testing.T) {
			t.Setenv("DATABASE_URL", url)
			if _, err := Load(); err == nil {
				t.Fatalf("шаблон %q не должен приниматься", url)
			}
		})
	}
}

func TestValidateRejectsEmptyDatabaseURL(t *testing.T) {
	if _, err := Load(); err == nil {
		t.Fatal("отсутствие DATABASE_URL должно приводить к ошибке")
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"0", "70000", "-1"} {
		t.Run(port, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
			t.Setenv("PORT", port)
			if _, err := Load(); err == nil {
				t.Fatalf("порт %s не должен приниматься", port)
			}
		})
	}
}

func TestValidateRejectsZeroTokenLimit(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("MAX_TOKENS_PER_USER", "0")

	if _, err := Load(); err == nil {
		t.Fatal("нулевой предел токенов не должен приниматься")
	}
}

func TestValidateMentionsFieldNames(t *testing.T) {
	_, err := Load()
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("ошибка не упоминает поле: %v", err)
	}
}
