// Package grpc реализует gRPC-сервис API-токенов.
package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiTokens "github.com/nikaydo/grpc-contract/gen/apiToken"

	"github.com/nikaydo/api-tokens-service/internal/database"
	"github.com/nikaydo/api-tokens-service/internal/token"
)

// ApiTokenService — реализация контракта ApiToken.
type ApiTokenService struct {
	apiTokens.UnimplementedApiTokenServer

	store *database.Store
	log   *slog.Logger

	// maxPerUser ограничивает количество токенов у одного пользователя.
	// Без предела один клиент мог бы забить таблицу.
	maxPerUser int
}

// New создаёт сервис.
func New(store *database.Store, log *slog.Logger, maxPerUser int) *ApiTokenService {
	if log == nil {
		log = slog.Default()
	}
	if maxPerUser < 1 {
		maxPerUser = 10
	}
	return &ApiTokenService{store: store, log: log, maxPerUser: maxPerUser}
}

// Create выпускает токен для пользователя.
//
// Токен возвращается один раз и в базе не сохраняется — только хеш. Поэтому
// при утрате восстановить его нельзя, нужно выпустить новый.
func (s *ApiTokenService) Create(ctx context.Context, req *apiTokens.CreateRequest) (*apiTokens.CreateResponse, error) {
	userID := int64(req.GetUserId())
	if userID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "некорректный идентификатор пользователя")
	}

	count, err := s.store.CountByUser(ctx, userID)
	if err != nil {
		return nil, internalError(ctx, s.log, "не удалось проверить число токенов", err)
	}
	if count >= s.maxPerUser {
		return nil, status.Errorf(codes.ResourceExhausted,
			"достигнут предел в %d токенов, удалите лишние", s.maxPerUser)
	}

	raw, hash, err := token.Generate()
	if err != nil {
		return nil, internalError(ctx, s.log, "не удалось сгенерировать токен", err)
	}
	if err := s.store.Create(ctx, userID, hash); err != nil {
		return nil, internalError(ctx, s.log, "не удалось сохранить токен", err)
	}

	s.log.Info("токен выпущен", "user_id", userID)
	return &apiTokens.CreateResponse{Token: raw}, nil
}

// Delete отзывает токен пользователя.
//
// Владелец обязателен: без него токен можно было бы отозвать у другого
// пользователя, зная его значение.
func (s *ApiTokenService) Delete(ctx context.Context, req *apiTokens.DeleteRequest) (*apiTokens.DeleteResponse, error) {
	userID := int64(req.GetUserId())
	if userID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "некорректный идентификатор пользователя")
	}
	if !token.IsPrefixed(req.GetToken()) {
		return nil, status.Error(codes.InvalidArgument, "некорректный токен")
	}

	err := s.store.Delete(ctx, userID, token.Hash(req.GetToken()))
	switch {
	case errors.Is(err, database.ErrNotFound):
		// Токен не найден или принадлежит другому пользователю. Ответ один
		// и тот же, иначе по коду можно было бы узнать о существовании
		// чужого токена.
		return nil, status.Error(codes.NotFound, "токен не найден")
	case err != nil:
		return nil, internalError(ctx, s.log, "не удалось удалить токен", err)
	}

	s.log.Info("токен отозван", "user_id", userID)
	return &apiTokens.DeleteResponse{Result: true}, nil
}

// Get возвращает токены пользователя.
//
// В ответе — только префиксы хешей: сами токены сервер не хранит и отдать их
// не может, да и не должен.
func (s *ApiTokenService) Get(ctx context.Context, req *apiTokens.GetRequest) (*apiTokens.GetResponse, error) {
	userID := int64(req.GetUserId())
	if userID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "некорректный идентификатор пользователя")
	}

	list, err := s.store.ListByUser(ctx, userID)
	if err != nil {
		return nil, internalError(ctx, s.log, "не удалось получить токены", err)
	}

	hashes := make([]string, 0, len(list))
	for _, t := range list {
		hashes = append(hashes, t.Hash)
	}
	return &apiTokens.GetResponse{Tokens: &apiTokens.Tokens{Tokens: hashes}}, nil
}

// Verify проверяет токен и возвращает признак действительности.
//
// Недействительный токен — это `result = false`, а не ошибка: отсутствие
// токена не должно выглядеть как сбой сервиса, иначе шлюзу пришлось бы
// различать две разные ситуации при обработке одного ответа.
func (s *ApiTokenService) Verify(ctx context.Context, req *apiTokens.VerifyRequest) (*apiTokens.VerifyResponse, error) {
	// Формат отсекается до обращения к базе: без этой проверки запрос с
	// произвольной строкой приводил бы к полному просмотру таблицы.
	if !token.IsPrefixed(req.GetToken()) {
		return &apiTokens.VerifyResponse{Result: false}, nil
	}

	userID, err := s.store.Verify(ctx, token.Hash(req.GetToken()))
	switch {
	case errors.Is(err, database.ErrNotFound):
		return &apiTokens.VerifyResponse{Result: false}, nil
	case err != nil:
		return nil, internalError(ctx, s.log, "не удалось проверить токен", err)
	}

	s.log.Debug("токен использован", "user_id", userID)
	return &apiTokens.VerifyResponse{Result: true}, nil
}

// internalError логирует причину и возвращает клиенту нейтральный ответ.
func internalError(ctx context.Context, log *slog.Logger, public string, cause error) error {
	log.LogAttrs(ctx, slog.LevelError, "ошибка сервиса API-токенов",
		slog.String("public", public),
		slog.String("cause", cause.Error()),
	)
	return status.Error(codes.Internal, fmt.Sprintf("%s: %v", public, internalSuffix(cause)))
}

// internalSuffix оставляет в тексте ошибки только тип, без деталей.
func internalSuffix(err error) string {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return "ошибка базы данных (" + pgErr.SQLState() + ")"
	}
	return "внутренняя ошибка"
}
