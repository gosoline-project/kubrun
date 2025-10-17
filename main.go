package main

import (
	"github.com/gosoline-project/httpserver"
	"github.com/justtrackio/gosoline/pkg/application"
)

func main() {
	httpserver.RunDefaultServer(NewRouter, []application.Option{
		application.WithModuleFactory("pool-manager", NewPoolModule),
	}...)
}
