package testsupport

import (
	"context"
	"fmt"
	"sync"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.MarketDataSource = (*FakeSource)(nil)

type FakeSource struct {
	mu        sync.Mutex
	Snapshots []domain.MarketSnapshot
	Err       error
	Calls     int
}

func (f *FakeSource) FetchAll(ctx context.Context) ([]domain.MarketSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls++
	if f.Err != nil {
		return nil, f.Err
	}
	res := make([]domain.MarketSnapshot, len(f.Snapshots))
	copy(res, f.Snapshots)
	return res, nil
}

var _ ports.QuoteProvider = (*FakeQuoter)(nil)

type QuoteCall struct {
	Token  domain.Token
	Budget domain.USDC
}

type FakeQuoter struct {
	mu     sync.Mutex
	quotes map[domain.Symbol]domain.Quote
	errors map[domain.Symbol]error
	Err    error
	Calls  []QuoteCall
}

func NewFakeQuoter() *FakeQuoter {
	return &FakeQuoter{
		quotes: make(map[domain.Symbol]domain.Quote),
		errors: make(map[domain.Symbol]error),
	}
}

func (q *FakeQuoter) SetQuote(sym domain.Symbol, quote domain.Quote) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.quotes[sym] = quote
}

func (q *FakeQuoter) SetError(sym domain.Symbol, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.errors[sym] = err
}

func (q *FakeQuoter) QuoteBuy(ctx context.Context, token domain.Token, budget domain.USDC) (domain.Quote, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Calls = append(q.Calls, QuoteCall{Token: token, Budget: budget})
	if q.Err != nil {
		return domain.Quote{}, q.Err
	}
	if err, ok := q.errors[token.Symbol]; ok && err != nil {
		return domain.Quote{}, err
	}
	if quote, ok := q.quotes[token.Symbol]; ok {
		return quote, nil
	}
	return domain.Quote{}, fmt.Errorf("%w: no quote for %s", domain.ErrQuoteUnavailable, token.Symbol)
}

var _ ports.Notifier = (*FakeNotifier)(nil)

type FakeNotifier struct {
	mu      sync.Mutex
	Signals []domain.Signal
	Err     error
}

func (f *FakeNotifier) Notify(ctx context.Context, s domain.Signal) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Signals = append(f.Signals, s)
	return f.Err
}
