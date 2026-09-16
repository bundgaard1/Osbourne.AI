package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"osbourne.local/common"
	"osbourne.local/frontend/internal/app"
)

func main() {
	common.SetupLogging("frontend")

	cfg := app.Config{
		Port:                       getEnv("PORT", "8080"),
		AuthServiceAddr:            getEnv("AUTH_SERVICE_ADDR", "dns:///auth-service:50056"),
		ProfileServiceAddr:         getEnv("PROFILE_SERVICE_ADDR", "dns:///profile-service:50051"),
		NotificationServiceAddr:    getEnv("NOTIFICATION_SERVICE_ADDR", "dns:///notification-service:50052"),
		CourseCatalogueServiceAddr: getEnv("COURSE_CATALOGUE_SERVICE_ADDR", "dns:///course-catalogue-service:50053"),
		CourseContentServiceAddr:   getEnv("COURSE_CONTENT_SERVICE_ADDR", "dns:///course-content-service:50054"),
		AssignmentServiceAddr:      getEnv("ASSIGNMENT_SERVICE_ADDR", "dns:///assignment-service:50055"),
		JWTSecret:                  getEnv("JWT_SECRET", "dev-secret-change-me"),
	}

	application, err := app.NewApp(cfg)
	if err != nil {
		slog.Error("could not create frontend app", "err", err)
		os.Exit(1)
	}

	go func() {
		if err := application.Run(); err != nil {
			slog.Error("error while running frontend", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutdown signal received")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	application.Stop(ctx)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
