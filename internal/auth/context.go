package auth

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
)

func WithActor(ctx context.Context, actor Actor) context.Context {
	return access.WithActor(ctx, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	return access.ActorFromContext(ctx)
}

func RequireActor(ctx context.Context) (Actor, error) {
	actor, ok := access.ActorFromContext(ctx)
	if !ok {
		return Actor{}, apperror.Unauthorized("authentication required")
	}
	return actor, nil
}
