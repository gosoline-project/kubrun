package main

import (
	"context"

	"github.com/gosoline-project/httpserver"
	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/log"
)

func NewRouter(ctx context.Context, config cfg.Config, logger log.Logger, router *httpserver.Router) error {
	router.HandleWith(httpserver.With(NewHandlerServices, func(router *httpserver.Router, handler *HandlerServices) {
		router.POST("/run", httpserver.Bind(handler.HandleRun))
		router.POST("/stop", httpserver.Bind(handler.HandleStop))
	}))

	router.HandleWith(httpserver.With(NewHandlerPool, func(router *httpserver.Router, handler *HandlerPool) {
		router.POST("/pool/warmup", httpserver.Bind(handler.HandleWarmUp))
		router.POST("/pool/shutdown", httpserver.Bind(handler.HandleShutdown))
	}))

	return nil
}
