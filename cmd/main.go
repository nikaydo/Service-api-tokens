// Command service — gRPC-сервис выпуска и проверки API-токенов.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	apiTokens "github.com/nikaydo/grpc-contract/gen/apiToken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcsrv "github.com/nikaydo/api-tokens-service/internal/grpc"

	"github.com/nikaydo/api-tokens-service/internal/config"
	"github.com/nikaydo/api-tokens-service/internal/database"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("сервис завершился с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.Info("конфигурация загружена", "addr", cfg.Addr())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := database.RunMigrations(startupCtx, cfg.DatabaseURL, migrationsDir()); err != nil {
		return err
	}

	store, err := database.New(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	lis, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		return err
	}

	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.MaxMessageBytes),
		// Без интерцептора паника в обработчике обрушивает процесс:
		// обработчики gRPC выполняются в горутинах сервера.
		grpc.UnaryInterceptor(grpcsrv.LoggingInterceptor(log)),
		grpc.ChainStreamInterceptor(grpcsrv.StreamLoggingInterceptor(log)),
	)
	apiTokens.RegisterApiTokenServer(server, grpcsrv.New(store, log, cfg.MaxTokensPerUser))
	reflection.Register(server)

	errCh := make(chan error, 1)
	go func() {
		log.Info("gRPC-сервер запущен", "addr", cfg.Addr())
		if err := server.Serve(lis); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("получен сигнал завершения")
	}

	// Контекст приложения к этому моменту отменён, поэтому таймаут задаётся
	// отдельным.
	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		log.Info("сервер остановлен")
		return nil
	case <-time.After(cfg.ShutdownTimeout):
		server.Stop()
		log.Warn("сервер остановлен принудительно")
		return <-errCh
	}
}

// migrationsDir возвращает путь к каталогу миграций.
func migrationsDir() string {
	if dir := os.Getenv("MIGRATIONS_DIR"); dir != "" {
		return dir
	}
	return "db/migrations"
}
