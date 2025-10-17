package main

import (
	"context"
	"fmt"
	"time"

	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/clock"
	"github.com/justtrackio/gosoline/pkg/kernel"
	"github.com/justtrackio/gosoline/pkg/log"
)

func NewPoolModule(ctx context.Context, config cfg.Config, logger log.Logger) (kernel.Module, error) {
	var err error
	var poolManager *ServicePoolManager

	if poolManager, err = ProvideServicePoolManager(ctx, config, logger); err != nil {
		return nil, fmt.Errorf("could not create service pool manager: %w", err)
	}

	return &PoolModule{
		logger:      logger.WithChannel("pool-module"),
		poolManager: poolManager,
		ticker:      clock.NewRealTicker(time.Minute),
	}, nil
}

type PoolModule struct {
	logger      log.Logger
	poolManager *ServicePoolManager
	ticker      clock.Ticker
}

func (p PoolModule) Run(ctx context.Context) error {
	if err := p.poolManager.ExpireServices(ctx); err != nil {
		p.logger.Error(ctx, "could not expire services: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.ticker.Chan():
			if err := p.poolManager.ExpireServices(ctx); err != nil {
				p.logger.Error(ctx, "could not expire services: %w", err)
			}
		}
	}

	return nil
}
