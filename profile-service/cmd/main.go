package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/grpc"

	authcommon "osbourne.local/auth-common"
	"osbourne.local/profile-service/gen/profile"
	"osbourne.local/profile-service/internal/consumer"
	"osbourne.local/profile-service/internal/database"
	"osbourne.local/profile-service/internal/repository"
	"osbourne.local/profile-service/internal/server"
	"osbourne.local/profile-service/internal/service"
)

func main() {
	authcommon.SetupLogging("profile-service")

	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("could not listen", "port", port, "err", err)
		os.Exit(1)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./profiles.db"
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		slog.Error("database error", "err", err)
		os.Exit(1)
	}

	profileRepo := repository.NewGORMProfileRepository(db)
	profileSvc := service.NewProfileService(profileRepo)
	profileGrpcServer := server.NewProfileServer(profileSvc)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	// RabbitMQ consumer: profiles are created reactively from account.created
	// events, so subscribe before serving gRPC.
	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@rabbitmq:5672/"
	}
	conn, err := rabbitmq.NewConn(amqpURL)
	if err != nil {
		slog.Error("error creating RabbitMQ connection", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	profileConsumer, err := consumer.NewProfileConsumer(conn, profileSvc)
	if err != nil {
		slog.Error("error creating profile consumer", "err", err)
		os.Exit(1)
	}
	defer profileConsumer.Close()

	go func() {
		if err := profileConsumer.Start(context.Background()); err != nil {
			slog.Error("profile consumer stopped", "err", err)
		}
	}()

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authcommon.AuthInterceptor(jwtSecret)),
	)
	profile.RegisterProfileServiceServer(grpcServer, profileGrpcServer)

	go func() {
		slog.Info("profile-service (gRPC) running", "port", port)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			slog.Error("error while running gRPC server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("received shutdown signal, shutting down gracefully")

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("gRPC server shut down gracefully")
	case <-time.After(5 * time.Second):
		slog.Warn("timeout exceeded, forcing shutdown")
		grpcServer.Stop()
	}
}