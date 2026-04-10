package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayfake "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/fake"
)

func TestEnsureService(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	gwClient := gatewayfake.NewClientset()
	namespace := "test-ns"
	log := logrus.New()
	client := &Client{
		clientset:     clientset,
		gatewayClient: gwClient,
		namespace:     namespace,
		log:           log.WithField("component", "k8s-client"),
	}

	ctx := context.Background()
	name := "test-service"
	port := int32(80)
	targetPort := int32(8080)
	labels := map[string]string{"app": "test"}

	// Test creation
	err := client.EnsureService(ctx, name, port, targetPort, labels)
	require.NoError(t, err)

	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, name, svc.Name)
	assert.Equal(t, port, svc.Spec.Ports[0].Port)
	assert.Equal(t, targetPort, svc.Spec.Ports[0].TargetPort.IntVal)

	// Test already exists
	err = client.EnsureService(ctx, name, port, targetPort, labels)
	require.NoError(t, err)
}

func TestIsMaintenanceMode(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	namespace := "test-ns"
	routeName := "test-route"
	maintSvcName := "maint-svc"

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(maintSvcName),
								},
							},
						},
					},
				},
			},
		},
	}

	gwClient := gatewayfake.NewClientset(route)
	log := logrus.New()
	client := &Client{
		clientset:     clientset,
		gatewayClient: gwClient,
		namespace:     namespace,
		log:           log.WithField("component", "k8s-client"),
	}

	ctx := context.Background()

	// Test ON
	isMaint, err := client.IsMaintenanceMode(ctx, routeName, maintSvcName)
	require.NoError(t, err)
	assert.True(t, isMaint)

	// Test OFF (different service name)
	isMaint, err = client.IsMaintenanceMode(ctx, routeName, "other-svc")
	require.NoError(t, err)
	assert.False(t, isMaint)

	// Test Route Not Found
	_, err = client.IsMaintenanceMode(ctx, "non-existent", maintSvcName)
	require.Error(t, err)
}

func TestSetMaintenanceMode(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	namespace := "test-ns"
	routeName := "test-route"

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName("sonora"),
								},
							},
						},
					},
				},
			},
		},
	}

	gwClient := gatewayfake.NewClientset(route)
	log := logrus.New()
	client := &Client{
		clientset:     clientset,
		gatewayClient: gwClient,
		namespace:     namespace,
		log:           log.WithField("component", "k8s-client"),
	}

	ctx := context.Background()

	// Set to maintenance
	err := client.SetMaintenanceMode(ctx, routeName, "maint-svc", 80, []string{"sonora", "maint-svc"})
	require.NoError(t, err)

	updatedRoute, err := gwClient.GatewayV1().HTTPRoutes(namespace).Get(ctx, routeName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, gatewayv1.ObjectName("maint-svc"), updatedRoute.Spec.Rules[0].BackendRefs[0].Name)

	// Set back to sonora
	err = client.SetMaintenanceMode(ctx, routeName, "sonora", 80, []string{"sonora", "maint-svc"})
	require.NoError(t, err)

	updatedRoute, err = gwClient.GatewayV1().HTTPRoutes(namespace).Get(ctx, routeName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, gatewayv1.ObjectName("sonora"), updatedRoute.Spec.Rules[0].BackendRefs[0].Name)

	// Test with multiple rules
	multiRuleRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-rule",
			Namespace: namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName("sonora"),
								},
							},
						},
					},
				},
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName("maint-svc"),
								},
							},
						},
					},
				},
			},
		},
	}
	_, _ = gwClient.GatewayV1().HTTPRoutes(namespace).Create(ctx, multiRuleRoute, metav1.CreateOptions{})
	err = client.SetMaintenanceMode(ctx, "multi-rule", "new-svc", 80, []string{"sonora", "maint-svc"})
	require.NoError(t, err)

	updatedRoute, err = gwClient.GatewayV1().HTTPRoutes(namespace).Get(ctx, "multi-rule", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, gatewayv1.ObjectName("new-svc"), updatedRoute.Spec.Rules[0].BackendRefs[0].Name)
	assert.Equal(t, gatewayv1.ObjectName("new-svc"), updatedRoute.Spec.Rules[1].BackendRefs[0].Name)

	// Test no suitable rules found
	routeNoRules := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-rules",
			Namespace: namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{},
				},
			},
		},
	}
	_, _ = gwClient.GatewayV1().HTTPRoutes(namespace).Create(ctx, routeNoRules, metav1.CreateOptions{})
	err = client.SetMaintenanceMode(ctx, "no-rules", "maint-svc", 80, []string{"sonora", "maint-svc"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoSuitableRules))
}
