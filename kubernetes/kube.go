package kubernetes

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/zauberhaus/rest2dhcp/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ReneKroon/ttlcache/v2"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type KubeClient interface {
	GetServicesForLB(ctx context.Context) ([]*v1.Service, error)
	GetConfig(ctx context.Context, namespace string, name string, config string) (string, error)
	GetService(ctx context.Context, namespace string, name string) (*v1.Service, error)
	GetConfigMap(ctx context.Context, namespace string, name string) (*v1.ConfigMap, error)
	PatchService(ctx context.Context, namespace string, name string, patch *Patch) (metav1.Object, error)
	WatchService(ctx context.Context) (chan [2]metav1.Object, context.CancelFunc)
	GetExternalIP(ctx context.Context, namespace string, service string) (net.IP, error)
}

type kubeClientImpl struct {
	client kubernetes.Interface
	logger logger.Logger

	cache ttlcache.SimpleCache
}

type patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type patchStruct struct {
	Op    string   `json:"op"`
	Path  string   `json:"path"`
	Value struct{} `json:"value"`
}

func NewKubeClient(kubeconfig string, logger logger.Logger) (KubeClient, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	cache := ttlcache.NewCache()
	cache.SetTTL(60 * time.Second)

	return &kubeClientImpl{
		client: clientset,
		logger: logger,
		cache:  cache,
	}, nil
}

func (k *kubeClientImpl) GetConfigMap(ctx context.Context, namespace string, name string) (*v1.ConfigMap, error) {
	m, err := k.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (k *kubeClientImpl) GetService(ctx context.Context, namespace string, name string) (*v1.Service, error) {
	m, err := k.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (k *kubeClientImpl) GetServicesForLB(ctx context.Context) ([]*v1.Service, error) {
	services, err := k.client.CoreV1().Services(v1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	rc := []*v1.Service{}

	for _, item := range services.Items {
		if item.Spec.Type == v1.ServiceTypeLoadBalancer {
			rc = append(rc, item.DeepCopy())
		}
	}

	return rc, nil
}

func (k *kubeClientImpl) GetConfig(ctx context.Context, namespace string, name string, config string) (string, error) {
	cm, err := k.GetConfigMap(ctx, namespace, name)
	if err != nil {
		return "", err
	}

	cfg, ok := cm.Data[config]

	if !ok {
		return "", fmt.Errorf("config entry not found")
	}

	return cfg, nil
}

func (k *kubeClientImpl) PatchService(ctx context.Context, namespace string, name string, patch *Patch) (metav1.Object, error) {

	data := []byte(patch.String())

	result, err := k.client.CoreV1().Services(namespace).Patch(ctx, name, types.JSONPatchType, data, metav1.PatchOptions{})
	if err == nil {
		return result, nil
	}

	return nil, err
}

func (k *kubeClientImpl) GetExternalIP(ctx context.Context, namespace string, service string) (net.IP, error) {
	if value, exists := k.cache.Get(namespace + "/" + service); exists == nil {
		return value.(net.IP), nil
	}

	result, err := k.GetService(ctx, namespace, service)
	if err != nil {
		return nil, fmt.Errorf("resolve external IP from %s/%s: %v", namespace, service, err)
	}

	ingress := result.Status.LoadBalancer.Ingress
	ips := []string{}
	for _, i := range ingress {
		if len(i.IP) > 0 {
			ips = append(ips, i.IP)
		} else if len(i.Hostname) > 0 {
			tmp, err := net.LookupIP(i.Hostname)
			if err == nil {
				for _, ip := range tmp {
					ip4 := ip.To4()
					if ip4 != nil {
						ips = append(ips, ip4.String())
					}
				}
			}
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("service %s/%s has no external IP", result.ObjectMeta.Namespace, result.ObjectMeta.Name)
	}

	if len(ips) > 1 {
		return nil, fmt.Errorf("service %s/%s has multiple external IPs", result.ObjectMeta.Namespace, result.ObjectMeta.Name)
	}

	ip := net.ParseIP(ips[0])
	if ip != nil {
		k.logger.Infof("Detect Kubernetes external service ip %v (%s/%s)", ip, result.ObjectMeta.Namespace, result.ObjectMeta.Name)
		k.cache.Set(namespace+"/"+service, ip)

		return ip, nil
	} else {
		return nil, fmt.Errorf("invalid external IP format '%s' for service %v/%v", ips[0], result.ObjectMeta.Namespace, result.ObjectMeta.Name)
	}
}

func (k *kubeClientImpl) WatchService(ctx context.Context) (chan [2]metav1.Object, context.CancelFunc) {
	changed := make(chan [2]metav1.Object, 1)
	stopper := make(chan struct{})

	cancel := func() {
		close(stopper)
	}

	factory := informers.NewSharedInformerFactory(k.client, 0)
	informer := factory.Core().V1().Services().Informer()

	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(o interface{}) {
				obj, ok := o.(metav1.Object)
				if ok {
					k.logger.Infof("Added service: %s/%s", obj.GetNamespace(), obj.GetName())
					changed <- [2]metav1.Object{nil, obj}
				}
			},
			DeleteFunc: func(o interface{}) {
				obj, ok := o.(metav1.Object)
				if ok {
					k.logger.Infof("Deleted service: %s/%s", obj.GetNamespace(), obj.GetName())
					changed <- [2]metav1.Object{obj, nil}
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				old, ok := oldObj.(metav1.Object)
				if ok {
					new, ok := newObj.(metav1.Object)
					if ok {
						k.logger.Infof("Changed service: %s/%s", old.GetNamespace(), new.GetName())
						changed <- [2]metav1.Object{old, new}
					}
				}
			},
		},
	)

	go informer.Run(stopper)

	return changed, cancel
}
