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

	"google.golang.org/grpc"

	"osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/assignment-service/internal/database"
	"osbourne.local/assignment-service/internal/repository"
	"osbourne.local/assignment-service/internal/server"
	"osbourne.local/assignment-service/internal/service"
)

type Config struct {
	GRPCPort  string
	DBPath    string
	UploadDir string
	SeedData  bool
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

	return Config{
		GRPCPort:  port,
		DBPath:    dbPath,
		UploadDir: uploadDir,
		SeedData:  os.Getenv("SEED_DATA") == "true",
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := loadConfig()

	// 1. Database Initialization
	db, err := database.NewGORMDB(cfg.DBPath)
	if err != nil {
		slog.Error("failed to connect to database", "err", err, "path", cfg.DBPath)
		os.Exit(1)
	}

	// SQLite connection pooling constraint: single writer prevents "database is locked" errors
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("failed to get underlying sql.DB", "err", err)
		os.Exit(1)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()

	// 2. Controlled Seeding (Dev/Local only)
	if cfg.SeedData {
		if err := database.SeedGORMData(db); err != nil {
			slog.Error("failed to seed database", "err", err)
			os.Exit(1)
		}
		slog.Info("database successfully seeded")
	}

	// 3. Storage Setup
	fileStore, err := repository.NewLocalFileStorage(cfg.UploadDir)
	if err != nil {
		slog.Error("failed to initialize file storage", "err", err, "dir", cfg.UploadDir)
		os.Exit(1)
	}

	// 4. Dependency Injection
	assignmentRepo := repository.NewGORMAssignmentRepository(db)
	submissionRepo := repository.NewGORMSubmissionRepository(db)
	appService := service.NewAssignmentService(assignmentRepo, submissionRepo, fileStore)
	grpcServerImpl := server.NewAssignmentServer(appService)

	grpcServer := grpc.NewServer()
	assignment.RegisterAssignmentServiceServer(grpcServer, grpcServerImpl)

	// 5. Server Listener & Graceful Shutdown
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

	<-shutdownCtx.Done()
	slog.Info("shutting down assignment-service...")

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
