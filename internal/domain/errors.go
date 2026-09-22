package domain

import "errors"

var (
	ErrInvalidRule      = errors.New("invalid rule")
	ErrInvalidSymbol    = errors.New("invalid symbol")
	ErrInvalidPrice     = errors.New("invalid price")
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrTokenNotFound    = errors.New("token not found")
	ErrRuleNotFound     = errors.New("rule not found")
	ErrUpstream         = errors.New("upstream unavailable")
	ErrQuoteUnavailable = errors.New("quote unavailable")
	ErrScanInProgress   = errors.New("scan already in progress")
)
