package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/auth"
	"github.com/PathumSandeepa/iconfigly-api/internal/config"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/health"
	"github.com/PathumSandeepa/iconfigly-api/internal/organization"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/PathumSandeepa/iconfigly-api/internal/team"
	"github.com/PathumSandeepa/iconfigly-api/internal/user"
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

	orgRepo := organization.NewPostgresRepository(txManager)
	orgSvc := organization.NewService(orgRepo, authorizer, auditSvc, txManager)
	orgHandler := organization.NewHandler(orgSvc)
	orgHandler.RegisterRoutes(mux)

	userRepo := user.NewPostgresRepository(txManager)
	sessionRepo := auth.NewPostgresSessionRepository(txManager)
	tokenRepo := auth.NewPostgresTokenRepository(txManager)
	membershipResolver := auth.NewPostgresMembershipResolver(txManager)

	authSvc := auth.NewService(
		orgRepo,
		userRepo,
		accessRepo,
		sessionRepo,
		tokenRepo,
		membershipResolver,
		auditSvc,
		txManager,
	)
	authHandler := auth.NewHandler(authSvc, userRepo)
	authHandler.RegisterRoutes(mux)

	userSvc := user.NewService(userRepo, authorizer, auditSvc, accessSvc, authSvc, txManager)
	userHandler := user.NewHandler(userSvc)
	userHandler.RegisterRoutes(mux)

	teamRepo := team.NewPostgresRepository(txManager)
	teamSvc := team.NewService(teamRepo, userRepo, authorizer, auditSvc, txManager)
	teamHandler := team.NewHandler(teamSvc)
	teamHandler.RegisterRoutes(mux)

	handler := httputil.Chain(
		mux,
		httputil.RecoverMiddleware(logger),
		httputil.RequestIDMiddleware,
		httputil.LoggerMiddleware(logger),
		auth.AuthenticateMiddleware(authSvc),
		auth.CSRFMiddleware(),
	)

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
