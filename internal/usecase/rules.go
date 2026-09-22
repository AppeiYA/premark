package usecase

import (
	"context"
	"fmt"
	"strings"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.RuleManager = (*RuleService)(nil)

type RuleService struct {
	rules  ports.RuleRepository
	source ports.MarketDataSource
	clock  ports.Clock
	ids    ports.IDGenerator
}

func NewRuleService(rules ports.RuleRepository, source ports.MarketDataSource,
	clock ports.Clock, ids ports.IDGenerator) *RuleService {
	return &RuleService{
		rules:  rules,
		source: source,
		clock:  clock,
		ids:    ids,
	}
}

func (s *RuleService) CreateRule(ctx context.Context, p domain.NewRuleParams) (domain.Rule, error) {
	now := s.clock.Now()

	// Validate rule parameters first without consuming an ID or fetching upstream.
	rule, err := domain.NewRule("", p, now)
	if err != nil {
		return domain.Rule{}, err
	}

	snaps, err := s.source.FetchAll(ctx)
	if err != nil {
		return domain.Rule{}, err
	}

	found := false
	for _, snap := range snaps {
		if snap.Token.Symbol == rule.Symbol {
			found = true
			break
		}
	}
	if !found {
		return domain.Rule{}, fmt.Errorf("%w: token %s not found in market", domain.ErrTokenNotFound, rule.Symbol)
	}

	rule.ID = domain.RuleID(s.ids.NewID())
	if err := s.rules.Save(ctx, rule); err != nil {
		return domain.Rule{}, err
	}

	return rule, nil
}

func (s *RuleService) ListRules(ctx context.Context, owner domain.OwnerID) ([]domain.Rule, error) {
	if strings.TrimSpace(string(owner)) == "" {
		return nil, fmt.Errorf("%w: owner is required", domain.ErrInvalidArgument)
	}
	return s.rules.ListByOwner(ctx, owner)
}

func (s *RuleService) SetRuleEnabled(ctx context.Context, owner domain.OwnerID, id domain.RuleID, enabled bool) (domain.Rule, error) {
	if strings.TrimSpace(string(owner)) == "" {
		return domain.Rule{}, fmt.Errorf("%w: owner is required", domain.ErrInvalidArgument)
	}

	rule, err := s.rules.Get(ctx, id)
	if err != nil {
		return domain.Rule{}, err
	}

	if !rule.OwnedBy(owner) {
		return domain.Rule{}, fmt.Errorf("%w: rule %s", domain.ErrRuleNotFound, id)
	}

	updated := rule.WithEnabled(enabled)
	if err := s.rules.Save(ctx, updated); err != nil {
		return domain.Rule{}, err
	}

	return updated, nil
}

func (s *RuleService) DeleteRule(ctx context.Context, owner domain.OwnerID, id domain.RuleID) error {
	if strings.TrimSpace(string(owner)) == "" {
		return fmt.Errorf("%w: owner is required", domain.ErrInvalidArgument)
	}

	rule, err := s.rules.Get(ctx, id)
	if err != nil {
		return err
	}

	if !rule.OwnedBy(owner) {
		return fmt.Errorf("%w: rule %s", domain.ErrRuleNotFound, id)
	}

	return s.rules.Delete(ctx, id)
}
