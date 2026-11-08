package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PathumSandeepa/iconfigly-api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestServer(t *testing.T) (*http.Server, *bytes.Buffer) {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), "postgres://user:pass@127.0.0.1:1/db?sslmode=disable")
	if err != nil {
		t.Fatalf("create lazy pool: %v", err)
	}
	t.Cleanup(pool.Close)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	return New(config.Config{Port: "8080"}, pool, logger), &logs
}

func TestNewListensOnConfiguredPort(t *testing.T) {
	srv, _ := newTestServer(t)

	if srv.Addr != ":8080" {
		t.Fatalf("expected :8080, got %s", srv.Addr)
	}
	if srv.ReadHeaderTimeout == 0 {
		t.Fatal("expected a read header timeout to protect against slow clients")
	}
}

func TestHealthRouteIsRegisteredWithRequestID(t *testing.T) {
	srv, logs := newTestServer(t)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected the request id middleware to set X-Request-ID")
	}
	if !strings.Contains(logs.String(), `"path":"/health"`) {
		t.Fatalf("expected the logger middleware to log the request, got %s", logs.String())
	}
}

func TestReadyRouteReturns503WhenDatabaseIsDown(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("unexpected content type %q", got)
	}
}

func TestAuditRoutesAreRegistered(t *testing.T) {
	srv, _ := newTestServer(t)

	recList := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recList, httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil))
	if recList.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing organization_id on list route, got %d", recList.Code)
	}

	recVerify := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recVerify, httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs/verify", nil))
	if recVerify.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing organization_id on verify route, got %d", recVerify.Code)
	}
}

func TestAccessRoutesAreRegistered(t *testing.T) {
	srv, _ := newTestServer(t)

	recBindings := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recBindings, httptest.NewRequest(http.MethodGet, "/api/v1/projects/0192e226-9f44-7f1b-a951-8728d48bb789/access/bindings", nil))
	if recBindings.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request on access bindings, got %d", recBindings.Code)
	}

	recInspect := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recInspect, httptest.NewRequest(http.MethodGet, "/api/v1/projects/0192e226-9f44-7f1b-a951-8728d48bb789/access/inspect", nil))
	if recInspect.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request on access inspect, got %d", recInspect.Code)
	}
}

func TestAuthAndUserRoutesAreRegistered(t *testing.T) {
	srv, _ := newTestServer(t)

	recMe := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recMe, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if recMe.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request on /api/v1/auth/me, got %d", recMe.Code)
	}

	recUsers := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recUsers, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))
	if recUsers.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request on /api/v1/users, got %d", recUsers.Code)
	}

	recTokens := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recTokens, httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil))
	if recTokens.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request on /api/v1/auth/tokens, got %d", recTokens.Code)
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
