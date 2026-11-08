package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/auth"
	"github.com/PathumSandeepa/iconfigly-api/internal/binding"
	"github.com/PathumSandeepa/iconfigly-api/internal/config"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/environment"
	"github.com/PathumSandeepa/iconfigly-api/internal/health"
	"github.com/PathumSandeepa/iconfigly-api/internal/organization"
	"github.com/PathumSandeepa/iconfigly-api/internal/project"
	"github.com/PathumSandeepa/iconfigly-api/internal/resource"
	"github.com/PathumSandeepa/iconfigly-api/internal/service"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/PathumSandeepa/iconfigly-api/internal/team"
	"github.com/PathumSandeepa/iconfigly-api/internal/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type serviceCheckerAdapter struct {
	repo service.ServiceRepository
}

func (a *serviceCheckerAdapter) GetByID(ctx context.Context, orgID, projectID, serviceID uuid.UUID) (*binding.ServiceSummary, error) {
	s, err := a.repo.GetByID(ctx, orgID, projectID, serviceID)
	if err != nil {
		return nil, err
	}
	return &binding.ServiceSummary{ID: s.ID, Slug: s.Slug.String(), Name: s.Name}, nil
}

func (a *serviceCheckerAdapter) ListByProject(ctx context.Context, orgID, projectID uuid.UUID) ([]*binding.ServiceSummary, error) {
	list, err := a.repo.ListByProject(ctx, orgID, projectID, nil, "")
	if err != nil {
		return nil, err
	}
	summaries := make([]*binding.ServiceSummary, len(list))
	for i, s := range list {
		summaries[i] = &binding.ServiceSummary{ID: s.ID, Slug: s.Slug.String(), Name: s.Name}
	}
	return summaries, nil
}

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

	envRepo := environment.NewPostgresRepository(txManager)
	envSvc := environment.NewService(envRepo, authorizer, auditSvc, txManager)
	envHandler := environment.NewHandler(envSvc)
	envHandler.RegisterRoutes(mux)

	projRepo := project.NewPostgresRepository(txManager)
	projSvc := project.NewService(projRepo, envRepo, authorizer, auditSvc, txManager)
	projHandler := project.NewHandler(projSvc)
	projHandler.RegisterRoutes(mux)

	resRepo := resource.NewPostgresRepository(txManager)
	resSvc := resource.NewService(resRepo, teamRepo, authorizer, auditSvc, txManager)
	resHandler := resource.NewHandler(resSvc)
	resHandler.RegisterRoutes(mux)

	svcRepo := service.NewPostgresRepository(txManager)
	bindingRepo := binding.NewPostgresRepository(txManager)
	bindingSvc := binding.NewService(bindingRepo, &serviceCheckerAdapter{repo: svcRepo}, resRepo, authorizer, auditSvc, txManager)
	bindingHandler := binding.NewHandler(bindingSvc)
	bindingHandler.RegisterRoutes(mux)

	svcSvc := service.NewService(svcRepo, teamRepo, envRepo, bindingRepo, auditRepo, authorizer, auditSvc, txManager)
	svcHandler := service.NewHandler(svcSvc)
	svcHandler.RegisterRoutes(mux)

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
