package logger_test

import (
	"context"
	"log/slog"
	"testing"

	"premark/internal/adapter/out/notify/logger"
	"premark/internal/domain"
)

func TestLoggerNotifier(t *testing.T) {
	ctx := context.Background()
	sig := domain.Signal{ID: "sig-1", Owner: "alice", Symbol: "ANTHROPIC"}

	t.Run("nil logger does not error", func(t *testing.T) {
		n := logger.New(nil)
		if err := n.Notify(ctx, sig); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid logger does not error", func(t *testing.T) {
		n := logger.New(slog.Default())
		if err := n.Notify(ctx, sig); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
