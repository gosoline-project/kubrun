package main

import "time"

const (
	AnnotationComponentType = "kubrun/component-type"
	AnnotationComponentName = "kubrun/component-name"
	AnnotationContainerName = "kubrun/container-name"
	AnnotationExpireAfter   = "kubrun/expire-after"
	AnnotationTestName      = "kubrun/test-name"

	LabelPoolId        = "kubrun/pool-id"
	LabelTestId        = "kubrun/test-id"
	LabelComponentType = "kubrun/component-type"
	LabelComponentName = "kubrun/component-name"
	LabelContainerName = "kubrun/container-name"
	LableIdle          = "kubrun/idle"
	LableUid           = "kubrun/uid"
)

type Labler interface {
	GetLabels() map[string]string
}

type Objecter interface {
	GetName() string
	GetAnnotations() map[string]string
	Labler
}

type SpawnAble interface {
	GetPoolId() string
	GetComponentType() string
	GetContainerName() string
	GetSpec() ContainerSpec
}

type WarmUpDeployment struct {
	PoolId        string        `json:"pool_id"`
	ComponentType string        `json:"component_type"`
	ContainerName string        `json:"container_name"`
	Spec          ContainerSpec `json:"spec"`
}

func (i WarmUpDeployment) GetPoolId() string {
	return i.PoolId
}

func (i WarmUpDeployment) GetComponentType() string {
	return i.ComponentType
}

func (i WarmUpDeployment) GetContainerName() string {
	return i.ContainerName
}

func (i WarmUpDeployment) GetSpec() ContainerSpec {
	return i.Spec
}

type RunInput struct {
	PoolId        string        `json:"pool_id"`
	TestId        string        `json:"test_id"`
	TestName      string        `json:"test_name"`
	ComponentType string        `json:"component_type"`
	ComponentName string        `json:"component_name"`
	ContainerName string        `json:"container_name"`
	Spec          ContainerSpec `json:"spec"`
	ExpireAfter   time.Duration `json:"expire_after"`
}

func (i RunInput) GetPoolId() string {
	return i.PoolId
}

func (i RunInput) GetComponentType() string {
	return i.ComponentType
}

func (i RunInput) GetComponentName() string {
	return i.ComponentName
}

func (i RunInput) GetContainerName() string {
	return i.ContainerName
}

func (i RunInput) GetName() string {
	return K8sNameString("g", i.PoolId, i.TestId, i.ComponentType, i.ComponentName)
}

func (i RunInput) GetLabels() map[string]string {
	return map[string]string{
		LabelPoolId:        K8sNameString(i.PoolId),
		LabelTestId:        K8sNameString(i.TestId),
		LabelComponentType: K8sNameString(i.ComponentType),
		LabelComponentName: K8sNameString(i.ComponentName),
	}
}

func (i RunInput) GetSpec() ContainerSpec {
	return i.Spec
}

func (i RunInput) GetExpireAfter() time.Duration {
	return i.ExpireAfter
}

type ExtendInput struct {
	PoolId   string        `json:"pool_id"`
	TestId   string        `json:"test_id"`
	Duration time.Duration `json:"duration"`
}

func (i ExtendInput) GetLabels() map[string]string {
	return map[string]string{
		LabelPoolId: K8sNameString(i.PoolId),
		LabelTestId: K8sNameString(i.TestId),
	}
}

type StopInput struct {
	PoolId string `json:"pool_id"`
	TestId string `json:"test_id"`
}

func (i StopInput) GetLabels() map[string]string {
	return map[string]string{
		LabelPoolId: K8sNameString(i.PoolId),
		LabelTestId: K8sNameString(i.TestId),
	}
}

type ContainerSpec struct {
	Repository   string                 `json:"repository"`
	Tag          string                 `json:"tag"`
	Env          map[string]string      `json:"env"`
	Cmd          []string               `json:"cmd"`
	PortBindings map[string]PortBinding `json:"port_bindings"`
}

type PortBinding struct {
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Protocol      string `json:"protocol"`
}

type AnnotationsAware interface {
	GetAnnotations() map[string]string
}
