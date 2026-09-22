package testsupport

import (
	"context"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.RuleRepository = (*FailingRuleRepo)(nil)

type FailingRuleRepo struct {
	ports.RuleRepository
	FailSave        error
	FailGet         error
	FailListByOwner error
	FailListEnabled error
	FailDelete      error
}

func NewFailingRuleRepo(underlying ports.RuleRepository) *FailingRuleRepo {
	return &FailingRuleRepo{RuleRepository: underlying}
}

func (f *FailingRuleRepo) Save(ctx context.Context, r domain.Rule) error {
	if f.FailSave != nil {
		return f.FailSave
	}
	return f.RuleRepository.Save(ctx, r)
}

func (f *FailingRuleRepo) Get(ctx context.Context, id domain.RuleID) (domain.Rule, error) {
	if f.FailGet != nil {
		return domain.Rule{}, f.FailGet
	}
	return f.RuleRepository.Get(ctx, id)
}

func (f *FailingRuleRepo) ListByOwner(ctx context.Context, owner domain.OwnerID) ([]domain.Rule, error) {
	if f.FailListByOwner != nil {
		return nil, f.FailListByOwner
	}
	return f.RuleRepository.ListByOwner(ctx, owner)
}

func (f *FailingRuleRepo) ListEnabled(ctx context.Context) ([]domain.Rule, error) {
	if f.FailListEnabled != nil {
		return nil, f.FailListEnabled
	}
	return f.RuleRepository.ListEnabled(ctx)
}

func (f *FailingRuleRepo) Delete(ctx context.Context, id domain.RuleID) error {
	if f.FailDelete != nil {
		return f.FailDelete
	}
	return f.RuleRepository.Delete(ctx, id)
}

var _ ports.SnapshotWriter = (*FailingSnapshotWriter)(nil)

type FailingSnapshotWriter struct {
	ports.SnapshotWriter
	FailSaveAll error
}

func NewFailingSnapshotWriter(underlying ports.SnapshotWriter) *FailingSnapshotWriter {
	return &FailingSnapshotWriter{SnapshotWriter: underlying}
}

func (f *FailingSnapshotWriter) SaveAll(ctx context.Context, snaps []domain.MarketSnapshot) error {
	if f.FailSaveAll != nil {
		return f.FailSaveAll
	}
	return f.SnapshotWriter.SaveAll(ctx, snaps)
}

var _ ports.SnapshotRepository = (*FailingSnapshotRepo)(nil)

type FailingSnapshotRepo struct {
	ports.SnapshotRepository
	FailSaveAll error
	FailHistory error
}

func NewFailingSnapshotRepo(underlying ports.SnapshotRepository) *FailingSnapshotRepo {
	return &FailingSnapshotRepo{SnapshotRepository: underlying}
}

func (f *FailingSnapshotRepo) SaveAll(ctx context.Context, snaps []domain.MarketSnapshot) error {
	if f.FailSaveAll != nil {
		return f.FailSaveAll
	}
	return f.SnapshotRepository.SaveAll(ctx, snaps)
}

func (f *FailingSnapshotRepo) History(ctx context.Context, symbol domain.Symbol, since time.Time) ([]domain.MarketSnapshot, error) {
	if f.FailHistory != nil {
		return nil, f.FailHistory
	}
	return f.SnapshotRepository.History(ctx, symbol, since)
}

var _ ports.SignalRepository = (*FailingSignalRepo)(nil)

type FailingSignalRepo struct {
	ports.SignalRepository
	FailSave        error
	FailListByOwner error
}

func NewFailingSignalRepo(underlying ports.SignalRepository) *FailingSignalRepo {
	return &FailingSignalRepo{SignalRepository: underlying}
}

func (f *FailingSignalRepo) Save(ctx context.Context, s domain.Signal) error {
	if f.FailSave != nil {
		return f.FailSave
	}
	return f.SignalRepository.Save(ctx, s)
}

func (f *FailingSignalRepo) ListByOwner(ctx context.Context, owner domain.OwnerID, limit int) ([]domain.Signal, error) {
	if f.FailListByOwner != nil {
		return nil, f.FailListByOwner
	}
	return f.SignalRepository.ListByOwner(ctx, owner, limit)
}
