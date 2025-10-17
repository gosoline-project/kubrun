package main

import (
	"fmt"

	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/exec"
)

const (
	ClientModeInCluster  = "in-cluster"
	ClientModeKubeConfig = "kube-config"
)

type KubeSettings struct {
	ClientMode  string `cfg:"client_mode" default:"in-cluster"`
	ContextName string `cfg:"context_name"`
	Namespace   string `cfg:"namespace" default:"justdev"`

	Backoff exec.BackoffSettings `cfg:"backoff"`
}

func ReadSettings(config cfg.Config) (*KubeSettings, error) {
	settings := &KubeSettings{}
	if err := config.UnmarshalKey("k8s", settings); err != nil {
		return nil, fmt.Errorf("could not unmarshal k8s settings: %w", err)
	}

	return settings, nil
}
