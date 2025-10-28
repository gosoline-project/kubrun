package main

import (
	"context"
	"fmt"
	"time"

	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/clock"
	"github.com/justtrackio/gosoline/pkg/coffin"
	"github.com/justtrackio/gosoline/pkg/kernel"
	"github.com/justtrackio/gosoline/pkg/log"
)

func NewPoolModule(ctx context.Context, config cfg.Config, logger log.Logger) (kernel.Module, error) {
	var err error
	var k8sClient *K8sClient
	var poolManager *ServicePoolManager

	if k8sClient, err = ProvideK8sClient(config, logger); err != nil {
		return nil, fmt.Errorf("could not create k8s client: %w", err)
	}

	if poolManager, err = ProvideServicePoolManager(ctx, config, logger); err != nil {
		return nil, fmt.Errorf("could not create service pool manager: %w", err)
	}

	return &PoolModule{
		logger:      logger.WithChannel("pool-module"),
		k8sClient:   k8sClient,
		poolManager: poolManager,
		ticker:      clock.NewRealTicker(time.Minute),
	}, nil
}

type PoolModule struct {
	logger      log.Logger
	k8sClient   *K8sClient
	poolManager *ServicePoolManager
	ticker      clock.Ticker
}

func (p PoolModule) Run(ctx context.Context) error {
	if err := p.poolManager.ExpireServices(ctx); err != nil {
		p.logger.Error(ctx, "could not expire services: %w", err)
	}

	cfn := coffin.New()
	cfn.GoWithContext(ctx, p.doExpireServices)
	cfn.GoWithContext(ctx, p.doWatchPools)

	return cfn.Wait()
}

func (p PoolModule) doExpireServices(ctx context.Context) error {
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
}

func (p PoolModule) doWatchPools(ctx context.Context) error {
	if err := p.poolManager.WatchPools(ctx); err != nil {
		return fmt.Errorf("could not watch pools: %w", err)
	}

	return nil
}
