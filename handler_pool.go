package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gosoline-project/httpserver"
	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/log"
)

type WarmUpInput struct {
	PoolId     string         `json:"pool_id"`
	Components map[string]int `json:"components"`
}

type ShutdownInput struct {
	PoolId string `json:"pool_id"`
}

type HandlerPool struct {
	poolManager *ServicePoolManager
}

func NewHandlerPool(ctx context.Context, config cfg.Config, logger log.Logger) (*HandlerPool, error) {
	var err error
	var poolManager *ServicePoolManager

	if poolManager, err = ProvideServicePoolManager(ctx, config, logger); err != nil {
		return nil, fmt.Errorf("could not create service pool manager: %w", err)
	}

	return &HandlerPool{
		poolManager: poolManager,
	}, nil
}

func (h *HandlerPool) HandleWarmUp(ctx context.Context, input *WarmUpInput) (httpserver.Response, error) {
	if err := h.poolManager.WarmUpPool(ctx, input); err != nil {
		return nil, fmt.Errorf("could not warm up pool: %w", err)
	}

	return httpserver.NewStatusResponse(http.StatusOK), nil
}

func (h *HandlerPool) HandleShutdown(ctx context.Context, input *ShutdownInput) (httpserver.Response, error) {
	if err := h.poolManager.ShutdownPool(ctx, input); err != nil {
		return nil, fmt.Errorf("could not warm up pool: %w", err)
	}

	return httpserver.NewStatusResponse(http.StatusOK), nil
}
