package multi

import (
	"context"
	"errors"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.Notifier = (*Notifier)(nil)

type Notifier struct {
	notifiers []ports.Notifier
}

func New(ns ...ports.Notifier) *Notifier {
	return &Notifier{
		notifiers: ns,
	}
}

func (m *Notifier) Notify(ctx context.Context, s domain.Signal) error {
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Notify(ctx, s); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
