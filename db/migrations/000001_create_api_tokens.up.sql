-- API-токены.
--
-- Хранится только хеш: утечка базы не даёт воспользоваться чужим токеном,
-- потому что сам токен ещё нужно предъявить. Пользователь видит значение
-- один раз, при выпуске, поэтому восстановить его невозможно.
--
-- token_hash уникален: это делает повторную вставку того же значения
-- невозможной и позволяет искать токен по хешу за индекс.
CREATE TABLE api_tokens (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL,
    token_hash  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX api_tokens_hash_key ON api_tokens (token_hash);

-- Получение токенов пользователя идёт по этому индексу.
CREATE INDEX api_tokens_user_idx ON api_tokens (user_id);
