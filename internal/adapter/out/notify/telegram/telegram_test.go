package telegram_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"premark/internal/adapter/out/notify/telegram"
	"premark/internal/domain"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTelegramNotifier(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	sig := domain.Signal{
		ID:             "sig-1",
		Owner:          "chat-12345",
		Symbol:         "ANTHROPIC",
		APIPremiumBps:  -500.0,
		ExecPremiumBps: -480.0,
		Quote: domain.Quote{
			InAmount:  100 * domain.USDCUnit,
			OutTokens: 0.1,
			SwapURL:   "https://jup.ag/swap/USDC-ANTHROPIC",
		},
		CreatedAt: now,
	}

	t.Run("empty bot token returns nil without making request", func(t *testing.T) {
		n := telegram.New("", nil)
		err := n.Notify(ctx, sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("sends formatted telegram message to owner chat ID", func(t *testing.T) {
		var interceptedURL string
		var capturedBody map[string]any

		mockClient := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				interceptedURL = req.URL.String()
				bodyBytes, _ := io.ReadAll(req.Body)
				_ = json.Unmarshal(bodyBytes, &capturedBody)

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`))),
					Header:     make(http.Header),
				}, nil
			}),
		}

		n := telegram.New("my-bot-token", mockClient)
		err := n.Notify(ctx, sig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedURL := "https://api.telegram.org/botmy-bot-token/sendMessage"
		if interceptedURL != expectedURL {
			t.Errorf("expected URL %s, got %s", expectedURL, interceptedURL)
		}

		if capturedBody["chat_id"] != "chat-12345" {
			t.Errorf("expected chat_id='chat-12345', got %v", capturedBody["chat_id"])
		}

		text, ok := capturedBody["text"].(string)
		if !ok || !strings.Contains(text, "ANTHROPIC") || !strings.Contains(text, "https://jup.ag/swap/USDC-ANTHROPIC") {
			t.Errorf("message text missing expected fields: %s", text)
		}
	})

	t.Run("non-200 telegram status returns error", func(t *testing.T) {
		mockClient := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"ok":false}`))),
					Header:     make(http.Header),
				}, nil
			}),
		}

		n := telegram.New("invalid-token", mockClient)
		err := n.Notify(ctx, sig)
		if err == nil {
			t.Fatalf("expected error for 401 response, got nil")
		}
	})
}
