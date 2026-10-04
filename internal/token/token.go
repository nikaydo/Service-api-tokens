// Package token отвечает за выпуск и проверку API-токенов.
//
// Токен — непрозрачная случайная строка. В базе хранится только его
// SHA-256 хеш: утечка базы не даёт воспользоваться чужим токеном, потому что
// сам токен ещё нужно предъявить.
//
// Токен показывается пользователю один раз, при выпуске. Дальше он
// существует только у клиента и в виде хеша на сервере, поэтому восстановить
// его невозможно — при утрате нужно выпустить новый.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

// entropyBytes — количество случайных байт в токене. 32 байта дают 256 бит
// энтропии, что делает подбор невозможным.
const entropyBytes = 32

// tokenPrefix — короткий префикс, чтобы токен можно было опознать в
// журналах и в списках, не раскрывая его значения.
const tokenPrefix = "vpt_"

// Generate выпускает новый токен и возвращает его вместе с хешем.
func Generate() (token, hash string, err error) {
	buf := make([]byte, entropyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("не удалось сгенерировать токен: %w", err)
	}
	token = tokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return token, Hash(token), nil
}

// Hash возвращает SHA-256 хеш токена.
//
// SHA-256 здесь уместен: токен — не пароль, а случайная строка из 256 бит
// энтропии, которую перебором не подобрать, поэтому медленный KDF не нужен
// и не оправдан.
func Hash(t string) string {
	sum := sha256.Sum256([]byte(t))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Matches сверяет предъявленный токен с хешем из базы за постоянное время.
func Matches(hash, t string) bool {
	if hash == "" || t == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(Hash(t)), []byte(hash)) == 1
}

// IsPrefixed сообщает, похоже ли значение на выпущенный токен.
//
// Проверка отсекает мусор до обращения к базы: без неё запрос с произвольной
// строкой приводил бы к полному просмотру таблицы токенов.
func IsPrefixed(t string) bool {
	return len(t) == len(tokenPrefix)+base64.RawURLEncoding.EncodedLen(entropyBytes) &&
		t[:len(tokenPrefix)] == tokenPrefix
}
