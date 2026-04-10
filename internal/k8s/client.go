package k8s

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// K8sClient defines the interface for interacting with Kubernetes resources needed for maintenance mode.
type K8sClient interface {
	// EnsureService creates the specified service if it does not exist.
	EnsureService(ctx context.Context, name string, port, targetPort int32, labels map[string]string) error
	// IsMaintenanceMode returns true if the specified HTTPRoute points to the maintenance service.
	IsMaintenanceMode(ctx context.Context, routeName, maintenanceServiceName string) (bool, error)
	// SetMaintenanceMode updates the specified HTTPRoute to point to the target service.
	SetMaintenanceMode(ctx context.Context, routeName, targetServiceName string, servicePort int32) error
}

// Client is a Kubernetes client that implements the K8sClient interface.
type Client struct {
	clientset     kubernetes.Interface
	gatewayClient gatewayclient.Interface
	namespace     string
	log           *logrus.Entry
}

// Ensure Client implements K8sClient.
var _ K8sClient = &Client{}

// NewClient creates a new Client instance.
func NewClient(kubeconfig, namespace string, log *logrus.Logger) (*Client, error) {
	var config *rest.Config
	var err error
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating k8s client: %w", err)
	}

	gwClient, err := gatewayclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating gateway API client: %w", err)
	}

	return &Client{
		clientset:     clientset,
		gatewayClient: gwClient,
		namespace:     namespace,
		log:           log.WithField("component", "k8s-client"),
	}, nil
}

// EnsureService creates the service with the given name and labels if it doesn't already exist.
// It does not update the service if it already exists with different configuration; the assumption
// is that services are either created fresh or managed externally.
func (c *Client) EnsureService(ctx context.Context, name string, port, targetPort int32, labels map[string]string) error {
	log := c.log.WithFields(logrus.Fields{
		"service":    name,
		"port":       port,
		"targetPort": targetPort,
	})

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       port,
					TargetPort: intstr.FromInt32(targetPort),
				},
			},
		},
	}

	_, err := c.clientset.CoreV1().Services(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		log.Info("creating service")
		_, err = c.clientset.CoreV1().Services(c.namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create service %s: %w", name, err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get service %s: %w", name, err)
	}

	return nil
}

// IsMaintenanceMode returns true if the specified HTTPRoute points to the maintenance service.
func (c *Client) IsMaintenanceMode(ctx context.Context, routeName, maintenanceServiceName string) (bool, error) {
	route, err := c.gatewayClient.GatewayV1().HTTPRoutes(c.namespace).Get(ctx, routeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get HTTPRoute %s: %w", routeName, err)
	}

	for _, rule := range route.Spec.Rules {
		for _, backend := range rule.BackendRefs {
			if string(backend.Name) == maintenanceServiceName {
				return true, nil
			}
		}
	}

	return false, nil
}

// SetMaintenanceMode updates the specified HTTPRoute to point to the target service.
// It only updates rules that currently point to either the maintenance service or the normal DE UI service.
func (c *Client) SetMaintenanceMode(ctx context.Context, routeName, targetServiceName string, servicePort int32) error {
	log := c.log.WithFields(logrus.Fields{
		"route":  routeName,
		"target": targetServiceName,
	})

	route, err := c.gatewayClient.GatewayV1().HTTPRoutes(c.namespace).Get(ctx, routeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get HTTPRoute %s: %w", routeName, err)
	}

	targetPort := gatewayv1.PortNumber(servicePort)
	targetName := gatewayv1.ObjectName(targetServiceName)

	updated := false
	for i, rule := range route.Spec.Rules {
		shouldUpdateRule := false
		var oldTarget string
		for _, backend := range rule.BackendRefs {
			// A rule is a candidate for toggling if it currently points to a service.
			// This heuristic assumes that rules in this specific HTTPRoute are intended to be toggled.
			if backend.Name != "" {
				shouldUpdateRule = true
				oldTarget = string(backend.Name)
				break
			}
		}

		if shouldUpdateRule {
			log.WithFields(logrus.Fields{
				"ruleIndex": i,
				"oldTarget": oldTarget,
			}).Debug("updating HTTPRoute rule")

			route.Spec.Rules[i].BackendRefs = []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: targetName,
							Port: &targetPort,
						},
					},
				},
			}
			updated = true
		}
	}

	if !updated {
		return fmt.Errorf("no suitable rules found in HTTPRoute %s to update", routeName)
	}

	_, err = c.gatewayClient.GatewayV1().HTTPRoutes(c.namespace).Update(ctx, route, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update HTTPRoute %s: %w", routeName, err)
	}

	log.Info("successfully updated HTTPRoute maintenance mode")
	return nil
}
