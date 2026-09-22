package ports

import (
	"context"
	"time"

	"premark/internal/domain"
)

// MarketDataSource returns the current state of every PreStock. Errors wrap domain.ErrUpstream.
type MarketDataSource interface {
	FetchAll(ctx context.Context) ([]domain.MarketSnapshot, error)
}

// QuoteProvider returns a live executable BUY quote for `budget` USDC of the token.
// Errors wrap domain.ErrQuoteUnavailable (no route, bad response) or domain.ErrUpstream (network/HTTP failure).
type QuoteProvider interface {
	QuoteBuy(ctx context.Context, token domain.Token, budget domain.USDC) (domain.Quote, error)
}

// RuleRepository contract:
//
//	Save    = upsert by ID.
//	Get     = domain.ErrRuleNotFound (wrapped) if missing.
//	ListByOwner = all rules of the owner, ordered by CreatedAt ascending then ID ascending.
//	ListEnabled = all rules with Enabled==true across owners, ordered by CreatedAt ascending then ID ascending.
//	Delete  = domain.ErrRuleNotFound (wrapped) if missing.
type RuleRepository interface {
	Save(ctx context.Context, r domain.Rule) error
	Get(ctx context.Context, id domain.RuleID) (domain.Rule, error)
	ListByOwner(ctx context.Context, owner domain.OwnerID) ([]domain.Rule, error)
	ListEnabled(ctx context.Context) ([]domain.Rule, error)
	Delete(ctx context.Context, id domain.RuleID) error
}

type SnapshotWriter interface {
	SaveAll(ctx context.Context, snaps []domain.MarketSnapshot) error
}

// SnapshotReader.History returns snapshots for the symbol with ObservedAt >= since, oldest first.
// Empty result is not an error.
type SnapshotReader interface {
	History(ctx context.Context, symbol domain.Symbol, since time.Time) ([]domain.MarketSnapshot, error)
}

type SnapshotRepository interface {
	SnapshotWriter
	SnapshotReader
}

// SignalRepository.ListByOwner returns newest first (CreatedAt desc, then ID desc), at most `limit`.
type SignalRepository interface {
	Save(ctx context.Context, s domain.Signal) error
	ListByOwner(ctx context.Context, owner domain.OwnerID, limit int) ([]domain.Signal, error)
}

type Notifier interface {
	Notify(ctx context.Context, s domain.Signal) error
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}
