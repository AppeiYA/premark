package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"premark/internal/domain"
	"premark/internal/ports"
)

var _ ports.Notifier = (*Notifier)(nil)

type Notifier struct {
	botToken string
	hc       *http.Client
}

func New(botToken string, hc *http.Client) *Notifier {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Notifier{
		botToken: botToken,
		hc:       hc,
	}
}

type sendMessagePayload struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

func (n *Notifier) Notify(ctx context.Context, s domain.Signal) error {
	if n.botToken == "" {
		return nil
	}

	apiPremPct := s.APIPremiumBps / 100.0
	execPremPct := s.ExecPremiumBps / 100.0

	text := fmt.Sprintf("PreMark Signal: %s\nAPI Premium: %.2f%%\nExec Premium: %.2f%%\nBudget: $%.2f\nExpected Tokens: %g\nSwap: %s",
		s.Symbol, apiPremPct, execPremPct, s.Quote.InAmount.Float64(), s.Quote.OutTokens, s.Quote.SwapURL)

	payload := sendMessagePayload{
		ChatID: string(s.Owner),
		Text:   text,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.botToken)
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.hc.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram api error status %d", resp.StatusCode)
	}

	return nil
}
