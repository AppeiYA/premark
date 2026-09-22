package domain

import (
	"fmt"
	"regexp"
	"strings"
)

type Symbol string

var symbolRegex = regexp.MustCompile(`^[A-Z0-9]{2,16}$`)

// ParseSymbol trims spaces, upper-cases, and requires ^[A-Z0-9]{2,16}$.
// Failure returns ErrInvalidSymbol (wrapped).
func ParseSymbol(s string) (Symbol, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(s))
	if !symbolRegex.MatchString(trimmed) {
		return "", fmt.Errorf("%w: symbol %q invalid", ErrInvalidSymbol, s)
	}
	return Symbol(trimmed), nil
}

func (s Symbol) String() string {
	return string(s)
}
