package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.SnapshotRepository = (*SnapshotRepository)(nil)

type SnapshotRepository struct {
	db *sql.DB
}

func NewSnapshotRepository(db *sql.DB) *SnapshotRepository {
	return &SnapshotRepository{db: db}
}

type snapshotRow struct {
	symbol           string
	observedMs       int64
	name             string
	description      string
	mint             string
	tokenPrice       float64
	markPrice        float64
	supply           float64
	markValuation    float64
	impliedValuation float64
}

func (s snapshotRow) toDomain() domain.MarketSnapshot {
	return domain.MarketSnapshot{
		Token: domain.Token{
			Symbol:      domain.Symbol(s.symbol),
			Name:        s.name,
			Description: s.description,
			Mint:        s.mint,
		},
		TokenPrice:       s.tokenPrice,
		MarkPrice:        s.markPrice,
		Supply:           s.supply,
		MarkValuation:    s.markValuation,
		ImpliedValuation: s.impliedValuation,
		ObservedAt:       time.UnixMilli(s.observedMs).UTC(),
	}
}

func (repo *SnapshotRepository) SaveAll(ctx context.Context, snaps []domain.MarketSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO snapshots (symbol, observed_ms, name, description, mint, token_price, mark_price, supply, mark_valuation, implied_valuation)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("sqlite prepare snapshot insert: %w", err)
	}
	defer stmt.Close()

	for _, s := range snaps {
		if _, err := stmt.ExecContext(ctx,
			string(s.Token.Symbol),
			s.ObservedAt.UnixMilli(),
			s.Token.Name,
			s.Token.Description,
			s.Token.Mint,
			s.TokenPrice,
			s.MarkPrice,
			s.Supply,
			s.MarkValuation,
			s.ImpliedValuation,
		); err != nil {
			return fmt.Errorf("sqlite insert snapshot %s: %w", s.Token.Symbol, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite commit snapshots: %w", err)
	}
	return nil
}

func (repo *SnapshotRepository) History(ctx context.Context, symbol domain.Symbol, since time.Time) ([]domain.MarketSnapshot, error) {
	query := `
SELECT symbol, observed_ms, name, description, mint, token_price, mark_price, supply, mark_valuation, implied_valuation
FROM snapshots
WHERE symbol = ? AND observed_ms >= ?
ORDER BY observed_ms ASC;`

	rows, err := repo.db.QueryContext(ctx, query, string(symbol), since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("sqlite history query: %w", err)
	}
	defer rows.Close()

	result := make([]domain.MarketSnapshot, 0)
	for rows.Next() {
		var row snapshotRow
		if err := rows.Scan(
			&row.symbol,
			&row.observedMs,
			&row.name,
			&row.description,
			&row.mint,
			&row.tokenPrice,
			&row.markPrice,
			&row.supply,
			&row.markValuation,
			&row.impliedValuation,
		); err != nil {
			return nil, fmt.Errorf("scan snapshot row: %w", err)
		}
		result = append(result, row.toDomain())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
