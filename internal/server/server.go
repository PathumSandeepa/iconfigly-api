package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/config"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/health"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(cfg config.Config, db *pgxpool.Pool, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", health.Handler)
	mux.HandleFunc("GET /ready", health.ReadyHandler(db))

	txManager := database.NewTxManager(db)
	auditRepo := audit.NewPostgresRepository(txManager)
	auditSvc := audit.NewService(auditRepo, txManager)
	auditHandler := audit.NewHandler(auditSvc)
	auditHandler.RegisterRoutes(mux)

	accessRepo := access.NewPostgresRepository(txManager)
	authorizer := access.NewAuthorizer(accessRepo)
	accessSvc := access.NewService(accessRepo, authorizer, auditSvc, txManager)
	accessHandler := access.NewHandler(accessSvc)
	accessHandler.RegisterRoutes(mux)

	handler := httputil.Chain(
		mux,
		httputil.RequestIDMiddleware,
		httputil.LoggerMiddleware(logger),
		httputil.RecoverMiddleware(logger),
	)

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
