package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/justtrackio/gosoline/pkg/appctx"
	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/funk"
	"github.com/justtrackio/gosoline/pkg/log"
	apiv1 "k8s.io/api/core/v1"
)

type servicePoolManagerKey struct{}

func ProvideServicePoolManager(ctx context.Context, config cfg.Config, logger log.Logger) (*ServicePoolManager, error) {
	return appctx.Provide(ctx, servicePoolManagerKey{}, func() (*ServicePoolManager, error) {
		var err error
		var k8sClient *K8sClient

		if k8sClient, err = NewK8sClient(config, logger); err != nil {
			return nil, fmt.Errorf("could not create k8s client: %w", err)
		}

		poolFactory := func(id string) (*ServicePool, error) {
			return NewServicePool(config, logger, k8sClient, id)
		}

		return &ServicePoolManager{
			logger:      logger.WithChannel("pool-manager"),
			k8sClient:   k8sClient,
			poolFactory: poolFactory,
			pools:       map[string]*ServicePool{},
		}, nil
	})
}

type ServicePoolManager struct {
	lck         sync.RWMutex
	logger      log.Logger
	k8sClient   *K8sClient
	poolFactory func(id string) (*ServicePool, error)
	pools       map[string]*ServicePool
}

func (c *ServicePoolManager) WarmUpPool(ctx context.Context, input *WarmUpInput) error {
	var err error
	var pool *ServicePool

	if pool, err = c.getPool(ctx, input.PoolId); err != nil {
		return fmt.Errorf("could not get pool: %w", err)
	}

	return pool.WarmUp(ctx, input)
}

func (c *ServicePoolManager) ShutdownPool(ctx context.Context, input *ShutdownInput) error {
	var err error
	var pool *ServicePool

	if pool, err = c.getPool(ctx, input.PoolId); err != nil {
		return fmt.Errorf("could not get pool: %w", err)
	}

	return pool.Shutdown(ctx)
}

func (c *ServicePoolManager) FetchService(ctx context.Context, input *RunInput) (*apiv1.Service, error) {
	var err error
	var pool *ServicePool
	var service *apiv1.Service

	if pool, err = c.getPool(ctx, input.PoolId); err != nil {
		return nil, fmt.Errorf("could not get pool: %w", err)
	}

	if service, err = pool.ClaimService(ctx, input); err != nil {
		return nil, fmt.Errorf("could not claim service: %w", err)
	}

	return service, nil
}

func (c *ServicePoolManager) ExtendServices(ctx context.Context, input *ExtendInput) error {
	var err error
	var pool *ServicePool

	if pool, err = c.getPool(ctx, input.PoolId); err != nil {
		return fmt.Errorf("could not get pool: %w", err)
	}

	return pool.ExtendServices(ctx, input)
}

func (c *ServicePoolManager) ReleaseServices(ctx context.Context, input *StopInput) error {
	var err error
	var pool *ServicePool

	if pool, err = c.getPool(ctx, input.PoolId); err != nil {
		return fmt.Errorf("could not get pool: %w", err)
	}

	return pool.ReleaseServices(ctx, input.GetLabels())
}

func (c *ServicePoolManager) ExpireServices(ctx context.Context) error {
	var err error
	var services []*apiv1.Service

	if err = expireObjects(ctx, c.logger, c.k8sClient.ListDeployments, c.k8sClient.DeleteDeployment, "deployment"); err != nil {
		return fmt.Errorf("could not expire deployments: %w", err)
	}

	if err = expireObjects(ctx, c.logger, c.k8sClient.ListServices, c.k8sClient.DeleteService, "service"); err != nil {
		return fmt.Errorf("could not expire services: %w", err)
	}

	c.lck.Lock()
	defer c.lck.Unlock()

	poolIds := funk.Keys(c.pools)
	for _, poolId := range poolIds {
		if services, err = c.k8sClient.ListServices(ctx, map[string]string{LabelPoolId: poolId}); err != nil {
			return fmt.Errorf("failed to list services: %w", err)
		}

		if len(services) != 0 {
			continue
		}

		delete(c.pools, poolId)
	}

	return nil
}

func (c *ServicePoolManager) getPool(ctx context.Context, poolId string) (*ServicePool, error) {
	c.lck.Lock()
	defer c.lck.Unlock()

	var ok bool
	var pool *ServicePool

	if pool, ok = c.pools[poolId]; ok {
		return pool, nil
	}

	return c.addPool(ctx, poolId)
}

func (c *ServicePoolManager) addPool(ctx context.Context, poolId string) (*ServicePool, error) {
	var err error

	if c.pools[poolId], err = c.poolFactory(poolId); err != nil {
		return nil, fmt.Errorf("could not create pool %q: %w", poolId, err)
	}

	c.logger.Info(ctx, "created new pool %q", poolId)

	return c.pools[poolId], nil
}

func expireObjects[T Objecter](
	ctx context.Context,
	logger log.Logger,
	lister func(ctx context.Context, selectors ...map[string]string) ([]T, error),
	deleter func(ctx context.Context, object Objecter) error,
	objectType string,
) error {
	var err error
	var objects []T
	var expireAfter time.Time

	if objects, err = lister(ctx, map[string]string{}); err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	for _, o := range objects {
		annotations := o.GetAnnotations()

		if _, ok := annotations[AnnotationExpireAfter]; !ok {
			continue
		}

		if expireAfter, err = time.Parse(time.RFC3339, annotations[AnnotationExpireAfter]); err != nil {
			return fmt.Errorf("could not parse annotation expire after: %w", err)
		}

		if expireAfter.After(time.Now()) {
			continue
		}

		if err = deleter(ctx, o); err != nil {
			return fmt.Errorf("could not delete service: %w", err)
		}

		logger.Info(ctx, "expired %q %q in pool %q", objectType, o.GetName(), o.GetLabels()[LabelPoolId])
	}

	return nil
}
