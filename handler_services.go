package main

import (
	"context"
	"fmt"
	"net"

	"github.com/gosoline-project/httpserver"
	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/log"
	apiv1 "k8s.io/api/core/v1"
)

type HandlerServices struct {
	poolManager *ServicePoolManager
}

func NewHandlerServices(ctx context.Context, config cfg.Config, logger log.Logger) (*HandlerServices, error) {
	var err error
	var poolManager *ServicePoolManager

	if poolManager, err = ProvideServicePoolManager(ctx, config, logger); err != nil {
		return nil, fmt.Errorf("could not create service pool manager: %w", err)
	}

	return &HandlerServices{
		poolManager: poolManager,
	}, nil
}

func (h *HandlerServices) HandleRun(ctx context.Context, input *RunInput) (httpserver.Response, error) {
	var err error
	var service *apiv1.Service

	if service, err = h.poolManager.FetchService(ctx, input); err != nil {
		return nil, fmt.Errorf("could not fetch service: %w", err)
	}

	bindings := make(map[string]string)
	for _, port := range service.Spec.Ports {
		host := fmt.Sprintf("%s.%s", service.GetName(), service.Namespace)
		bindings[port.Name] = net.JoinHostPort(host, fmt.Sprint(port.Port))
	}

	return httpserver.NewJsonResponse(bindings), nil
}

func (h *HandlerServices) HandleStop(ctx context.Context, input *StopInput) (httpserver.Response, error) {
	if err := h.poolManager.ReleaseServices(ctx, input); err != nil {
		return nil, fmt.Errorf("could not fetch service: %w", err)
	}

	return httpserver.NewStatusResponse(200), nil
}
