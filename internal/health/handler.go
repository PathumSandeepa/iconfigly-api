package health

import (
	"context"
	"net/http"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
)

const readinessTimeout = 2 * time.Second

type Pinger interface {
	Ping(ctx context.Context) error
}

func Handler(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "iconfigly-api",
	})
}

func ReadyHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			httputil.WriteProblem(w, r, apperror.Unavailable("database is not reachable", err))
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]string{
			"status":   "ready",
			"database": "connected",
		})
	}
}
