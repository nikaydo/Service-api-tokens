package token

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateShape(t *testing.T) {
	raw, hash, err := Generate()
	if err != nil {
		t.Fatalf("Generate вернул ошибку: %v", err)
	}

	if !IsPrefixed(raw) {
		t.Errorf("токен %q не распознаётся как выпущенный", raw)
	}
	if hash == raw {
		t.Fatal("хеш совпадает с самим токеном")
	}
	if Hash(raw) != hash {
		t.Fatal("хеш не воспроизводится")
	}
	want := len(tokenPrefix) + base64.RawURLEncoding.EncodedLen(entropyBytes)
	if len(raw) != want {
		t.Errorf("длина токена %d символов, ожидалось %d", len(raw), want)
	}
}

func TestGenerateIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, 500)
	for i := 0; i < 500; i++ {
		raw, _, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if _, dup := seen[raw]; dup {
			t.Fatalf("токен повторился на итерации %d", i)
		}
		seen[raw] = struct{}{}
	}
}

func TestMatches(t *testing.T) {
	raw, hash, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !Matches(hash, raw) {
		t.Error("верный токен должен совпадать со своим хешем")
	}
	if Matches(hash, raw+"x") {
		t.Error("изменённый токен не должен совпадать")
	}
	if Matches("", raw) {
		t.Error("пустой хеш не должен совпадать ни с чем")
	}
	if Matches(hash, "") {
		t.Error("пустой токен не должен совпадать")
	}
}

func TestIsPrefixedRejectsGarbage(t *testing.T) {
	// Неверный формат должен отсекаться до обращения к базе: иначе запрос с
	// произвольной строкой приводил бы к полному просмотру таблицы.
	for _, s := range []string{
		"",
		"мусор",
		strings.Repeat("a", 100),
		"vpt_",
		"vpt_короткий",
		"other_abcdefghijklmnopqrstuvwxyz0123456789",
		"vpt_abcdefghijklmnopqrstuvwxyzABCDEF+/",
	} {
		if IsPrefixed(s) {
			t.Errorf("строка %q ошибочно принята за токен", s)
		}
	}
}

func TestIsPrefixedAcceptsGenerated(t *testing.T) {
	raw, _, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !IsPrefixed(raw) {
		t.Error("выпущенный токен должен распознаваться")
	}
}
