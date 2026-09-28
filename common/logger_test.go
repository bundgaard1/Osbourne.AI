package common

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

)

// The handler enriches every record with request_id and user_id from the
// context, and several call sites also pass them explicitly. When both happen
// the output has the same key twice, which is valid JSON but ambiguous: a log
// pipeline keeping the first occurrence and one keeping the last will disagree
// about what the request id was.
func TestContextHandlerDoesNotDuplicateAnExplicitKey(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(&contextHandler{
		handler: slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}).With("service", "test")

	ctx := WithRequestID(context.Background(), "req-abc123")
	ctx = WithClaims(ctx, &Claims{UserID: "user-1", Email: "u@example.com", Role: "student"})

	logger.InfoContext(ctx, "explicit wins", "request_id", "from-callsite")

	// Decoding into a map collapses duplicates, so count the raw key instead.
	if got := strings.Count(buf.String(), `"request_id":`); got != 1 {
		t.Errorf("request_id appears %d times, want 1: %s", got, buf.String())
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got["request_id"] != "from-callsite" {
		t.Errorf("request_id = %v, want the call site's value; enrichment must not override it", got["request_id"])
	}
	if got["user_id"] != "user-1" {
		t.Errorf("user_id = %v, want user-1 filled in from the context", got["user_id"])
	}
}

// The flip side: with nothing set by the call site, the context still supplies
// both, and only once.
func TestContextHandlerFillsInFromContext(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(&contextHandler{
		handler: slog.NewJSONHandler(&buf, nil),
	}).With("service", "test")

	ctx := WithRequestID(context.Background(), "req-xyz789")
	ctx = WithClaims(ctx, &Claims{UserID: "user-2"})

	logger.InfoContext(ctx, "plain message")

	if got := strings.Count(buf.String(), `"request_id":`); got != 1 {
		t.Errorf("request_id appears %d times, want 1: %s", got, buf.String())
	}
	if got := strings.Count(buf.String(), `"user_id":`); got != 1 {
		t.Errorf("user_id appears %d times, want 1: %s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "req-xyz789") || !strings.Contains(buf.String(), "user-2") {
		t.Errorf("context values missing from %s", buf.String())
	}
}

// A context with nothing in it must not sprout empty keys, which is what made
// the duplicate-detection above worth having at all.
func TestContextHandlerAddsNothingWithoutContext(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(&contextHandler{handler: slog.NewJSONHandler(&buf, nil)})

	logger.InfoContext(context.Background(), "bare")

	if strings.Contains(buf.String(), "request_id") || strings.Contains(buf.String(), "user_id") {
		t.Errorf("bare record gained context keys: %s", buf.String())
	}
}
