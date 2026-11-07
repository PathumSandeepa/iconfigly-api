package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
)

const (
	jsonContentType    = "application/json"
	problemContentType = "application/problem+json"
	problemTypePrefix  = "urn:iconfigly:problem:"
)

type ProblemDetails struct {
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Status    int               `json:"status"`
	Detail    string            `json:"detail,omitempty"`
	Instance  string            `json:"instance,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
	Errors    map[string]string `json:"errors,omitempty"`
}

type Envelope[T any] struct {
	Data T `json:"data"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	writeBody(w, status, jsonContentType, payload)
}

func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := apperror.From(err)
	if !ok {
		appErr = apperror.Internal(err)
	}

	status := StatusFor(appErr.Code)

	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed",
			"code", appErr.Code,
			"path", r.URL.Path,
			"request_id", RequestIDFromContext(r.Context()),
			"error", err,
		)
	}

	writeProblem(w, r, appErr)
}

func StatusFor(code apperror.Code) int {
	switch code {
	case apperror.CodeNotFound:
		return http.StatusNotFound
	case apperror.CodeConflict:
		return http.StatusConflict
	case apperror.CodeValidationFailed:
		return http.StatusUnprocessableEntity
	case apperror.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperror.CodeForbidden, apperror.CodeSeparationOfDuties:
		return http.StatusForbidden
	case apperror.CodeRateLimited:
		return http.StatusTooManyRequests
	case apperror.CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func writeProblem(w http.ResponseWriter, r *http.Request, appErr *apperror.Error) {
	status := StatusFor(appErr.Code)

	writeBody(w, status, problemContentType, ProblemDetails{
		Type:      problemType(appErr.Code),
		Title:     appErr.Title,
		Status:    status,
		Detail:    appErr.Detail,
		Instance:  r.URL.Path,
		RequestID: RequestIDFromContext(r.Context()),
		Errors:    appErr.Fields,
	})
}

func problemType(code apperror.Code) string {
	return problemTypePrefix + strings.ReplaceAll(strings.ToLower(string(code)), "_", "-")
}

func writeBody(w http.ResponseWriter, status int, contentType string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("encode response body", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
