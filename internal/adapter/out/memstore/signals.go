package memstore

import (
	"context"
	"sort"
	"sync"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.SignalRepository = (*SignalRepository)(nil)

type SignalRepository struct {
	mu      sync.RWMutex
	signals []domain.Signal
}

func NewSignalRepository() *SignalRepository {
	return &SignalRepository{
		signals: make([]domain.Signal, 0),
	}
}

func (s *SignalRepository) Save(ctx context.Context, signal domain.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals = append(s.signals, signal)
	return nil
}

func (s *SignalRepository) ListByOwner(ctx context.Context, owner domain.OwnerID, limit int) ([]domain.Signal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]domain.Signal, 0)
	for _, sig := range s.signals {
		if sig.Owner == owner {
			matched = append(matched, sig)
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID > matched[j].ID
	})

	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}

	return matched, nil
}
