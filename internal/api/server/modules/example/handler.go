package example

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	contract "github.com/supernurture/go-template/internal/api/server/oapicodegen/example"
)

const visitsKey = "example:visits"

// Handler shows the shape of a module that needs a dependency: take it in the constructor, keep it unexported.
type Handler struct {
	redis *goredis.Client
}

func NewHandler(client *goredis.Client) *Handler {
	return &Handler{redis: client}
}

var _ contract.StrictServerInterface = (*Handler)(nil)

func (h *Handler) GetExampleVisits(ctx context.Context, _ contract.GetExampleVisitsRequestObject) (contract.GetExampleVisitsResponseObject, error) {
	// ctx carries the request deadline set by the timeout middleware, so a stalled Redis cannot outlive the request.
	visits, err := h.redis.Incr(ctx, visitsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("increment %q: %w", visitsKey, err)
	}

	return contract.GetExampleVisits200JSONResponse{Visits: visits}, nil
}
