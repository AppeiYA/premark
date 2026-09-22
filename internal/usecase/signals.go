package usecase

import (
	"context"
	"fmt"
	"strings"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.SignalReader = (*SignalQuery)(nil)

type SignalQuery struct {
	signals ports.SignalRepository
}

func NewSignalQuery(signals ports.SignalRepository) *SignalQuery {
	return &SignalQuery{
		signals: signals,
	}
}

func (q *SignalQuery) ListSignals(ctx context.Context, owner domain.OwnerID, limit int) ([]domain.Signal, error) {
	if strings.TrimSpace(string(owner)) == "" {
		return nil, fmt.Errorf("%w: owner is required", domain.ErrInvalidArgument)
	}

	if limit == 0 {
		limit = 20
	}
	if limit < 0 || limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100", domain.ErrInvalidArgument)
	}

	return q.signals.ListByOwner(ctx, owner, limit)
}
