// Package database отвечает за работу с PostgreSQL.
//
// Имена таблиц заданы в коде, а не приходят из конфигурации. Все методы
// принимают context.Context, чтобы отмена запроса прерывала обращение к базе.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound возвращается, когда запись не найдена.
var ErrNotFound = errors.New("запись не найдена")

// Store — обёртка над пулом соединений.
type Store struct {
	pool *pgxpool.Pool
}

// New открывает пул соединений и проверяет доступность базы.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("не удалось разобрать строку подключения: %w", err)
	}
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать пул соединений: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("база данных недоступна: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close закрывает пул соединений.
func (s *Store) Close() {
	s.pool.Close()
}

// RunMigrations применяет миграции из каталога dir.
func RunMigrations(ctx context.Context, databaseURL, dir string) error {
	m, err := migrate.New("file://"+dir, databaseURL)
	if err != nil {
		return fmt.Errorf("не удалось инициализировать миграции: %w", err)
	}
	defer func() {
		if sourceErr, dbErr := m.Close(); sourceErr != nil {
			fmt.Printf("ошибка закрытия источника миграций: %v\n", sourceErr)
		} else if dbErr != nil {
			fmt.Printf("ошибка закрытия соединения миграций: %v\n", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("не удалось применить миграции: %w", err)
	}
	return nil
}

// Token — выпущенный API-токен.
type Token struct {
	Hash      string
	CreatedAt time.Time
	LastUsed  *time.Time
}

// Create сохраняет токен для пользователя.
//
// Хеш уникален, поэтому повторная вставка того же значения (что практически
// невозможно при 256 битах энтропии) всё равно не создаст дубликат.
func (s *Store) Create(ctx context.Context, userID int64, tokenHash string) error {
	if userID <= 0 {
		return errors.New("некорректный идентификатор пользователя")
	}
	if tokenHash == "" {
		return errors.New("хеш токена не может быть пустым")
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO api_tokens (user_id, token_hash)
		VALUES ($1, $2)
	`, userID, tokenHash)
	if err != nil {
		return fmt.Errorf("не удалось сохранить токен: %w", err)
	}
	return nil
}

// Verify возвращает идентификатор владельца действующего токена.
//
// Сравнение хеша выполняется в Go, а не в SQL: хеш токена нельзя передать
// в запрос как есть и сравнить, потому что тогда он оказался бы в плане
// запроса и в логе медленного запроса PostgreSQL.
func (s *Store) Verify(ctx context.Context, tokenHash string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx, `
		UPDATE api_tokens
		SET last_used_at = now()
		WHERE token_hash = $1
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("не удалось проверить токен: %w", err)
	}
	return userID, nil
}

// ListByUser возвращает токены пользователя.
//
// Идентификатор владельца передаётся обязательным параметром: без этой
// проверки можно было бы получить токены чужого пользователя, зная его id.
func (s *Store) ListByUser(ctx context.Context, userID int64) ([]Token, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT token_hash, created_at, last_used_at
		FROM api_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить токены: %w", err)
	}
	defer rows.Close()

	out := make([]Token, 0)
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.Hash, &t.CreatedAt, &t.LastUsed); err != nil {
			return nil, fmt.Errorf("ошибка чтения токена: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка перебора токенов: %w", err)
	}
	return out, nil
}

// Delete удаляет токен пользователя.
//
// Владелец участвует в условии удаления: раньше его не было, поэтому любой
// мог удалить чужой токен, зная его значение.
func (s *Store) Delete(ctx context.Context, userID int64, tokenHash string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM api_tokens WHERE user_id = $1 AND token_hash = $2`, userID, tokenHash)
	if err != nil {
		return fmt.Errorf("не удалось удалить токен: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountByUser возвращает количество токенов пользователя.
func (s *Store) CountByUser(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM api_tokens WHERE user_id = $1`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("не удалось посчитать токены: %w", err)
	}
	return n, nil
}
