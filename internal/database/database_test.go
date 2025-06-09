package database_test

import (
	"context"
	"log"
	"main/internal/config"
	"main/internal/database"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
)

var db database.UserDB

func TestMain(m *testing.M) {
	env := config.Env{
		EnvMap: map[string]string{
			"POSTGRESS_URL": "postgres://appuser:password@localhost:15432/auth?sslmode=disable",
		},
	}

	var err error
	db, err = database.DatabaseInit(env)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}

	code := m.Run()
	os.Exit(code)
}

func clearTable(t *testing.T) {
	_, err := db.UserBD.ExecContext(context.Background(), `DELETE FROM apitokens`)
	if err != nil {
		t.Fatalf("failed to clear table: %v", err)
	}
}

func TestAddAndGetTokens(t *testing.T) {
	clearTable(t)

	id := 1
	token := "test-token-123"

	err := db.AddToken(id, token)
	assert.NoError(t, err)

	tokens, err := db.GetTokens(id)
	assert.NoError(t, err)
	assert.Contains(t, tokens.Token, token)
}

func TestVerifyToken(t *testing.T) {
	clearTable(t)

	id := 2
	token := "verify-token"

	err := db.AddToken(id, token)
	assert.NoError(t, err)

	result, err := db.Verify(token)
	assert.NoError(t, err)
	assert.Equal(t, token, result)
}

func TestDelToken(t *testing.T) {
	clearTable(t)

	id := 3
	token := "delete-me"

	err := db.AddToken(id, token)
	assert.NoError(t, err)

	err = db.DelToken(token)
	assert.NoError(t, err)

	_, err = db.Verify(token)
	assert.Error(t, err)
}
