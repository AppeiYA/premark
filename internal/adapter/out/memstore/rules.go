package memstore

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.RuleRepository = (*RuleRepository)(nil)

type RuleRepository struct {
	mu    sync.RWMutex
	rules map[domain.RuleID]domain.Rule
}

func NewRuleRepository() *RuleRepository {
	return &RuleRepository{
		rules: make(map[domain.RuleID]domain.Rule),
	}
}

func (r *RuleRepository) Save(ctx context.Context, rule domain.Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rule.ID] = rule
	return nil
}

func (r *RuleRepository) Get(ctx context.Context, id domain.RuleID) (domain.Rule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rule, ok := r.rules[id]
	if !ok {
		return domain.Rule{}, fmt.Errorf("%w: rule %s", domain.ErrRuleNotFound, id)
	}
	return rule, nil
}

func (r *RuleRepository) ListByOwner(ctx context.Context, owner domain.OwnerID) ([]domain.Rule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Rule, 0)
	for _, rule := range r.rules {
		if rule.Owner == owner {
			result = append(result, rule)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.Before(result[j].CreatedAt)
		}
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func (r *RuleRepository) ListEnabled(ctx context.Context) ([]domain.Rule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Rule, 0)
	for _, rule := range r.rules {
		if rule.Enabled {
			result = append(result, rule)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.Before(result[j].CreatedAt)
		}
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func (r *RuleRepository) Delete(ctx context.Context, id domain.RuleID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.rules[id]; !ok {
		return fmt.Errorf("%w: rule %s", domain.ErrRuleNotFound, id)
	}
	delete(r.rules, id)
	return nil
}
