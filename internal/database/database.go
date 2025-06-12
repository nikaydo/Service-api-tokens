package database

import (
	"context"
	"database/sql"
	"fmt"
	"main/internal/config"
	"main/internal/models"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type UserDB struct {
	UserBD *sql.DB
	ENV    config.Env
}

func DatabaseInit(Env config.Env) (UserDB, error) {
	var err error
	DB, err := sql.Open("pgx", Env.EnvMap["DATABASE_URL"])
	if err != nil {
		return UserDB{}, fmt.Errorf("error open database: %w", err)
	}
	if err = DB.Ping(); err != nil {
		return UserDB{}, fmt.Errorf("error connect to database: %w", err)
	}
	u := UserDB{UserBD: DB, ENV: Env}
	err = u.Tables()
	if err != nil {
		return u, fmt.Errorf("error create tanle: %w", err)
	}
	return u, nil
}

func (u *UserDB) Tables() error {
	_, err := u.UserBD.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS apitokens (
		id INT,
		token TEXT NOT NULL
		);`)
	if err != nil {
		return err
	}
	return nil
}

func (u *UserDB) AddToken(id int, token string) error {
	_, err := u.UserBD.ExecContext(context.Background(), `
		INSERT INTO apitokens (id,token)
		VALUES ($1,$2);`, id, token)
	if err != nil {
		return err
	}
	return nil
}

func (u *UserDB) GetTokens(id int) (models.Tokens, error) {
	var g models.Tokens
	var n string
	rows, err := u.UserBD.QueryContext(context.Background(), `
	SELECT token FROM apitokens WHERE id = $1`, id)
	if err != nil {
		return g, err
	}
	for rows.Next() {
		err = rows.Scan(&n)
		if err != nil {
			return g, err
		}
		g.Token = append(g.Token, n)
	}
	return g, nil
}

func (u *UserDB) Verify(token string) (string, error) {
	var result string
	err := u.UserBD.QueryRowContext(context.Background(), `SELECT token FROM apitokens WHERE token = $1`, token).Scan(&result)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (u *UserDB) DelToken(token string) error {
	_, err := u.UserBD.ExecContext(context.Background(), `
		DELETE FROM apitokens WHERE token = $1;`, token)
	if err != nil {
		return err
	}
	return nil
}
