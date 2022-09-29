package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/go-logr/logr"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

type ControllerResources struct {
	Client            client.Client
	Scheme            *runtime.Scheme
	EventRecorder     record.EventRecorder
	OperatorNamespace string
	DiscoveryClient   discovery.DiscoveryInterface
}

// UpdateField updates a nested field at the specified path, e.g.
// UpdateField(smcp.Spec.Istio, "global.proxy.image", "docker.io/maistra/proxyv2-ubi8:1.1.0")
func UpdateField(helmValues *v1.HelmValues, path string, value interface{}) error {
	return helmValues.SetField(path, value)
}

func IndexOf(l []string, s string) int {
	for i, elem := range l {
		if elem == s {
			return i
		}
	}
	return -1
}

func HasLabel(resource metav1.Object, label string) bool {
	labels := resource.GetLabels()
	if labels == nil {
		return false
	}
	_, ok := labels[label]
	return ok
}

func DeleteLabel(resource metav1.Object, label string) {
	labels := resource.GetLabels()
	if labels == nil {
		return
	}
	delete(labels, label)
	resource.SetLabels(labels)
}

func GetLabel(resource metav1.Object, label string) (value string, ok bool) {
	labels := resource.GetLabels()
	if labels == nil {
		return "", false
	}
	value, ok = labels[label]
	return
}

func SetLabels(resource metav1.Object, labels map[string]string) {
	existingLabels := resource.GetLabels()
	if existingLabels == nil {
		existingLabels = map[string]string{}
	}
	for key, value := range labels {
		existingLabels[key] = value
	}
	resource.SetLabels(existingLabels)
}

func SetLabel(resource metav1.Object, label, value string) {
	SetLabels(resource, map[string]string{label: value})
}

func HasAnnotation(resource metav1.Object, annotation string) bool {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[annotation]
	return ok
}

func DeleteAnnotation(resource metav1.Object, annotation string) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return
	}
	delete(annotations, annotation)
	resource.SetAnnotations(annotations)
}

func GetAnnotation(resource metav1.Object, annotation string) (value string, ok bool) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return "", false
	}
	value, ok = annotations[annotation]
	return
}

func SetAnnotation(resource metav1.Object, annotation, value string) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[annotation] = value
	resource.SetAnnotations(annotations)
}

var initOperatorNamespace sync.Once
var operatorNamespace string

// GetOperatorNamespace initializes and caches this operator's namespace; panics on error
func GetOperatorNamespace() string {
	initOperatorNamespace.Do(func() {
		var err error

		// Check for this variable before calling k8sutil.GetOperatorNamespace()
		// This variable is set in unit tests - and potentially in dev debugging sessions
		if operatorNamespace = os.Getenv("POD_NAMESPACE"); operatorNamespace != "" {
			return
		}

		if operatorNamespace, err = k8sutil.GetOperatorNamespace(); err != nil {
			panic(err)
		}
	})
	return operatorNamespace
}

func ToNamespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName()}
}

func BoolToConditionStatus(b bool) core.ConditionStatus {
	if b {
		return core.ConditionTrue
	} else {
		return core.ConditionFalse
	}
}

// GetMeshNamespaces returns all namespaces that are part of a mesh.
func GetMeshNamespaces(controlPlaneNamespace string, smmr *v1.ServiceMeshMemberRoll) sets.String {
	if controlPlaneNamespace == "" {
		return sets.NewString()
	}
	meshNamespaces := sets.NewString(controlPlaneNamespace)
	if smmr != nil {
		meshNamespaces.Insert(smmr.Status.ConfiguredMembers...)
	}
	return meshNamespaces
}

func NameSet(list runtime.Object) sets.String {
	set := sets.NewString()
	err := meta.EachListItem(list, func(obj runtime.Object) error {
		o, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		set.Insert(o.GetName())
		return nil
	})
	if err != nil {
		// meta.EachListItem only returns an error if you pass in something that's not a ResourceList, so
		// it we don't expect it to ever return an error.
		panic(err)
	}
	return set
}

// ConvertObjectToDeployment tries to convert an runtime.Object into a Deployment
func ConvertObjectToDeployment(obj runtime.Object) (*appsv1.Deployment, error) {
	var deployment *appsv1.Deployment

	switch typedObj := obj.(type) {
	case *appsv1.Deployment:
		deployment = typedObj
	case *unstructured.Unstructured:
		var j []byte
		var err error
		if j, err = json.Marshal(typedObj); err != nil {
			return nil, err
		}
		deployment = &appsv1.Deployment{}
		if err := json.Unmarshal(j, deployment); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("object is not an appsv1.Deployment: %T", obj)
	}

	return deployment, nil
}

// ConvertObjectToConfigMap tries to convert an runtime.Object into a ConfigMap
func ConvertObjectToConfigMap(obj runtime.Object) (*core.ConfigMap, error) {
	var cm *core.ConfigMap

	switch typedObj := obj.(type) {
	case *core.ConfigMap:
		cm = typedObj
	case *unstructured.Unstructured:
		var j []byte
		var err error
		if j, err = json.Marshal(typedObj); err != nil {
			return nil, err
		}
		cm = &core.ConfigMap{}
		if err := json.Unmarshal(j, cm); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("object is not a core.ConfigMap: %T", obj)
	}

	return cm, nil
}

func NewEnhancedManager(mgr manager.Manager, dc discovery.DiscoveryInterface) EnhancedManager {
	return EnhancedManager{
		delegate: mgr,
		dc:       dc,
	}
}

type EnhancedManager struct {
	delegate manager.Manager
	dc       discovery.DiscoveryInterface
}

func (m EnhancedManager) Add(runnable manager.Runnable) error {
	return m.delegate.Add(runnable)
}

func (m EnhancedManager) Elected() <-chan struct{} {
	return m.delegate.Elected()
}

func (m EnhancedManager) SetFields(fields interface{}) error {
	return m.delegate.SetFields(fields)
}

func (m EnhancedManager) AddMetricsExtraHandler(path string, handler http.Handler) error {
	return m.delegate.AddMetricsExtraHandler(path, handler)
}

func (m EnhancedManager) AddHealthzCheck(name string, check healthz.Checker) error {
	return m.delegate.AddHealthzCheck(name, check)
}

func (m EnhancedManager) AddReadyzCheck(name string, check healthz.Checker) error {
	return m.delegate.AddReadyzCheck(name, check)
}

func (m EnhancedManager) Start(ch <-chan struct{}) error {
	return m.delegate.Start(ch)
}

func (m EnhancedManager) GetConfig() *rest.Config {
	return m.delegate.GetConfig()
}

func (m EnhancedManager) GetScheme() *runtime.Scheme {
	return m.delegate.GetScheme()
}

func (m EnhancedManager) GetClient() client.Client {
	return m.delegate.GetClient()
}

func (m EnhancedManager) GetFieldIndexer() client.FieldIndexer {
	return m.delegate.GetFieldIndexer()
}

func (m EnhancedManager) GetCache() cache.Cache {
	return m.delegate.GetCache()
}

func (m EnhancedManager) GetEventRecorderFor(name string) record.EventRecorder {
	return m.delegate.GetEventRecorderFor(name)
}

func (m EnhancedManager) GetRESTMapper() meta.RESTMapper {
	return m.delegate.GetRESTMapper()
}

func (m EnhancedManager) GetAPIReader() client.Reader {
	return m.delegate.GetAPIReader()
}

func (m EnhancedManager) GetWebhookServer() *webhook.Server {
	return m.delegate.GetWebhookServer()
}

func (m EnhancedManager) GetLogger() logr.Logger {
	return m.delegate.GetLogger()
}

func (m EnhancedManager) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return m.dc, nil
}
