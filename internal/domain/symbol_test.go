package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSymbol(t *testing.T) {
	t.Run("valid symbols", func(t *testing.T) {
		valid := []struct {
			input    string
			expected Symbol
		}{
			{"AI", Symbol("AI")},
			{"anthropic", Symbol("ANTHROPIC")},
			{"  openai  ", Symbol("OPENAI")},
			{"12345", Symbol("12345")},
			{"POLYMARKET", Symbol("POLYMARKET")},
			{"1234567890123456", Symbol("1234567890123456")}, // 16 chars
		}

		for _, tc := range valid {
			sym, err := ParseSymbol(tc.input)
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.input, err)
			}
			if sym != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, sym)
			}
			if sym.String() != string(tc.expected) {
				t.Errorf("expected String()=%s, got %s", tc.expected, sym.String())
			}
		}
	})

	t.Run("invalid symbols", func(t *testing.T) {
		invalid := []string{
			"",
			"   ",
			"A",                     // 1 char (<2)
			strings.Repeat("B", 17), // 17 chars (>16)
			"OPEN-AI",               // hyphen
			"SOL_USD",               // underscore
			"BTC.D",                 // dot
			"ANTH ROPIC",            // space in middle
			"OPEN@AI",               // special char
		}

		for _, input := range invalid {
			_, err := ParseSymbol(input)
			if !errors.Is(err, ErrInvalidSymbol) {
				t.Errorf("expected ErrInvalidSymbol for %q, got %v", input, err)
			}
		}
	})
}
