package logger

import (
	"context"
	"log/slog"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.Notifier = (*Notifier)(nil)

type Notifier struct {
	log *slog.Logger
}

func New(log *slog.Logger) *Notifier {
	return &Notifier{
		log: log,
	}
}

func (n *Notifier) Notify(ctx context.Context, s domain.Signal) error {
	if n.log != nil {
		n.log.Info("signal triggered",
			"symbol", s.Symbol,
			"owner", s.Owner,
			"api_premium_bps", s.APIPremiumBps,
			"exec_premium_bps", s.ExecPremiumBps,
			"budget", s.Quote.InAmount.Float64(),
			"swap_url", s.Quote.SwapURL,
		)
	}
	return nil
}
