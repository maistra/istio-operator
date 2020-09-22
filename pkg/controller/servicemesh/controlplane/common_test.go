package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	goruntime "runtime"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/versions"
	routev1 "github.com/openshift/api/route/v1"
)

var ctx = common.NewContextWithLog(context.Background(), logf.Log)

type IntegrationTestValidation struct {
	Assertions []test.ActionAssertion
	Verifier   test.ActionVerifier
}

type IntegrationTestCase struct {
	name      string
	smcp      *v2.ServiceMeshControlPlane
	resources []runtime.Object
	create    IntegrationTestValidation
	delete    IntegrationTestValidation
}

func RunSimpleInstallTest(t *testing.T, testCases []IntegrationTestCase) {
	t.Helper()
	const (
		operatorNamespace = "istio-operator"
		cniDaemonSetName  = "istio-node"
		domain            = "test.com"
	)
	if testing.Verbose() {
		logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(os.Stderr), zap.Level(zapcore.Level(-5))))
	}

	for _, tc := range testCases {
		ctc := test.ControllerTestCase{
			Name:             tc.name,
			ConfigureGlobals: InitializeGlobals(operatorNamespace),
			AddControllers:   []test.AddControllerFunc{Add},
			Resources: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNamespace}},
			},
			GroupResources: []*restmapper.APIGroupResources{
				CNIGroupResources,
				//MaistraGroupResources,
			},
			StorageVersions: []schema.GroupVersion{maistrav2.SchemeGroupVersion},
			Events: []test.ControllerTestEvent{
				{
					Name: "create-smcp",
					Execute: func(mgr *test.FakeManager, _ *test.EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), tc.smcp)
					},
					Verifier:   tc.create.Verifier,
					Assertions: tc.create.Assertions,
					Reactors: []clienttesting.Reactor{
						// make sure deployments come back as ready
						test.ReactTo("create").On("deployments").In(controlPlaneNamespace).With(SetDeploymentReady),
						// create reasonable default Host value
						test.ReactTo("create").On("routes").In(controlPlaneNamespace).With(SetRouteHostName(domain)),
						// create jaeger routes and services
						test.ReactTo("create").On("jaegers").In(controlPlaneNamespace).With(SimulateJaegerInstall(domain, nil)),
					},
					Timeout: 10 * time.Second,
				},
				{
					Name: "delete-smcp",
					Execute: func(mgr *test.FakeManager, _ *test.EnhancedTracker) error {
						return mgr.GetClient().Delete(context.TODO(), tc.smcp)
					},
					Verifier:   tc.delete.Verifier,
					Assertions: tc.delete.Assertions,
					Timeout:    10 * time.Second,
				},
			},
		}
		if tc.resources != nil {
			ctc.Resources = append(ctc.Resources, tc.resources...)
		}
		t.Run(tc.name, func(t *testing.T) {
			test.RunControllerTestCase(t, ctc)
		})
	}
}

func New20SMCPResource(name, namespace string, spec *v2.ControlPlaneSpec) *v2.ServiceMeshControlPlane {
	smcp := &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	spec.DeepCopyInto(&smcp.Spec)
	smcp.Spec.Version = versions.V2_0.String()
	smcp.Spec.Profiles = []string{"maistra"}
	return smcp
}

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
// control plane is installed.
func VerifyReadinessCheckOccurs(controlPlaneNamespace string) test.ActionVerifier {
	return test.VerifyActions(
		test.Verify("list").On("deployments").In(controlPlaneNamespace).IsSeen(),
		test.Verify("list").On("statefulsets").In(controlPlaneNamespace).IsSeen(),
		test.Verify("list").On("daemonsets").In(controlPlaneNamespace).IsSeen(),
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
