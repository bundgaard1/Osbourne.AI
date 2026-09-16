package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	authcommon "osbourne.local/auth-common"
	coursecontent "osbourne.local/course-content-service/gen/course-content"
	"osbourne.local/course-content-service/internal/database"
	"osbourne.local/course-content-service/internal/repository"
	"osbourne.local/course-content-service/internal/server"
	"osbourne.local/course-content-service/internal/service"
)

func main() {
	authcommon.SetupLogging("course-content-service")

	port := "50054"

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("could not listen", "port", port, "err", err)
		os.Exit(1)
	}

	dbDir := "content-db"
	if envDbDir := os.Getenv("NOSQL_PATH"); envDbDir != "" {
		dbDir = envDbDir
	}

	cloverdb, err := database.NewCloverDB(dbDir)
	if err != nil {
		slog.Error("failed to connect to CloverDB", "err", err)
		os.Exit(1)
	}
	err = database.SeedCloverData(cloverdb)
	if err != nil {
		slog.Error("failed to seed CloverDB", "err", err)
		os.Exit(1)
	}

	courseContentRepo, err := repository.NewCloverModuleRepository(cloverdb, "modules")
	if err != nil {
		slog.Error("failed to create module repository", "err", err)
		os.Exit(1)
	}
	courseContentSvc := service.NewModuleService(courseContentRepo)
	courseContentGrpcServer := server.NewContentServer(courseContentSvc)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authcommon.AuthInterceptor(jwtSecret)),
	)

	coursecontent.RegisterCourseContentServiceServer(
		grpcServer,
		courseContentGrpcServer)

	go func() {
		slog.Info("course-content-service (gRPC) running", "port", port)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("failed to serve gRPC server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down gRPC server")

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("gRPC server stopped gracefully")
	case <-time.After(5 * time.Second):
		slog.Warn("timeout reached, forcing gRPC server shutdown")
		grpcServer.Stop()
	}
}