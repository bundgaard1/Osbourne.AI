package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"osbourne.local/auth-service/gen/auth"
	"osbourne.local/auth-service/internal/database"
	"osbourne.local/auth-service/internal/publisher"
	"osbourne.local/auth-service/internal/repository"
	"osbourne.local/auth-service/internal/server"
	"osbourne.local/auth-service/internal/service"
	"osbourne.local/common"
)

func main() {
	common.SetupLogging("auth-service")

	port := getEnv("PORT", "50056")
	dbPath := getEnv("DB_PATH", "./auth.db")
	amqpURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-change-me")
	tokenTTL := time.Duration(getEnvInt("TOKEN_TTL_MINUTES", 120)) * time.Minute

	// Loopback only: the REST listener and the gRPC server are the same process
	// and the traffic never leaves the container, so there is nothing to encrypt
	// and nothing to authenticate. Every other service uses the same pair.
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("could not listen", "port", port, "err", err)
		os.Exit(1)
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		slog.Error("database error", "err", err)
		os.Exit(1)
	}

	accountRepo := repository.NewGORMAccountRepository(db)

	conn, err := rabbitmq.NewConn(amqpURL)
	if err != nil {
		slog.Error("error creating RabbitMQ connection", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	pub, err := publisher.New(conn)
	if err != nil {
		slog.Error("error creating RabbitMQ publisher", "err", err)
		os.Exit(1)
	}
	defer pub.Close()

	for _, event := range database.SeedData(db) {
		if err := pub.PublishAccountCreated(context.Background(), event); err != nil {
			slog.Warn("failed to publish account.created event", "account_id", event.AccountID, "err", err)
		}
	}

	authSvc := service.NewAuthService(accountRepo, service.Config{
		JWTSecret: jwtSecret,
		TokenTTL:  tokenTTL,
	})
	authGrpcServer := server.NewAuthServer(authSvc, tokenTTL)

	// The service authenticates nobody: Login, ValidateToken and Logout are all
	// reachable without a token, which is the point of them. Every other
	// service installs common.AuthInterceptor.
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(common.RequestLoggerInterceptor()),
	)
	auth.RegisterAuthServiceServer(grpcServer, authGrpcServer)

	go func() {
		slog.Info("auth-service (gRPC) running", "port", port)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			slog.Error("error while running gRPC server", "err", err)
			os.Exit(1)
		}
	}()

	// The REST listener dials this same gRPC server over loopback rather than
	// calling the implementation in-process, so common.AuthInterceptor and
	// common.RequestLoggerInterceptor still apply to browser traffic.
	gateway, err := common.NewGateway(func(mux *runtime.ServeMux) error {
		return auth.RegisterAuthServiceHandlerFromEndpoint(
			context.Background(), mux, "localhost:"+port, dialOpts,
		)
	},
		// The token is delivered to the browser as an HttpOnly cookie. Leaving
		// it in the JSON body as well would mean any XSS on the login page could
		// read it straight out of the response, which defeats the point of
		// HttpOnly. The gRPC path keeps the token: the frontend's SSR handlers
		// call Login over gRPC and need it, and this option only runs on the
		// HTTP response.
		runtime.WithForwardResponseOption(redactLoginToken),
	)
	if err != nil {
		slog.Error("could not start the auth REST gateway", "err", err)
		os.Exit(1)
	}

	go func() {
		if err := gateway.Serve(); err != nil {
			slog.Error("error while running the REST server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("received shutdown signal, shutting down gracefully")

	// REST is drained first: every in-flight request is still waiting on a
	// loopback gRPC call, so stopping gRPC first would fail them mid-translation.
	if err := gateway.ShutdownWithTimeout(); err != nil {
		slog.Warn("REST listener did not drain cleanly", "err", err)
	}

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

func redactLoginToken(_ context.Context, _ http.ResponseWriter, msg proto.Message) error {
	if resp, ok := msg.(*auth.LoginResponse); ok {
		resp.Token = ""
	}
	return nil
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
