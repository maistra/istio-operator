package controlplane

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	goruntime "runtime"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
)

// InitializeGlobals returns a function which initializes global variables used
// by the system under test.  operatorNamespace is the namespace within which
// the operator is installed.
func InitializeGlobals(operatorNamespace string) func() {
	return func() {
		// make sure globals are initialized for testing
		common.Config.OLM.Images.V1_0.CNI = "istio-cni-test-1_0"
		common.Config.OLM.Images.V1_1.CNI = "istio-cni-test-1_1"
		common.Config.OLM.Images.V2_0.CNI = "istio-cni-test-2_0"
		os.Setenv("POD_NAMESPACE", operatorNamespace)
		common.GetOperatorNamespace()
		if _, filename, _, ok := goruntime.Caller(0); ok {
			common.Config.Rendering.ResourceDir = path.Join(path.Dir(filename), "../../../../resources")
			common.Config.Rendering.ChartsDir = path.Join(common.Config.Rendering.ResourceDir, "helm")
			common.Config.Rendering.DefaultTemplatesDir = path.Join(common.Config.Rendering.ResourceDir, "smcp-templates")
		} else {
			panic("could not initialize common.ResourceDir")
		}
	}
}

// CNIGroupResources is a restmapper.APIGroupResources representing
// k8s.cni.cncf.io resources.  This can be used with controller tests to
// verify proper initialization of CNI settings at runtime.
var CNIGroupResources = &restmapper.APIGroupResources{
	Group: metav1.APIGroup{
		Name: "k8s.cni.cncf.io",
		Versions: []metav1.GroupVersionForDiscovery{
			{Version: "v1"},
		},
	},
	VersionedResources: map[string][]metav1.APIResource{
		"v1": {
			metav1.APIResource{
				Name:         "network-attachment-definitions",
				SingularName: "network-attachment-definition",
				Namespaced:   false,
				Kind:         "NetworkAttachmentDefinition",
			},
		},
	},
}

// VerifyReadinessCheckOccurs returns an ActionVerifier which includes
// verifications for all actions that should be performed during a successful
// readiness check.  controlPlaneNamespace is the namespace within which the
// control plane is installed.  operatorNamespace is the namespace within which
// the operator is running.
func VerifyReadinessCheckOccurs(controlPlaneNamespace, operatorNamespace string) test.ActionVerifier {
	return test.VerifyActions(
		test.Verify("list").On("deployments").In(controlPlaneNamespace).IsSeen(),
		test.Verify("list").On("statefulsets").In(controlPlaneNamespace).IsSeen(),
		test.Verify("list").On("daemonsets").In(controlPlaneNamespace).IsSeen(),
		test.Verify("list").On("daemonsets").In(operatorNamespace).IsSeen(),
	)
}

func SetRouteHostName(domain string) func(action clienttesting.Action, tracker clienttesting.ObjectTracker) (applied bool, handled bool, obj runtime.Object, err error) {
	return func(action clienttesting.Action, tracker clienttesting.ObjectTracker) (applied bool, handled bool, obj runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		applied = false
		handled = true
		obj = createAction.GetObject()
		var route *routev1.Route
		switch typedObj := obj.(type) {
		case *routev1.Route:
			route = typedObj
		case *unstructured.Unstructured:
			var j []byte
			if j, err = json.Marshal(typedObj); err != nil {
				return
			}
			route = &routev1.Route{}
			if err = json.Unmarshal(j, route); err != nil {
				return
			}
		default:
			err = fmt.Errorf("object is not an routev1.Route: %T", obj)
			return
		}
		route.Spec.Host = fmt.Sprintf("%s.%s", route.Name, domain)
		err = tracker.Create(action.GetResource(), route, action.GetNamespace())
		return
	}
}

func SimulateJaegerInstall(domain string, tls *routev1.TLSConfig) func(action clienttesting.Action, tracker clienttesting.ObjectTracker) (applied bool, handled bool, obj runtime.Object, err error) {
	return func(action clienttesting.Action, tracker clienttesting.ObjectTracker) (applied bool, handled bool, obj runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		applied = false
		handled = false
		var metaobj metav1.Object
		metaobj, err = meta.Accessor(createAction.GetObject())
		if err != nil {
			return
		}
		route := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metaobj.GetName(),
				Namespace: metaobj.GetNamespace(),
				Labels: map[string]string{
					"app.kubernetes.io/instance":  metaobj.GetName(),
					"app.kubernetes.io/component": "query-route",
				},
			},
			Spec: routev1.RouteSpec{
				Host: fmt.Sprintf("%s.%s", metaobj.GetName(), domain),
				TLS:  tls,
			},
		}
		err = tracker.Create(routev1.GroupVersion.WithResource("routes"), route, action.GetNamespace())
		return
	}
}

func SetDeploymentReady(action clienttesting.Action, tracker clienttesting.ObjectTracker) (applied bool, handled bool, obj runtime.Object, err error) {
	createAction := action.(clienttesting.CreateAction)
	applied = false
	handled = true
	obj = createAction.GetObject()
	var deployment *appsv1.Deployment
	switch typedObj := obj.(type) {
	case *appsv1.Deployment:
		deployment = typedObj
	case *unstructured.Unstructured:
		var j []byte
		if j, err = json.Marshal(typedObj); err != nil {
			return
		}
		deployment = &appsv1.Deployment{}
		if err = json.Unmarshal(j, deployment); err != nil {
			return
		}
	default:
		err = fmt.Errorf("object is not an appsv1.Deployment: %T", obj)
		return
	}

	deployment.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		},
	}
	if deployment.Spec.Replicas == nil {
		deployment.Status.AvailableReplicas = 1
	} else {
		deployment.Status.AvailableReplicas = *deployment.Spec.Replicas
	}
	err = tracker.Create(action.GetResource(), deployment, action.GetNamespace())
	return
}
