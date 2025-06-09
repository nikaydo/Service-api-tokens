package main

import (
	"log"
	"main/internal/config"
	"main/internal/database"
	au "main/internal/grpc"
	"net"
	"os"
	"os/signal"
	"syscall"

	apiTokens "github.com/nikaydo/grpc-contract/gen/apiToken"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	env, err := config.ReadEnv()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}
	log.Println("Database succesful read")
	db, err := database.DatabaseInit(env)
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}
	log.Println("Database succesful connected")
	apiTokens.RegisterApiTokenServer(grpcServer, &au.ApiTokenService{Db: db})
	log.Println("gRPC server started on :50051")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	<-quit
	log.Println("Shutting down server...")
	grpcServer.GracefulStop()
	log.Println("Server gracefully stopped")
}
