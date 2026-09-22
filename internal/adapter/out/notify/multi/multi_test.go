package multi_test

import (
	"context"
	"errors"
	"testing"

	"premark/internal/adapter/out/notify/multi"
	"premark/internal/domain"
)

type mockNotifier struct {
	called bool
	err    error
}

func (m *mockNotifier) Notify(ctx context.Context, s domain.Signal) error {
	m.called = true
	return m.err
}

func TestMultiNotifier(t *testing.T) {
	ctx := context.Background()
	sig := domain.Signal{ID: "sig-1"}

	t.Run("all notifiers succeed", func(t *testing.T) {
		m1 := &mockNotifier{}
		m2 := &mockNotifier{}
		m := multi.New(m1, m2)

		err := m.Notify(ctx, sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !m1.called || !m2.called {
			t.Errorf("expected both notifiers called")
		}
	})

	t.Run("collects and joins errors", func(t *testing.T) {
		err1 := errors.New("err1")
		err2 := errors.New("err2")
		m1 := &mockNotifier{err: err1}
		m2 := &mockNotifier{err: err2}
		m := multi.New(m1, m2)

		err := m.Notify(ctx, sig)
		if err == nil {
			t.Fatalf("expected joined error, got nil")
		}
		if !errors.Is(err, err1) || !errors.Is(err, err2) {
			t.Errorf("expected joined error containing both err1 and err2, got %v", err)
		}
	})
}
