package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zauberhaus/rest2dhcp/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const tag = "kube"

type KubeClient interface {
	GetServicesForLB(ctx context.Context) ([]*v1.Service, error)
	GetConfig(ctx context.Context, namespace string, name string, config string) (string, error)
	GetService(ctx context.Context, namespace string, name string) (*v1.Service, error)
	Patch(ctx context.Context, namespace string, name string, patch *Patch) (metav1.Object, error)
	Watch(ctx context.Context, resource string, objType runtime.Object) chan [2]metav1.Object
}

type KubeClientImpl struct {
	clientset *kubernetes.Clientset
	logger    logger.Logger
}

type patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type patchUint64 struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value uint64 `json:"value"`
}

type patchStruct struct {
	Op    string   `json:"op"`
	Path  string   `json:"path"`
	Value struct{} `json:"value"`
}

func NewKubeClient(kubeconfig string, logger logger.Logger) KubeClient {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			logger.Errorf("BuildConfigFromFlags: %v", err)
			return nil
		}
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Errorf("NewForConfig: %v", err)
		return nil
	}

	return &KubeClientImpl{
		clientset: clientset,
		logger:    logger,
	}
}

func (k *KubeClientImpl) getConfigMap(ctx context.Context, namespace string, name string) (*v1.ConfigMap, error) {
	m, err := k.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (k *KubeClientImpl) GetService(ctx context.Context, namespace string, name string) (*v1.Service, error) {
	m, err := k.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (k *KubeClientImpl) GetServicesForLB(ctx context.Context) ([]*v1.Service, error) {
	services, err := k.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
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

func (k *KubeClientImpl) GetConfig(ctx context.Context, namespace string, name string, config string) (string, error) {
	cm, err := k.getConfigMap(ctx, namespace, name)
	if err != nil {
		return "", err
	}

	cfg, ok := cm.Data[config]

	if !ok {
		return "", fmt.Errorf("Config entry not found")
	}

	return cfg, nil
}

func (k *KubeClientImpl) Patch(ctx context.Context, namespace string, name string, patch *Patch) (metav1.Object, error) {

	data, err := json.Marshal(patch.set)
	if err != nil {
		return nil, err
	}

	result, err := k.clientset.CoreV1().Services(namespace).Patch(ctx, name, types.JSONPatchType, data, metav1.PatchOptions{})
	if err == nil {
		return result, nil
	}

	return nil, err
}

func (k *KubeClientImpl) Watch(ctx context.Context, resource string, objType runtime.Object) chan [2]metav1.Object {
	changed := make(chan [2]metav1.Object, 1)

	watchlist := cache.NewListWatchFromClient(k.clientset.CoreV1().RESTClient(), resource, v1.NamespaceAll, fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		objType,
		0*time.Second,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(o interface{}) {
				obj, ok := o.(metav1.Object)
				if ok {
					k.logger.Infof("Added to "+resource+": %s/%s", obj.GetNamespace(), obj.GetName())
					changed <- [2]metav1.Object{nil, obj}
				}
			},
			DeleteFunc: func(o interface{}) {
				obj, ok := o.(metav1.Object)
				if ok {
					k.logger.Infof("Deleted from "+resource+": %s/%s", obj.GetNamespace(), obj.GetName())
					changed <- [2]metav1.Object{obj, nil}
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				old, ok := oldObj.(metav1.Object)
				if ok {
					new, ok := newObj.(metav1.Object)
					if ok {
						k.logger.Infof("Changed in "+resource+": %s/%s", old.GetNamespace(), new.GetName())
						changed <- [2]metav1.Object{old, new}
					}
				}
			},
		},
	)

	go controller.Run(ctx.Done())

	return changed
}
