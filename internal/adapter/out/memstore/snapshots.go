package memstore

import (
	"context"
	"sort"
	"sync"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.SnapshotRepository = (*SnapshotRepository)(nil)

type SnapshotRepository struct {
	mu        sync.RWMutex
	snapshots []domain.MarketSnapshot
}

func NewSnapshotRepository() *SnapshotRepository {
	return &SnapshotRepository{
		snapshots: make([]domain.MarketSnapshot, 0),
	}
}

func (s *SnapshotRepository) SaveAll(ctx context.Context, snaps []domain.MarketSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = append(s.snapshots, snaps...)
	return nil
}

func (s *SnapshotRepository) History(ctx context.Context, symbol domain.Symbol, since time.Time) ([]domain.MarketSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.MarketSnapshot, 0)
	for _, snap := range s.snapshots {
		if snap.Token.Symbol == symbol && !snap.ObservedAt.Before(since) {
			result = append(result, snap)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ObservedAt.Before(result[j].ObservedAt)
	})

	return result, nil
}
