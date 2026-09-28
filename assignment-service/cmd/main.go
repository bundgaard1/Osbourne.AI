package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/assignment-service/internal/database"
	"osbourne.local/assignment-service/internal/httpapi"
	"osbourne.local/assignment-service/internal/publisher"
	"osbourne.local/assignment-service/internal/repository"
	"osbourne.local/assignment-service/internal/seed"
	"osbourne.local/assignment-service/internal/server"
	"osbourne.local/assignment-service/internal/service"
	"osbourne.local/common"
)

type Config struct {
	GRPCPort  string
	DBPath    string
	UploadDir string
	SeedData  bool
	JWTSecret string
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/assignment.db"
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	return Config{
		GRPCPort:  port,
		DBPath:    dbPath,
		UploadDir: uploadDir,
		SeedData:  os.Getenv("SEED_DATA") == "true",
		JWTSecret: jwtSecret,
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := loadConfig()

	db, err := database.NewGORMDB(cfg.DBPath)
	if err != nil {
		slog.Error("failed to connect to database", "err", err, "path", cfg.DBPath)
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("failed to get underlying sql.DB", "err", err)
		os.Exit(1)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()

	fileStore, err := repository.NewLocalFileStorage(cfg.UploadDir)
	if err != nil {
		slog.Error("failed to initialize file storage", "err", err, "dir", cfg.UploadDir)
		os.Exit(1)
	}

	if cfg.SeedData {
		if err := database.SeedGORMData(db); err != nil {
			slog.Error("failed to seed database", "err", err)
			os.Exit(1)
		}
		if err := seed.SeedFiles(context.Background(), fileStore); err != nil {
			slog.Error("failed to seed files", "err", err)
			os.Exit(1)
		}
		slog.Info("database and files successfully seeded")
	}

	assignmentRepo := repository.NewGORMAssignmentRepository(db)
	submissionRepo := repository.NewGORMSubmissionRepository(db)

	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@rabbitmq:5672/"
	}
	rmqConn, err := rabbitmq.NewConn(amqpURL)
	if err != nil {
		slog.Error("failed to create rabbitmq connection", "err", err)
		os.Exit(1)
	}
	defer rmqConn.Close()

	evPublisher, err := publisher.New(rmqConn)
	if err != nil {
		slog.Error("failed to create rabbitmq publisher", "err", err)
		os.Exit(1)
	}
	defer evPublisher.Close()

	appService := service.NewAssignmentService(assignmentRepo, submissionRepo, fileStore, evPublisher)
	grpcServerImpl := server.NewAssignmentServer(appService)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			common.AuthInterceptor(cfg.JWTSecret),
			common.RequestLoggerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			common.AuthStreamInterceptor(cfg.JWTSecret),
		),
	)
	assignment.RegisterAssignmentServiceServer(grpcServer, grpcServerImpl)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		slog.Error("failed to listen", "port", cfg.GRPCPort, "err", err)
		os.Exit(1)
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("assignment-service starting", "port", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			slog.Error("grpc server failed unexpectedly", "err", err)
			stop()
		}
	}()

	gateway, err := common.NewGateway(func(mux *runtime.ServeMux) error {
		if err := assignment.RegisterAssignmentServiceHandlerFromEndpoint(
			context.Background(), mux, "localhost:"+cfg.GRPCPort,
			[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		); err != nil {
			return err
		}
		return httpapi.InstallRoutes(context.Background(), mux, "localhost:"+cfg.GRPCPort)
	})
	if err != nil {
		slog.Error("could not start the assignment REST gateway", "err", err)
		os.Exit(1)
	}

	go func() {
		if err := gateway.Serve(); err != nil {
			slog.Error("error while running the REST server", "err", err)
			os.Exit(1)
		}
	}()

	<-shutdownCtx.Done()
	slog.Info("shutting down assignment-service...")

	if err := gateway.ShutdownWithTimeout(); err != nil {
		slog.Warn("REST listener did not drain cleanly", "err", err)
	}

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		slog.Info("server stopped cleanly")
	case <-time.After(10 * time.Second):
		slog.Warn("graceful shutdown timed out; forcing stop")
		grpcServer.Stop()
	}
}
