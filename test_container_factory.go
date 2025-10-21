package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/mdl"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type TestContainerSettings struct {
	NodeSelector map[string]string         `cfg:"node_selector"`
	Tolerations  []TestContainerToleration `cfg:"tolerations"`
}

type TestContainerToleration struct {
	Key      string `cfg:"key"`
	Operator string `cfg:"operator" default:"Equal"`
	Value    string `cfg:"value"`
	Effect   string `cfg:"effect"`
}

type TestContainerFactory struct {
	settings *TestContainerSettings
}

func NewTestContainerFactory(config cfg.Config) (*TestContainerFactory, error) {
	settings := &TestContainerSettings{}
	if err := config.UnmarshalKey("testcontainers.default", settings); err != nil {
		return nil, fmt.Errorf("can not unmarshal test container settings: %w", err)
	}

	return &TestContainerFactory{
		settings: settings,
	}, nil
}

func (f *TestContainerFactory) CreateDeployment(uid string, input SpawnAble) *appsv1.Deployment {
	spec := input.GetSpec()

	container := apiv1.Container{
		Name:  "main",
		Image: fmt.Sprintf("%s:%s", spec.Repository, spec.Tag),
		Args:  spec.Cmd,
		Env:   []apiv1.EnvVar{},
		Resources: apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceCPU:    resource.MustParse("300m"),
				apiv1.ResourceMemory: resource.MustParse("300Mi"),
			},
		},
	}

	for k, v := range spec.Env {
		container.Env = append(container.Env, apiv1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	for portName, portConfig := range spec.PortBindings {
		container.Ports = append(container.Ports, apiv1.ContainerPort{
			Name:          K8sNameString(portName),
			Protocol:      apiv1.Protocol(strings.ToUpper(portConfig.Protocol)),
			ContainerPort: int32(portConfig.ContainerPort),
		})
	}

	nodeSelector := map[string]string{}
	for key, value := range f.settings.NodeSelector {
		key = strings.ReplaceAll(key, "\\", "")
		nodeSelector[key] = value
	}

	tolerations := make([]apiv1.Toleration, 0)
	for _, t := range f.settings.Tolerations {
		tolerations = append(tolerations, apiv1.Toleration{
			Key:    t.Key,
			Value:  t.Value,
			Effect: apiv1.TaintEffect(t.Effect),
		})
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: K8sNameString("p", input.GetPoolId(), uid, input.GetComponentType(), input.GetContainerName()),
			Labels: map[string]string{
				LabelPoolId:        K8sNameString(input.GetPoolId()),
				LableUid:           uid,
				LabelComponentType: K8sNameString(input.GetComponentType()),
				LabelContainerName: K8sNameString(input.GetContainerName()),
				LableIdle:          "true",
			},
			Annotations: map[string]string{
				AnnotationComponentType: input.GetComponentType(),
				AnnotationContainerName: input.GetContainerName(),
				AnnotationExpireAfter:   time.Now().Add(time.Hour).Format(time.RFC3339),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: mdl.Box(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					LabelPoolId:        K8sNameString(input.GetPoolId()),
					LabelComponentType: K8sNameString(input.GetComponentType()),
					LabelContainerName: K8sNameString(input.GetContainerName()),
					LableUid:           uid,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelPoolId:        K8sNameString(input.GetPoolId()),
						LabelComponentType: K8sNameString(input.GetComponentType()),
						LabelContainerName: K8sNameString(input.GetContainerName()),
						LableUid:           uid,
					},
				},
				Spec: apiv1.PodSpec{
					Containers:   []apiv1.Container{container},
					NodeSelector: nodeSelector,
					Tolerations:  tolerations,
				},
			},
		},
	}

	return deployment
}

func (f *TestContainerFactory) CreateService(uid string, input SpawnAble) *apiv1.Service {
	spec := input.GetSpec()

	ports := make([]apiv1.ServicePort, 0)
	for portName, portConfig := range spec.PortBindings {
		ports = append(ports, apiv1.ServicePort{
			Name:       K8sNameString(portName),
			Protocol:   apiv1.Protocol(strings.ToUpper(portConfig.Protocol)),
			Port:       int32(portConfig.ContainerPort),
			TargetPort: intstr.FromString(K8sNameString(portName)),
		})
	}

	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: K8sNameString("p", input.GetPoolId(), uid, input.GetComponentType(), input.GetContainerName()),
			Labels: map[string]string{
				LabelPoolId:        K8sNameString(input.GetPoolId()),
				LableUid:           uid,
				LabelComponentType: K8sNameString(input.GetComponentType()),
				LabelContainerName: K8sNameString(input.GetContainerName()),
				LableIdle:          "true",
			},
			Annotations: map[string]string{
				AnnotationComponentType: input.GetComponentType(),
				AnnotationContainerName: input.GetContainerName(),
				AnnotationExpireAfter:   time.Now().Add(time.Hour).Format(time.RFC3339),
			},
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				LabelPoolId:        K8sNameString(input.GetPoolId()),
				LabelComponentType: K8sNameString(input.GetComponentType()),
				LabelContainerName: K8sNameString(input.GetContainerName()),
				LableUid:           uid,
			},
			Ports: ports,
			Type:  apiv1.ServiceTypeClusterIP,
		},
	}

	return service
}

var nonAlphanumericRegex = regexp.MustCompile(`[^-a-z0-9]+`)

func K8sNameString(strs ...string) string {
	str := strings.Join(strs, "-")
	str = strings.ToLower(str)

	return nonAlphanumericRegex.ReplaceAllString(str, "-")
}
