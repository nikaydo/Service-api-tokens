package database

import (
	"context"
	"database/sql"
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
	DB, err := sql.Open("pgx", Env.EnvMap["POSTGRESS_URL"])
	if err != nil {
		return UserDB{}, err
	}
	if err = DB.Ping(); err != nil {
		return UserDB{}, err
	}
	u := UserDB{UserBD: DB, ENV: Env}
	err = u.Tables()
	if err != nil {
		return u, err
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

func (u *UserDB) DelToken(token string) error {
	_, err := u.UserBD.ExecContext(context.Background(), `
		DELETE FROM apitokens WHERE token = $1;`, token)
	if err != nil {
		return err
	}
	return nil
}
