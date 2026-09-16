package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/grpc"
	authcommon "osbourne.local/auth-common"
	coursecatalogue "osbourne.local/course-catalogue-service/gen/course-catalogue"
	"osbourne.local/course-catalogue-service/internal/database"
	"osbourne.local/course-catalogue-service/internal/publisher"
	"osbourne.local/course-catalogue-service/internal/repository"
	"osbourne.local/course-catalogue-service/internal/server"
	"osbourne.local/course-catalogue-service/internal/service"
)

func main() {
	authcommon.SetupLogging("course-catalogue-service")

	port := os.Getenv("PORT")
	if port == "" {
		port = "50053"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("could not listen", "port", port, "err", err)
		os.Exit(1)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./courses.db"
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		slog.Error("could not connect to database", "err", err)
		os.Exit(1)
	}

	database.SeedData(db)

	coursecatalogueRepo := repository.NewGORMCourseCatalogueRepository(db)

	// RabbitMQ connection for publishing domain events (course.enrolled).
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

	pub, err := publisher.New(conn)
	if err != nil {
		slog.Error("error creating RabbitMQ publisher", "err", err)
		os.Exit(1)
	}
	defer pub.Close()

	coursecatalogueSvc := service.NewCourseService(coursecatalogueRepo, pub)
	coursecatalogueGrpcServer := server.NewCourseServer(coursecatalogueSvc)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authcommon.AuthInterceptor(jwtSecret)),
	)

	coursecatalogue.RegisterCourseCatalogueServiceServer(
		grpcServer,
		coursecatalogueGrpcServer)

	go func() {
		slog.Info("course-catalogue-service (gRPC) running", "port", port)
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