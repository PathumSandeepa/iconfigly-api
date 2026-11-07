package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(ctx context.Context) error {
	return f.err
}

func TestHandlerReportsLiveness(t *testing.T) {
	rec := httptest.NewRecorder()

	Handler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content type %q", got)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body: %v", err)
	}
	if body["status"] != "ok" || body["service"] != "iconfigly-api" {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestReadyHandlerReportsReadyWhenDatabaseResponds(t *testing.T) {
	rec := httptest.NewRecorder()

	ReadyHandler(fakePinger{})(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body: %v", err)
	}
	if body["status"] != "ready" || body["database"] != "connected" {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestReadyHandlerReturnsProblemWhenDatabaseFails(t *testing.T) {
	rec := httptest.NewRecorder()

	ReadyHandler(fakePinger{err: errors.New("connection refused")})(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("unexpected content type %q", got)
	}

	var problem map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("expected JSON body: %v", err)
	}
	if problem["status"] != float64(http.StatusServiceUnavailable) {
		t.Fatalf("unexpected problem %v", problem)
	}
	if problem["detail"] == "connection refused" {
		t.Fatal("driver error must not leak to the client")
	}
}
