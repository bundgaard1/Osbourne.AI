package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/grpc"

	"osbourne.local/auth-service/gen/auth"
	"osbourne.local/auth-service/internal/database"
	"osbourne.local/auth-service/internal/publisher"
	"osbourne.local/auth-service/internal/repository"
	"osbourne.local/auth-service/internal/server"
	"osbourne.local/auth-service/internal/service"
)

func main() {
	port := getEnv("PORT", "50056")
	dbPath := getEnv("DB_PATH", "./auth.db")
	amqpURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-change-me")
	tokenTTL := time.Duration(getEnvInt("TOKEN_TTL_MINUTES", 120)) * time.Minute

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Could not listen on port :%s: %v", port, err)
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}

	accountRepo := repository.NewGORMAccountRepository(db)

	conn, err := rabbitmq.NewConn(amqpURL)
	if err != nil {
		log.Fatalf("Error creating RabbitMQ connection: %v", err)
	}
	defer conn.Close()

	pub, err := publisher.New(conn)
	if err != nil {
		log.Fatalf("Error creating RabbitMQ publisher: %v", err)
	}
	defer pub.Close()

	// Seed demo accounts and fire an account.created event for each one so the
	// profile and notification services react to them like any new account.
	for _, event := range database.SeedData(db) {
		if err := pub.PublishAccountCreated(context.Background(), event); err != nil {
			log.Printf("Failed to publish account.created event: %v", err)
		}
	}

	authSvc := service.NewAuthService(accountRepo, service.Config{
		JWTSecret: jwtSecret,
		TokenTTL:  tokenTTL,
	})
	authGrpcServer := server.NewAuthServer(authSvc)

	grpcServer := grpc.NewServer()
	auth.RegisterAuthServiceServer(grpcServer, authGrpcServer)

	go func() {
		log.Printf("auth-service (gRPC) running on port :%s...", port)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Fatalf("Error while running gRPC server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Received shutdown signal. Shutting down gracefully...")

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Println("gRPC server shut down gracefully.")
	case <-time.After(5 * time.Second):
		log.Println("Timeout exceeded - forcing shutdown.")
		grpcServer.Stop()
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}
