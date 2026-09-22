package system_test

import (
	"testing"
	"time"

	"premark/internal/adapter/out/system"
)

func TestSystem(t *testing.T) {
	t.Run("SystemClock Now returns UTC time close to current", func(t *testing.T) {
		clock := system.NewClock()
		now := clock.Now()
		if now.Location() != time.UTC {
			t.Errorf("expected UTC location, got %v", now.Location())
		}
		if time.Since(now) > time.Second {
			t.Errorf("clock.Now() returned time too far in past: %v", now)
		}
	})

	t.Run("RandomIDGenerator generates unique non-empty 32-hex IDs", func(t *testing.T) {
		gen := system.NewIDGenerator()
		id1 := gen.NewID()
		id2 := gen.NewID()

		if len(id1) != 32 {
			t.Errorf("expected 32 hex chars, got %d (%s)", len(id1), id1)
		}
		if id1 == id2 {
			t.Errorf("expected unique random IDs, got collision: %s", id1)
		}
	})
}
