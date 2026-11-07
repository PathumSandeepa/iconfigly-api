package httputil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
)

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) ProblemDetails {
	t.Helper()

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("expected a JSON problem body, got %q: %v", rec.Body.String(), err)
	}

	return problem
}

func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &buf
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteJSON(rec, http.StatusCreated, Envelope[map[string]string]{Data: map[string]string{"id": "1"}})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content type %q", got)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"data":{"id":"1"}}` {
		t.Fatalf("unexpected body %s", got)
	}
}

func TestWriteJSONReturns500WhenPayloadCannotBeEncoded(t *testing.T) {
	captureDefaultLogger(t)
	rec := httptest.NewRecorder()

	WriteJSON(rec, http.StatusOK, make(chan int))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestStatusFor(t *testing.T) {
	tests := []struct {
		code apperror.Code
		want int
	}{
		{apperror.CodeNotFound, http.StatusNotFound},
		{apperror.CodeConflict, http.StatusConflict},
		{apperror.CodeValidationFailed, http.StatusUnprocessableEntity},
		{apperror.CodeUnauthorized, http.StatusUnauthorized},
		{apperror.CodeForbidden, http.StatusForbidden},
		{apperror.CodeSeparationOfDuties, http.StatusForbidden},
		{apperror.CodeRateLimited, http.StatusTooManyRequests},
		{apperror.CodeUnavailable, http.StatusServiceUnavailable},
		{apperror.CodeInternal, http.StatusInternalServerError},
		{apperror.Code("UNKNOWN"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := StatusFor(tt.code); got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestWriteProblemFormatsAppError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/42", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestIDKey{}, "req-123"))
	rec := httptest.NewRecorder()

	WriteProblem(rec, req, apperror.NotFound("project"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("unexpected content type %q", got)
	}

	problem := decodeProblem(t, rec)
	if problem.Type != "urn:iconfigly:problem:not-found" {
		t.Fatalf("unexpected type %q", problem.Type)
	}
	if problem.Status != http.StatusNotFound || problem.Detail != "project was not found" {
		t.Fatalf("unexpected problem %+v", problem)
	}
	if problem.Instance != "/api/v1/projects/42" || problem.RequestID != "req-123" {
		t.Fatalf("unexpected instance or request id: %+v", problem)
	}
}

func TestWriteProblemIncludesFieldErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/teams", nil)
	rec := httptest.NewRecorder()

	WriteProblem(rec, req, apperror.Validation("invalid slug", map[string]string{"slug": "too short"}))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	if problem := decodeProblem(t, rec); problem.Errors["slug"] != "too short" {
		t.Fatalf("expected field errors, got %+v", problem)
	}
}

func TestWriteProblemUnwrapsWrappedAppError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	WriteProblem(rec, req, fmt.Errorf("load team: %w", apperror.Forbidden("denied")))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestWriteProblemHidesUnknownErrorDetails(t *testing.T) {
	logs := captureDefaultLogger(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	WriteProblem(rec, req, errors.New("pq: password authentication failed"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "password authentication") {
		t.Fatalf("internal cause leaked to the client: %s", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "password authentication failed") {
		t.Fatalf("expected the cause to be logged, got %s", logs.String())
	}
}

func TestWriteProblemHidesInternalAppErrorCause(t *testing.T) {
	logs := captureDefaultLogger(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	WriteProblem(rec, req, apperror.Unavailable("database is unreachable", errors.New("dial tcp refused")))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "dial tcp") {
		t.Fatalf("cause leaked to the client: %s", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "dial tcp refused") {
		t.Fatalf("expected the cause to be logged, got %s", logs.String())
	}
}

func TestChainAppliesMiddlewaresInDeclaredOrder(t *testing.T) {
	var order []string
	tag := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	Chain(handler, tag("first"), tag("second")).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if got := strings.Join(order, ","); got != "first,second,handler" {
		t.Fatalf("unexpected order %s", got)
	}
}

func TestRequestIDMiddlewareGeneratesID(t *testing.T) {
	var seen string
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if _, err := uuid.Parse(seen); err != nil {
		t.Fatalf("expected a generated UUID in context, got %q", seen)
	}
	if rec.Header().Get("X-Request-ID") != seen {
		t.Fatal("expected the response header to match the context value")
	}
}

func TestRequestIDMiddlewareKeepsValidIncomingID(t *testing.T) {
	var seen string
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-id_42")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if seen != "client-id_42" || rec.Header().Get("X-Request-ID") != "client-id_42" {
		t.Fatalf("expected the incoming id to be kept, got %q", seen)
	}
}

func TestRequestIDMiddlewareReplacesUnsafeIncomingID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"spaces", "bad id"},
		{"newline", "bad\nid"},
		{"too long", strings.Repeat("a", 65)},
		{"symbols", "id;drop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = RequestIDFromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header["X-Request-Id"] = []string{tt.id}

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if seen == tt.id {
				t.Fatalf("expected %q to be replaced", tt.id)
			}
			if _, err := uuid.Parse(seen); err != nil {
				t.Fatalf("expected a generated UUID, got %q", seen)
			}
		})
	}
}

func TestLoggerMiddlewareLogsRequestFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}),
		RequestIDMiddleware,
		LoggerMiddleware(logger),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/things", nil)
	req.RemoteAddr = "10.1.2.3:5555"
	req.Header.Set("X-Request-ID", "req-1")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected a JSON log line, got %q: %v", buf.String(), err)
	}
	if entry["method"] != "POST" || entry["path"] != "/api/v1/things" || entry["request_id"] != "req-1" || entry["ip"] != "10.1.2.3" {
		t.Fatalf("unexpected log entry %v", entry)
	}
	if entry["status"] != float64(http.StatusTeapot) {
		t.Fatalf("expected status 418, got %v", entry["status"])
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Fatal("expected duration_ms in log entry")
	}
}

func TestLoggerMiddlewareDefaultsStatusTo200(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := LoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected a JSON log line: %v", err)
	}
	if entry["status"] != float64(http.StatusOK) {
		t.Fatalf("expected status 200, got %v", entry["status"])
	}
}

func TestRecoverMiddlewareReturnsProblemOnPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("kaboom")
		}),
		RequestIDMiddleware,
		LoggerMiddleware(logger),
		RecoverMiddleware(logger),
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	problem := decodeProblem(t, rec)
	if problem.Type != "urn:iconfigly:problem:internal" || problem.RequestID == "" {
		t.Fatalf("unexpected problem %+v", problem)
	}
	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatal("panic value leaked to the client")
	}
	if !strings.Contains(buf.String(), "kaboom") || !strings.Contains(buf.String(), "stack") {
		t.Fatalf("expected the panic and stack to be logged, got %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"status":500`) {
		t.Fatalf("expected the logger to record status 500, got %s", buf.String())
	}
}

func TestRecoverMiddlewareRepanicsOnAbortHandler(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	handler := RecoverMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Fatalf("expected ErrAbortHandler to propagate, got %v", recovered)
		}
	}()

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestEncodeDecodeCursorRoundTrip(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	createdAt := time.Date(2026, 11, 7, 12, 30, 45, 123456000, time.FixedZone("IST", 5*3600+1800))

	gotID, gotCreatedAt, err := DecodeCursor(EncodeCursor(id, createdAt))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gotID != id {
		t.Fatalf("expected id %s, got %s", id, gotID)
	}
	if !gotCreatedAt.Equal(createdAt) {
		t.Fatalf("expected time %s, got %s", createdAt, gotCreatedAt)
	}
}

func TestDecodeCursorRejectsInvalidInput(t *testing.T) {
	notJSON := "bm90LWpzb24"
	nilID := EncodeCursor(uuid.Nil, time.Now())

	for name, cursor := range map[string]string{
		"not base64":  "***",
		"not json":    notJSON,
		"nil uuid":    nilID,
		"empty input": "",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := DecodeCursor(cursor)

			if !apperror.HasCode(err, apperror.CodeValidationFailed) {
				t.Fatalf("expected a validation error, got %v", err)
			}
		})
	}
}

func TestParsePageRequest(t *testing.T) {
	validCursor := EncodeCursor(uuid.Must(uuid.NewV7()), time.Now())

	tests := []struct {
		name    string
		query   string
		want    PageRequest
		wantErr bool
	}{
		{"defaults", "", PageRequest{Limit: 25}, false},
		{"custom limit", "limit=50", PageRequest{Limit: 50}, false},
		{"minimum limit", "limit=1", PageRequest{Limit: 1}, false},
		{"maximum limit", "limit=100", PageRequest{Limit: 100}, false},
		{"with cursor", "limit=10&cursor=" + validCursor, PageRequest{Limit: 10, Cursor: validCursor}, false},
		{"zero limit", "limit=0", PageRequest{}, true},
		{"negative limit", "limit=-3", PageRequest{}, true},
		{"too large limit", "limit=101", PageRequest{}, true},
		{"non numeric limit", "limit=abc", PageRequest{}, true},
		{"invalid cursor", "cursor=garbage***", PageRequest{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/items?"+tt.query, nil)

			got, err := ParsePageRequest(req)

			if tt.wantErr {
				if !apperror.HasCode(err, apperror.CodeValidationFailed) {
					t.Fatalf("expected a validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %+v, got %+v", tt.want, got)
			}
		})
	}
}

func TestNewPageResponseTrimsExtraItemAndSetsCursor(t *testing.T) {
	items := []int{1, 2, 3}

	page := NewPageResponse(items, 2, func(n int) string { return fmt.Sprintf("cursor-%d", n) })

	if !page.HasMore || len(page.Items) != 2 {
		t.Fatalf("expected two items and has_more, got %+v", page)
	}
	if page.NextCursor == nil || *page.NextCursor != "cursor-2" {
		t.Fatalf("expected cursor of the last returned item, got %v", page.NextCursor)
	}
}

func TestNewPageResponseWithoutMoreItems(t *testing.T) {
	page := NewPageResponse([]int{1, 2}, 2, func(n int) string { return "unused" })

	if page.HasMore || page.NextCursor != nil || len(page.Items) != 2 {
		t.Fatalf("expected a final page, got %+v", page)
	}
}

func TestNewPageResponseEncodesEmptyItemsAsArray(t *testing.T) {
	page := NewPageResponse[int](nil, 10, func(n int) string { return "" })

	body, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(body) != `{"items":[],"next_cursor":null,"has_more":false}` {
		t.Fatalf("unexpected body %s", body)
	}
}
