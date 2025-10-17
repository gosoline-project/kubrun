package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/exec"
	"github.com/justtrackio/gosoline/pkg/funk"
	"github.com/justtrackio/gosoline/pkg/log"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientApps "k8s.io/client-go/kubernetes/typed/apps/v1"
	clientCore "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewK8sClient(config cfg.Config, logger log.Logger) (*K8sClient, error) {
	var err error
	var settings *KubeSettings
	var clientConfig *rest.Config

	if settings, err = ReadSettings(config); err != nil {
		return nil, fmt.Errorf("could not read kube local settings: %w", err)
	}

	if settings.ClientMode == ClientModeInCluster {
		return newK8sClientInCluster(config, logger, settings)
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{
		CurrentContext: settings.ContextName,
	})

	if clientConfig, err = loader.ClientConfig(); err != nil {
		return nil, fmt.Errorf("could not load config: %w", err)
	}

	return newK8sClient(config, logger, clientConfig, settings)
}

func newK8sClientInCluster(config cfg.Config, logger log.Logger, settings *KubeSettings) (*K8sClient, error) {
	var err error
	var clientConfig *rest.Config

	if clientConfig, err = rest.InClusterConfig(); err != nil {
		return nil, fmt.Errorf("could not load in cluster config: %w", err)
	}

	return newK8sClient(config, logger, clientConfig, settings)
}

func newK8sClient(config cfg.Config, logger log.Logger, clientConfig *rest.Config, settings *KubeSettings) (*K8sClient, error) {
	var err error
	var client *kubernetes.Clientset

	if client, err = kubernetes.NewForConfig(clientConfig); err != nil {
		return nil, fmt.Errorf("could not create client: %w", err)
	}

	return &K8sClient{
		logger:      logger.WithChannel("k8s"),
		client:      client,
		deployments: client.AppsV1().Deployments(settings.Namespace),
		services:    client.CoreV1().Services(settings.Namespace),
	}, nil
}

type K8sClient struct {
	logger log.Logger
	client *kubernetes.Clientset

	deployments clientApps.DeploymentInterface
	services    clientCore.ServiceInterface
}

func (c K8sClient) ListDeployments(ctx context.Context, selectors ...map[string]string) ([]*appsv1.Deployment, error) {
	var err error
	var objects *appsv1.DeploymentList

	if objects, err = c.deployments.List(ctx, c.getListOptions(selectors...)); err != nil {
		return nil, fmt.Errorf("could not list deployments: %w", err)
	}

	return funk.Map(objects.Items, func(obj appsv1.Deployment) *appsv1.Deployment {
		return &obj
	}), nil
}

func (c K8sClient) CreateDeployment(ctx context.Context, object *appsv1.Deployment) (*appsv1.Deployment, error) {
	var err error
	var deployment *appsv1.Deployment

	if deployment, err = c.deployments.Create(ctx, object, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("could not create deployment: %w", err)
	}

	return deployment, nil
}

func (c K8sClient) DeleteDeployment(ctx context.Context, object Objecter) error {
	if err := c.deployments.Delete(ctx, object.GetName(), metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("could not delete deployment: %w", err)
	}

	return nil
}

func (c K8sClient) PatchDeployment(ctx context.Context, object *appsv1.Deployment, ops []string) (*appsv1.Deployment, error) {
	var err error
	var deployment *appsv1.Deployment

	patch := []byte(fmt.Sprintf("[%s]", strings.Join(ops, ",")))
	if deployment, err = c.deployments.Patch(ctx, object.GetName(), types.JSONPatchType, patch, metav1.PatchOptions{}); err != nil {
		return nil, fmt.Errorf("could not patch the deployment '%s': %w", object.GetName(), err)
	}

	return deployment, nil
}

func (c K8sClient) ListServices(ctx context.Context, selectors ...map[string]string) ([]*apiv1.Service, error) {
	var err error
	var objects *apiv1.ServiceList

	if objects, err = c.services.List(ctx, c.getListOptions(selectors...)); err != nil {
		return nil, fmt.Errorf("could not list services: %w", err)
	}

	return funk.Map(objects.Items, func(obj apiv1.Service) *apiv1.Service {
		return &obj
	}), nil
}

func (c K8sClient) CreateService(ctx context.Context, object *apiv1.Service) (*apiv1.Service, error) {
	var err error
	var service *apiv1.Service

	if service, err = c.services.Create(ctx, object, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("could not create service: %w", err)
	}

	return service, nil
}

func (c K8sClient) DeleteService(ctx context.Context, object Objecter) error {
	if err := c.services.Delete(ctx, object.GetName(), metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("could not delete deployment: %w", err)
	}

	return nil
}

func (k *K8sClient) getListOptions(selectors ...map[string]string) metav1.ListOptions {
	set := funk.MergeMaps(selectors...)
	selector := labels.SelectorFromSet(set)

	return metav1.ListOptions{
		LabelSelector: selector.String(),
	}
}

func resourceVersionConflictErrChecker(result any, err error) exec.ErrorType {
	// Check for Kubernetes conflict error (409) which indicates the object has been modified
	if k8sErrors.IsConflict(err) {
		return exec.ErrorTypeRetryable
	}

	return exec.ErrorTypePermanent
}
