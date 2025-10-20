package main

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/justtrackio/gosoline/pkg/clock"
	"github.com/justtrackio/gosoline/pkg/funk"
	"github.com/justtrackio/gosoline/pkg/log"
	"github.com/justtrackio/gosoline/pkg/uuid"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

var specs = map[string]ContainerSpec{
	"ddb": {
		Repository: "amazon/dynamodb-local",
		Tag:        "2.5.4",
		PortBindings: map[string]PortBinding{
			"main": {
				ContainerPort: 8000,
				Protocol:      "tcp",
			},
		},
	},
	"localstack": {
		Repository: "localstack/localstack",
		Tag:        "4.1.0",
		PortBindings: map[string]PortBinding{
			"main": {
				ContainerPort: 4566,
				Protocol:      "tcp",
			},
		},
	},
	"mysql": {
		Repository: "mysql/mysql-server",
		Tag:        "8.0",
		Env: map[string]string{
			"MYSQL_DATABASE":      "gosoline",
			"MYSQL_USER":          "gosoline",
			"MYSQL_PASSWORD":      "gosoline",
			"MYSQL_ROOT_PASSWORD": "gosoline",
			"MYSQL_ROOT_HOST":     "%",
		},
		Cmd: []string{"--sql_mode=NO_ENGINE_SUBSTITUTION", "--log-bin-trust-function-creators=TRUE", "--max_connections=1000"},
		PortBindings: map[string]PortBinding{
			"main": {
				ContainerPort: 3306,
				Protocol:      "tcp",
			},
		},
	},
	"redis": {
		Repository: "redis",
		Tag:        "7-alpine",
		PortBindings: map[string]PortBinding{
			"main": {
				ContainerPort: 6379,
				Protocol:      "tcp",
			},
		},
	},
	"s3": {
		Repository: "minio/minio",
		Tag:        "RELEASE.2024-02-17T01-15-57Z",
		Cmd: []string{
			"server",
			"/data",
		},
		Env: map[string]string{
			"MINIO_ACCESS_KEY": "gosoline",
			"MINIO_SECRET_KEY": "gosoline",
		},
		PortBindings: map[string]PortBinding{
			"main": {
				ContainerPort: 6379,
				Protocol:      "tcp",
			},
		},
	},
	"wiremock": {
		Repository: "wiremock/wiremock",
		Tag:        "3.4.1",
		Cmd:        []string{"--local-response-templating"},
		PortBindings: map[string]PortBinding{
			"main": {
				ContainerPort: 8080,
				Protocol:      "tcp",
			},
		},
	},
}

type ServicePool struct {
	lck       sync.RWMutex
	logger    log.Logger
	k8sClient *K8sClient
	factory   *ApplicationFactory
	id        string
	clock     clock.Clock
}

func NewServicePool(logger log.Logger, k8sClient *K8sClient, id string) *ServicePool {
	return &ServicePool{
		logger:    logger.WithChannel("pool").WithFields(log.Fields{"pool-id": id}),
		k8sClient: k8sClient,
		factory:   &ApplicationFactory{},
		id:        id,
		clock:     clock.NewRealClock(),
	}
}

func (c *ServicePool) WarmUp(ctx context.Context, input *WarmUpInput) error {
	var ok bool
	var spec ContainerSpec

	for componentType, count := range input.Components {
		if spec, ok = specs[componentType]; !ok {
			c.logger.Info(ctx, "no warm up spec found for component type %q: skipping", componentType)

			continue
		}

		warmUp := &WarmUpDeployment{
			PoolId:        input.PoolId,
			ComponentType: componentType,
			ContainerName: "main",
			Spec:          spec,
		}

		for i := 0; i < count; i++ {
			if _, err := c.spawnDeployment(ctx, warmUp); err != nil {
				return fmt.Errorf("could not spawn warm up deployment: %w", err)
			}
		}
	}

	return nil
}

func (c *ServicePool) Shutdown(ctx context.Context) error {
	return c.ReleaseServices(ctx, map[string]string{LabelPoolId: c.id})
}

func (c *ServicePool) ClaimService(ctx context.Context, input *RunInput) (*apiv1.Service, error) {
	c.lck.Lock()
	defer c.lck.Unlock()

	var err error
	var deployments []*appsv1.Deployment
	var service *apiv1.Service

	if _, err = c.spawnDeployment(ctx, input); err != nil {
		return nil, fmt.Errorf("could not spawn deployment: %w", err)
	}

	labels := map[string]string{
		LabelPoolId:        c.id,
		LabelComponentType: input.ComponentType,
		LabelContainerName: input.ContainerName,
		LableIdle:          "true",
	}

	if deployments, err = c.k8sClient.ListDeployments(ctx, labels); err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	slices.SortFunc(deployments, func(a, b *appsv1.Deployment) int {
		if a.CreationTimestamp.Before(&b.CreationTimestamp) {
			return -1
		}

		return 1
	})

	if service, err = c.claimDeployment(ctx, deployments[0], input); err != nil {
		return nil, fmt.Errorf("could not claim deployment: %w", err)
	}

	return service, nil
}

func (c *ServicePool) ExtendServices(ctx context.Context, input *ExtendInput) error {
	var err error
	var deployments []*appsv1.Deployment
	var services []*apiv1.Service

	expireAfter := c.clock.Now().Add(input.Duration).Format(time.RFC3339)
	ops := []string{
		fmt.Sprintf(`{"op": "replace", "path": "/metadata/annotations/%s", "value": "%s"}`, strings.ReplaceAll(AnnotationExpireAfter, "/", "~1"), expireAfter),
	}

	if deployments, err = c.k8sClient.ListDeployments(ctx, input.GetLabels()); err != nil {
		return fmt.Errorf("could not list deployments: %w", err)
	}

	for _, deployment := range deployments {
		if deployment, err = c.k8sClient.PatchDeployment(ctx, deployment, ops); err != nil {
			return fmt.Errorf("could not patch deployment: %w", err)
		}
	}

	if services, err = c.k8sClient.ListServices(ctx, input.GetLabels()); err != nil {
		return fmt.Errorf("could not list services: %w", err)
	}

	for _, service := range services {
		if service, err = c.k8sClient.PatchService(ctx, service, ops); err != nil {
			return fmt.Errorf("could not patch service: %w", err)
		}
	}

	return nil
}

func (c *ServicePool) ReleaseServices(ctx context.Context, labels map[string]string) error {
	var err error
	var deployments []*appsv1.Deployment
	var services []*apiv1.Service

	if deployments, err = c.k8sClient.ListDeployments(ctx, labels); err != nil {
		return fmt.Errorf("could not list deployments: %w", err)
	}

	for _, d := range deployments {
		if err = c.k8sClient.DeleteDeployment(ctx, d); err != nil {
			return fmt.Errorf("could not delete deployment: %w", err)
		}
	}

	if services, err = c.k8sClient.ListServices(ctx, labels); err != nil {
		return fmt.Errorf("could not list services: %w", err)
	}

	for _, s := range services {
		if err = c.k8sClient.DeleteService(ctx, s); err != nil {
			return fmt.Errorf("could not delete service: %w", err)
		}

	}

	keys := funk.Keys(labels)
	sort.Strings(keys)
	ids := make([]string, 0)

	for _, k := range keys {
		ids = append(ids, fmt.Sprintf("%s=%s", k, labels[k]))
	}

	c.logger.Info(ctx, "released test resources %q", strings.Join(ids, ", "))

	return nil
}

func (c *ServicePool) spawnDeployment(ctx context.Context, input SpawnAble) (*appsv1.Deployment, error) {
	var err error
	uid := uuid.New().NewV4()[0:8]

	deployment := c.factory.CreateDeployment(uid, input)
	if deployment, err = c.k8sClient.CreateDeployment(ctx, deployment); err != nil {
		return nil, fmt.Errorf("could not create deployment: %w", err)
	}

	service := c.factory.CreateService(uid, input)
	if service, err = c.k8sClient.CreateService(ctx, service); err != nil {
		return nil, fmt.Errorf("could not create service: %w", err)
	}

	c.logger.Info(ctx, "spawned deployment %q", deployment.Name)

	return deployment, nil
}

func (c *ServicePool) claimDeployment(ctx context.Context, deployment *appsv1.Deployment, input *RunInput) (*apiv1.Service, error) {
	var err error
	var service *apiv1.Service

	expireAfter := c.clock.Now().Add(input.ExpireAfter).Format(time.RFC3339)
	ops := []string{
		fmt.Sprintf(`{"op": "remove", "path": "/metadata/labels/%s"}`, strings.ReplaceAll(LableIdle, "/", "~1")),
		fmt.Sprintf(`{"op": "add", "path": "/metadata/labels/%s", "value": "%s"}`, strings.ReplaceAll(LabelTestId, "/", "~1"), input.TestId),
		fmt.Sprintf(`{"op": "add", "path": "/metadata/labels/%s", "value": "%s"}`, strings.ReplaceAll(LabelComponentName, "/", "~1"), input.ComponentName),
		fmt.Sprintf(`{"op": "add", "path": "/metadata/annotations/%s", "value": "%s"}`, strings.ReplaceAll(AnnotationExpireAfter, "/", "~1"), expireAfter),
	}

	if deployment, err = c.k8sClient.PatchDeployment(ctx, deployment, ops); err != nil {
		return nil, fmt.Errorf("could not patch deployment: %w", err)
	}

	if service, err = c.k8sClient.GetService(ctx, deployment.GetName()); err != nil {
		return nil, fmt.Errorf("could not get service: %w", err)
	}

	if service, err = c.k8sClient.PatchService(ctx, service, ops); err != nil {
		return nil, fmt.Errorf("could not patch service: %w", err)
	}

	c.logger.Info(ctx, "claimed deployment %q", deployment.Name)

	return service, nil
}
