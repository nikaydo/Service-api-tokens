package grpc

import (
	"context"
	"fmt"
	"main/internal/database"
	t "main/internal/token"

	apiTokens "github.com/nikaydo/grpc-contract/gen/apiToken"
)

type ApiTokenService struct {
	apiTokens.UnimplementedApiTokenServer
	Db database.UserDB
}

func (as *ApiTokenService) Create(ctx context.Context, req *apiTokens.CreateRequest) (*apiTokens.CreateResponse, error) {
	token, err := t.GenerateTokenValue()
	if err != nil {
		return &apiTokens.CreateResponse{}, fmt.Errorf("error generating token: %w", err)
	}
	if err := as.Db.AddToken(int(req.Id), token); err != nil {
		return &apiTokens.CreateResponse{}, fmt.Errorf("error adding token to db: %w", err)
	}
	return &apiTokens.CreateResponse{Token: token}, nil
}

func (as *ApiTokenService) Delete(ctx context.Context, req *apiTokens.DeleteRequest) (*apiTokens.DeleteResponse, error) {
	if err := as.Db.DelToken(req.Token); err != nil {
		return &apiTokens.DeleteResponse{Result: false}, fmt.Errorf("error deleting token: %w", err)
	}
	return &apiTokens.DeleteResponse{Result: true}, nil
}

func (as *ApiTokenService) Get(ctx context.Context, req *apiTokens.GetRequest) (*apiTokens.GetResponse, error) {
	tokens, err := as.Db.GetTokens(int(req.Id))
	if err != nil {
		return &apiTokens.GetResponse{}, fmt.Errorf("error getting tokens from db: %w", err)
	}
	return &apiTokens.GetResponse{Tokens: &apiTokens.Tokens{Tokens: tokens.Token}}, nil
}

func (as *ApiTokenService) Verify(ctx context.Context, req *apiTokens.VerifyRequest) (*apiTokens.VerifyResponse, error) {
	_, err := as.Db.Verify(req.Token)
	if err != nil {
		return &apiTokens.VerifyResponse{}, fmt.Errorf("error verifying token: %w", err)
	}
	return &apiTokens.VerifyResponse{Result: true}, nil
}
