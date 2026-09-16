package authcommon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"google.golang.org/grpc/metadata"
)

// RequestIDMetadataKey is the gRPC metadata key used to propagate the
// x-request-id between the frontend and every service it calls.
const RequestIDMetadataKey = "x-request-id"

type requestIDKey struct{}

// WithRequestID stores the request (correlation) id in the context. Empty ids
// are ignored so callers can pass them through unconditionally.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request id stored in the context, if any.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// AttachRequestID appends the request id to the outgoing gRPC metadata so a
// single request can be followed from the frontend through every service it
// touches (and back).
func AttachRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, RequestIDMetadataKey, id)
}

// contextHandler enriches every log record with request_id and user_id pulled
// from the (interceptor-populated) context, so correlation ids show up without
// threading attributes through every call site. Use slog.InfoContext /
// slog.WarnContext / slog.ErrorContext with the request context to pick them up.
type contextHandler struct {
	handler slog.Handler
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, rec slog.Record) error {
	if rid := RequestIDFromContext(ctx); rid != "" {
		rec.AddAttrs(slog.String("request_id", rid))
	}
	if claims, ok := ClaimsFromContext(ctx); ok {
		rec.AddAttrs(slog.String("user_id", claims.UserID))
	}
	return h.handler.Handle(ctx, rec)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{handler: h.handler.WithGroup(name)}
}

// SetupLogging installs a JSON slog logger on stdout tagged with the given
// service name, honours the LOG_LEVEL env var (debug|info|warn|error) and sets
// it as slog's default logger. Every service calls this once in main().
func SetupLogging(service string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	logger := slog.New(&contextHandler{
		handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	}).With("service", service)
	slog.SetDefault(logger)
	return logger
}

// RabbitLogger implements github.com/wagslane/go-rabbitmq's Logger interface
// and forwards its internal chatter into slog so the JSON log stream stays
// pure. Pass it as WithPublisherOptionsLogger / WithConsumerOptionsLogger.
type RabbitLogger struct{}

func (RabbitLogger) Fatalf(format string, v ...interface{}) {
	slog.Error(fmt.Sprintf("gorabbit "+format, v...))
}
func (RabbitLogger) Errorf(format string, v ...interface{}) {
	slog.Error(fmt.Sprintf("gorabbit "+format, v...))
}
func (RabbitLogger) Warnf(format string, v ...interface{}) {
	slog.Warn(fmt.Sprintf("gorabbit "+format, v...))
}
func (RabbitLogger) Infof(format string, v ...interface{}) {
	slog.Info(fmt.Sprintf("gorabbit "+format, v...))
}
func (RabbitLogger) Debugf(format string, v ...interface{}) {
	slog.Debug(fmt.Sprintf("gorabbit "+format, v...))
}

// GormLogger is a slog-backed gorm.io/logger Interface. It keeps database
// messages as JSON, surfaces genuine errors only (dropping the expected
// RecordNotFound lookups) and never dumps raw SQL at production log levels.
type GormLogger struct {
	level logger.LogLevel
}

// NewGormLogger returns a GORM logger at Warn level (errors and slow queries
// only). Call LogMode to raise/lower per connection.
func NewGormLogger() logger.Interface {
	return &GormLogger{level: logger.Warn}
}

func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	slog.InfoContext(ctx, "gorm: "+msg, gormArgs(data...)...)
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	slog.WarnContext(ctx, "gorm: "+msg, gormArgs(data...)...)
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	slog.ErrorContext(ctx, "gorm: "+msg, gormArgs(data...)...)
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level < logger.Warn {
		return
	}

	elapsed := time.Since(begin).Milliseconds()

	// RecordNotFound drives "get or create" flows; it is not an error worth
	// alarming on unless the caller propagates it.
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.ErrorContext(ctx, "gorm query error", "err", err, "elapsed_ms", elapsed)
		return
	}

	if err == nil && l.level >= logger.Info {
		sql, rows := fc()
		slog.InfoContext(ctx, "gorm query", "rows", rows, "sql", sql, "elapsed_ms", elapsed)
	}
}

func gormArgs(data ...interface{}) []any {
	if len(data) == 0 {
		return nil
	}
	return []any{"details", data}
}