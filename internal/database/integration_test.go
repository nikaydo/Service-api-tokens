package database_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/nikaydo/api-tokens-service/internal/database"
	"github.com/nikaydo/api-tokens-service/internal/token"
)

func testStore(t *testing.T) *database.Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задан: интеграционные тесты пропущены")
	}

	ctx := context.Background()
	if err := database.RunMigrations(ctx, url, migrationsDir(t)); err != nil {
		t.Fatalf("миграции не применились: %v", err)
	}

	store, err := database.New(ctx, url)
	if err != nil {
		t.Fatalf("не удалось подключиться к базе: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("TEST_MIGRATIONS_DIR"); dir != "" {
		return dir
	}
	return "../../db/migrations"
}

// uniqueUserID даёт идентификатор, уникальный на каждый запуск: тесты
// выполняются против общей базы.
func uniqueUserID() int64 {
	return time.Now().UnixNano() % 1_000_000_000
}

// TestTokenIsNotStoredInPlaintext — регрессия на хранение секрета открытым
// текстом. Раньше значение токена попадало в таблицу как есть.
func TestTokenIsNotStoredInPlaintext(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := uniqueUserID()
	raw, hash, err := token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := store.Create(ctx, userID, hash); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("токенов %d, ожидался 1", len(list))
	}
	if list[0].Hash == raw {
		t.Fatal("токен сохранён в открытом виде")
	}
	if list[0].Hash != hash {
		t.Errorf("хеш не совпадает: %q", list[0].Hash)
	}
}

func TestVerifyReturnsOwner(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := uniqueUserID()
	_, hash, err := token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := store.Create(ctx, userID, hash); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Verify(ctx, hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != userID {
		t.Errorf("Verify вернул владельца %d, ожидался %d", got, userID)
	}
}

func TestVerifyUnknownToken(t *testing.T) {
	store := testStore(t)

	_, hash, err := token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Токен сгенерирован, но не сохранён: он не должен проходить проверку.
	if _, err := store.Verify(context.Background(), hash); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("ожидался ErrNotFound, получено %v", err)
	}
}

func TestVerifyUpdatesLastUsed(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := uniqueUserID()
	_, hash, _ := token.Generate()
	if err := store.Create(ctx, userID, hash); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := store.Verify(ctx, hash); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	list, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 || list[0].LastUsed == nil {
		t.Errorf("время последнего использования не обновилось: %+v", list)
	}
}

// TestOwnershipEnforcement — главный тест на устранение IDOR.
//
// Раньше удаление и чтение шли по одному значению токена без проверки
// владельца, поэтому любой мог удалить или получить чужой токен.
func TestOwnershipEnforcement(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	ownerID := uniqueUserID()
	intruderID := ownerID + 1

	_, hash, err := token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := store.Create(ctx, ownerID, hash); err != nil {
		t.Fatalf("Create: %v", err)
	}

	t.Run("чужой токен нельзя удалить", func(t *testing.T) {
		err := store.Delete(ctx, intruderID, hash)
		if !errors.Is(err, database.ErrNotFound) {
			t.Fatalf("Delete вернул %v, ожидался ErrNotFound", err)
		}

		// Токен владельца должен сохраниться.
		if _, err := store.Verify(ctx, hash); err != nil {
			t.Errorf("токен владельца удалён чужим запросом: %v", err)
		}
	})

	t.Run("чужие токены не видны в списке", func(t *testing.T) {
		list, err := store.ListByUser(ctx, intruderID)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		for _, tok := range list {
			if tok.Hash == hash {
				t.Fatal("в списке чужого пользователя виден чужой токен")
			}
		}
	})

	t.Run("владелец может удалить свой токен", func(t *testing.T) {
		if err := store.Delete(ctx, ownerID, hash); err != nil {
			t.Fatalf("владелец не смог удалить свой токен: %v", err)
		}
		if _, err := store.Verify(ctx, hash); !errors.Is(err, database.ErrNotFound) {
			t.Errorf("токен продолжает работать после удаления: %v", err)
		}
	})
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, 0, "hash"); err == nil {
		t.Error("нулевой идентификатор пользователя должен отклоняться")
	}
	if err := store.Create(ctx, 1, ""); err == nil {
		t.Error("пустой хеш должен отклоняться")
	}
}

func TestCountByUser(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := uniqueUserID()

	for i := 0; i < 3; i++ {
		_, hash, _ := token.Generate()
		if err := store.Create(ctx, userID, hash); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	n, err := store.CountByUser(ctx, userID)
	if err != nil {
		t.Fatalf("CountByUser: %v", err)
	}
	if n != 3 {
		t.Errorf("токенов %d, ожидалось 3", n)
	}
}

func TestTokensAreDistinct(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	userID := uniqueUserID()
	hashes := make(map[string]struct{})

	for i := 0; i < 5; i++ {
		_, hash, _ := token.Generate()
		if _, dup := hashes[hash]; dup {
			t.Fatal("хеш повторился")
		}
		hashes[hash] = struct{}{}
		if err := store.Create(ctx, userID, hash); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	list, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 5 {
		t.Errorf("сохранено %d токенов, ожидалось 5", len(list))
	}
}
