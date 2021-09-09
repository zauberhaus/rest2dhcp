/*
Copyright © 2020 Dirk Lembke <dirk@lembke.nz>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes_test

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/kubernetes"
	"github.com/zauberhaus/rest2dhcp/logger"
	"github.com/zauberhaus/rest2dhcp/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestNewKubeClient(t *testing.T) {
	logger := mock.NewTestLogger()
	client, err := kubernetes.NewKubeClient("", logger)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.EqualError(t, err, "invalid configuration: no configuration has been provided, try setting KUBERNETES_MASTER environment variable")

	client, err = kubernetes.NewKubeClient("./testdata/config.yaml", logger)
	assert.NoError(t, err)
	assert.NotNil(t, client)

}

func TestKubeClientImpl_GetConfigMap(t *testing.T) {
	logger := mock.NewTestLogger()
	clientset := testclient.NewSimpleClientset()

	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns001",
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	cm, err := clientset.CoreV1().ConfigMaps(ns.ObjectMeta.Name).Create(ctx, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cm001",
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	result, err := k.GetConfigMap(ctx, ns.GetObjectMeta().GetName(), "abc")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "configmaps \"abc\" not found", err.Error())

	result, err = k.GetConfigMap(ctx, ns.GetObjectMeta().GetName(), cm.GetObjectMeta().GetName())
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, cm.ObjectMeta.Name, result.ObjectMeta.Name)

}

func TestKubeClientImpl_GetService(t *testing.T) {
	logger := mock.NewTestLogger()
	clientset := testclient.NewSimpleClientset()

	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns001",
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "svc001",
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	result, err := k.GetService(ctx, ns.GetObjectMeta().GetName(), "abc")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "services \"abc\" not found", err.Error())

	result, err = k.GetService(ctx, ns.GetObjectMeta().GetName(), svc.GetObjectMeta().GetName())
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, svc.ObjectMeta.Name, result.ObjectMeta.Name)

}

func TestKubeClientImpl_GetServicesForLB(t *testing.T) {
	logger := mock.NewTestLogger()
	clientset := testclient.NewSimpleClientset()

	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns001",
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	svc1, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "svc001",
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeLoadBalancer,
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc1)

	svc2, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "svc002",
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc2)

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	result, err := k.GetServicesForLB(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, svc1.ObjectMeta.Name, result[0].ObjectMeta.Name)
}

func TestKubeClientImpl_GetConfig(t *testing.T) {
	logger := mock.NewTestLogger()
	clientset := testclient.NewSimpleClientset()

	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns001",
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	cm, err := clientset.CoreV1().ConfigMaps(ns.ObjectMeta.Name).Create(ctx, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cm001",
		},
		Data: map[string]string{
			"test": "abcdefg",
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, cm)

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	result, err := k.GetConfig(ctx, ns.GetObjectMeta().GetName(), cm.GetObjectMeta().GetName(), "test")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, result, "abcdefg")

	result, err = k.GetConfig(ctx, ns.GetObjectMeta().GetName(), cm.GetObjectMeta().GetName(), "test2")
	assert.Error(t, err, "config entry not found")
	assert.NotNil(t, result)

	result, err = k.GetConfig(ctx, ns.GetObjectMeta().GetName(), "cm002", "test")
	assert.Error(t, err, "configmaps \"cm002\" not found")
	assert.NotNil(t, result)
}

func TestKubeClientImpl_Patch(t *testing.T) {
	logger := mock.NewTestLogger()
	clientset := testclient.NewSimpleClientset()

	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns001",
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "svc001",
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	patch := kubernetes.NewPatch()
	patch.SetAnnotation(svc, "test", "123")

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	result, err := k.PatchService(ctx, ns.GetObjectMeta().GetName(), svc.GetObjectMeta().GetName(), patch)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	a, ok := result.GetAnnotations()["test"]
	assert.True(t, ok)
	assert.Equal(t, a, "123")

}

func TestKubeClientImpl_PatchFailed(t *testing.T) {
	logger := mock.NewTestLogger()
	clientset := testclient.NewSimpleClientset()

	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns001",
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "svc001",
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	patch := kubernetes.NewPatch()
	patch.SetAnnotation(svc, "test", "123")

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	result, err := k.PatchService(ctx, ns.GetObjectMeta().GetName(), "abc", patch)
	assert.Error(t, fmt.Errorf("services \"abc\" not found"), err)
	assert.Nil(t, result)
}

func TestKubeClientImpl_GetEXternalIP(t *testing.T) {
	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	tests := []struct {
		name    string
		svc     *v1.Service
		svc_err error
		want    net.IP
		logs    []int64
		err     error
	}{
		{
			name: "LoadBalancerIP",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{
							{IP: local.String()},
						},
					},
				},
			},
			want: local,
			logs: []int64{0, 0, 0, 1, 0, 0},
		},
		{
			name: "LoadBalancerHostname",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{
							{Hostname: "localhost"},
						},
					},
				},
			},
			want: net.IP{127, 0, 0, 1},
			logs: []int64{0, 0, 0, 1, 0, 0},
		},
		{
			name: "No ExternalIPs",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{},
					},
				},
			},
			err:  fmt.Errorf("service ns001/svc001 has no external IP"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name: "Multiple ExternalIPs",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{
							{IP: local.String()},
							{IP: remote.String()},
						},
					},
				},
			},
			err:  fmt.Errorf("service ns001/svc001 has multiple external IPs"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name: "Invalid ExternalIPs",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{
							{IP: "abc"},
						},
					},
				},
			},
			err:  fmt.Errorf("invalid external IP format 'abc' for service ns001/svc001"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name: "No Ingress",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{},
				},
			},
			err:  fmt.Errorf("service ns001/svc001 has no external IP"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name: "No LoadBalancer",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Status: v1.ServiceStatus{},
			},
			err:  fmt.Errorf("service ns001/svc001 has no external IP"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name: "No Status",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
			},
			err:  fmt.Errorf("service ns001/svc001 has no external IP"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := tt.svc.ObjectMeta.Namespace

			logger := mock.NewTestLogger()
			defer logger.Assert(t, tt.logs...)

			clientset := testclient.NewSimpleClientset()
			ctx := context.Background()

			ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}, metav1.CreateOptions{})

			assert.NoError(t, err)
			assert.NotNil(t, ns)

			svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, tt.svc, metav1.CreateOptions{})
			assert.NoError(t, err)
			assert.NotNil(t, svc)

			k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
			assert.NoError(t, err)
			setClientSet(k, clientset)

			ip, err := k.GetExternalIP(ctx, namespace, tt.svc.ObjectMeta.Name)
			if tt.err == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, ip.To4())
			} else {
				assert.EqualError(t, err, tt.err.Error())
			}
		})
	}
}

func TestKubeClientImpl_GetExternalIPCache(t *testing.T) {
	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	o := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc001",
			Namespace: "ns001",
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{IP: local.String()},
				},
			},
		},
	}

	namespace := o.ObjectMeta.Namespace

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 1, 0, 0)

	clientset := testclient.NewSimpleClientset()
	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, o, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	ip, err := k.GetExternalIP(ctx, namespace, o.ObjectMeta.Name)
	assert.NoError(t, err)
	assert.Equal(t, local, ip.To4())

	svc.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{
		{
			IP: remote.String(),
		},
	}

	ip, err = k.GetExternalIP(ctx, namespace, o.ObjectMeta.Name)
	assert.NoError(t, err)
	assert.Equal(t, local, ip.To4())
}

func TestKubeClientImpl_GetExternalIPServiceNotFound(t *testing.T) {
	local := net.IP{1, 1, 1, 1}

	o := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc001",
			Namespace: "ns001",
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{IP: local.String()},
				},
			},
		},
	}

	namespace := o.ObjectMeta.Namespace

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	clientset := testclient.NewSimpleClientset()
	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, o, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	ip, err := k.GetExternalIP(ctx, namespace, "abc")
	assert.Error(t, fmt.Errorf("esolve external IP from ns001/abc: services \"abc\" not found"), err)
	assert.Nil(t, ip)
}

func TestKubeClientImpl_Watch(t *testing.T) {
	logger := mock.NewTestLogger()
	clientset := testclient.NewSimpleClientset()

	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns001",
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "svc001",
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	k, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	assert.NoError(t, err)
	setClientSet(k, clientset)

	changed, cancel := k.WatchService(ctx)
	defer cancel()

	result := <-changed

	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Nil(t, result[0])
	assert.NotNil(t, result[1])
	assert.Equal(t, svc, result[1])

	patch := kubernetes.NewPatch()
	patch.SetAnnotation(svc, "test", "123")

	svc2, err := k.PatchService(ctx, ns.GetObjectMeta().GetName(), svc.GetObjectMeta().GetName(), patch)
	assert.NoError(t, err)
	assert.NotNil(t, svc2)

	result = <-changed
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.NotNil(t, result[0])
	assert.NotNil(t, result[1])
	assert.Equal(t, svc, result[0])
	assert.Equal(t, svc2, result[1])

	err = clientset.CoreV1().Services(ns.GetObjectMeta().GetName()).Delete(ctx, svc.GetObjectMeta().GetName(), metav1.DeleteOptions{})
	assert.NoError(t, err)

	result = <-changed
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.NotNil(t, result[0])
	assert.Nil(t, result[1])
	assert.Equal(t, svc2, result[0])
}

func setClientSet(k kubernetes.KubeClient, c kube.Interface) {
	pointerVal := reflect.ValueOf(k)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName("client")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*kube.Interface)(ptrToY)
	*realPtrToY = c
}

func getClientSet(k interface{}) kube.Interface {
	pointerVal := reflect.ValueOf(k)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName("client")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*kube.Interface)(ptrToY)
	return *realPtrToY
}

func getLogger(k interface{}) logger.Logger {
	pointerVal := reflect.ValueOf(k)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName("logger")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*logger.Logger)(ptrToY)
	return *realPtrToY
}
