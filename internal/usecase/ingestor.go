package usecase

import (
	"context"
	"log/slog"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.MarketIngestor = (*Ingestor)(nil)

type Ingestor struct {
	source ports.MarketDataSource
	snaps  ports.SnapshotWriter
	log    *slog.Logger
}

func NewIngestor(source ports.MarketDataSource, snaps ports.SnapshotWriter, log *slog.Logger) *Ingestor {
	return &Ingestor{
		source: source,
		snaps:  snaps,
		log:    log,
	}
}

func (i *Ingestor) Ingest(ctx context.Context) ([]domain.MarketSnapshot, error) {
	snaps, err := i.source.FetchAll(ctx)
	if err != nil {
		return nil, err
	}

	if err := i.snaps.SaveAll(ctx, snaps); err != nil {
		if i.log != nil {
			i.log.Error("failed to save market snapshots", "error", err)
		}
	}

	return snaps, nil
}
