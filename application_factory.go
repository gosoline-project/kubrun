package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/justtrackio/gosoline/pkg/mdl"
	"gitlab.justtrack.io/justtrack/devops/tools/justdev/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ApplicationFactory struct{}

func (f *ApplicationFactory) CreateDeployment(uid string, input SpawnAble) *appsv1.Deployment {
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
			Name:          types.K8sNameString(portName),
			Protocol:      apiv1.Protocol(strings.ToUpper(portConfig.Protocol)),
			ContainerPort: int32(portConfig.ContainerPort),
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
				AnnotationExpireAfter: time.Now().Add(time.Hour).Format(time.RFC3339),
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
					Containers: []apiv1.Container{container},
				},
			},
		},
	}

	return deployment
}

func (f *ApplicationFactory) CreateService(deployment *appsv1.Deployment, input *RunInput) *apiv1.Service {
	ports := make([]apiv1.ServicePort, 0)

	for portName, portConfig := range input.Spec.PortBindings {
		ports = append(ports, apiv1.ServicePort{
			Name:       types.K8sNameString(portName),
			Protocol:   apiv1.Protocol(strings.ToUpper(portConfig.Protocol)),
			Port:       int32(portConfig.ContainerPort),
			TargetPort: intstr.FromString(types.K8sNameString(portName)),
		})
	}

	serviceName := K8sNameString(
		"p",
		deployment.Labels[LabelPoolId],
		deployment.Labels[LableUid],
		deployment.Labels[LabelComponentType],
		deployment.Labels[LabelContainerName],
		input.ComponentName,
	)

	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   serviceName,
			Labels: deployment.GetLabels(),
			Annotations: map[string]string{
				AnnotationExpireAfter: time.Now().Add(input.ExpireAfter).Format(time.RFC3339),
			},
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				LabelPoolId:        deployment.Labels[LabelPoolId],
				LabelComponentType: deployment.Labels[LabelComponentType],
				LabelContainerName: deployment.Labels[LabelContainerName],
				LableUid:           deployment.Labels[LableUid],
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
