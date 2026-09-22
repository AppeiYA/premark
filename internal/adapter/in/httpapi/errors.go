package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"premark/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

func writeAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRule),
		errors.Is(err, domain.ErrInvalidSymbol),
		errors.Is(err, domain.ErrInvalidArgument),
		errors.Is(err, domain.ErrInvalidPrice):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, domain.ErrTokenNotFound),
		errors.Is(err, domain.ErrRuleNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrScanInProgress):
		writeError(w, http.StatusConflict, "scan_in_progress", err.Error())
	case errors.Is(err, domain.ErrUpstream),
		errors.Is(err, domain.ErrQuoteUnavailable):
		writeError(w, http.StatusBadGateway, "upstream_unavailable", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}
