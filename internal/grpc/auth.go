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

func (as *ApiTokenService) ApiTokenCreate(ctx context.Context, req *apiTokens.ApiTokenCreateRequest) (*apiTokens.ApiTokenCreateResponse, error) {
	token, err := t.GenerateTokenValue()
	fmt.Println(token, err)
	if err != nil {
		return &apiTokens.ApiTokenCreateResponse{}, err
	}
	if err := as.Db.AddToken(int(req.Id), token); err != nil {
		return &apiTokens.ApiTokenCreateResponse{}, err
	}
	return &apiTokens.ApiTokenCreateResponse{Token: token}, nil
}

func (as *ApiTokenService) ApiTokenDelete(ctx context.Context, req *apiTokens.ApiTokenDeleteRequest) (*apiTokens.ApiTokenDeleteResponse, error) {
	if err := as.Db.DelToken(req.Token); err != nil {
		return &apiTokens.ApiTokenDeleteResponse{Result: false}, err
	}
	return &apiTokens.ApiTokenDeleteResponse{Result: true}, nil
}

func (as *ApiTokenService) ApiTokenGet(ctx context.Context, req *apiTokens.ApiTokenGetRequest) (*apiTokens.ApiTokenGetResponse, error) {
	tokens, err := as.Db.GetTokens(int(req.Id))
	if err != nil {
		return &apiTokens.ApiTokenGetResponse{}, err
	}
	return &apiTokens.ApiTokenGetResponse{Tokens: &apiTokens.Tokens{Token: tokens.Token}}, nil
}
