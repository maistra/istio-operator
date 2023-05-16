package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func TestDefaultInstall(t *testing.T) {
	testCases := []IntegrationTestCase{
		{
			name: "default." + versions.V2_2.String(),
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &maistrav2.ControlPlaneSpec{Version: versions.V2_2.String()}),
			create: IntegrationTestValidation{
				Assertions: generateAssertions("create", "istiod-"+controlPlaneName, "wasm-cacher-"+controlPlaneName,
					"istio-ingressgateway", "istio-egressgateway", "prometheus", "grafana"),
			},
			delete: IntegrationTestValidation{
				Assertions: generateAssertions("delete", "istiod-"+controlPlaneName, "wasm-cacher-"+controlPlaneName,
					"istio-ingressgateway", "istio-egressgateway", "prometheus", "grafana"),
			},
		},
	}
	for _, v := range versions.TestedVersions {
		if v.AtLeast(versions.V2_3) {
			testCases = append(testCases, IntegrationTestCase{
				name: "default." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &maistrav2.ControlPlaneSpec{Version: v.String()}),
				create: IntegrationTestValidation{
					Assertions: generateAssertions("create",
						"istiod-"+controlPlaneName, "istio-ingressgateway", "istio-egressgateway", "prometheus", "grafana"),
				},
				delete: IntegrationTestValidation{
					Assertions: generateAssertions("delete",
						"istiod-"+controlPlaneName, "istio-ingressgateway", "istio-egressgateway", "prometheus", "grafana"),
				},
			})
		}
	}
	RunSimpleInstallTests(t, testCases)
}

func generateAssertions(verb string, components ...string) []ActionAssertion {
	var assertions []ActionAssertion
	for _, c := range components {
		assertions = append(assertions, Assert(verb).On("deployments").Named(c).In(controlPlaneNamespace).IsSeen())
	}
	return assertions
}

func TestBootstrapping(t *testing.T) {
	const (
		operatorNamespace     = "istio-operator"
		controlPlaneNamespace = "test"
		smcpName              = "test"
	)

	cniDaemonSetNames := map[versions.Version]string{
		versions.V2_2: "istio-cni-node",
		versions.V2_3: "istio-cni-node-v2-3",
		versions.V2_4: "istio-cni-node-v2-4",
	}

	type testCase struct {
		version                 versions.Version
		smcp                    *maistrav2.ServiceMeshControlPlane
		crdCount                int
		cniDaemonSetName        string
		unexpectedCNIDaemonSets []ActionAssertion
	}

	var testCases []testCase
	for _, v := range versions.TestedVersions {
		testCases = append(testCases, testCase{
			version: v,
			smcp: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{Name: smcpName, Namespace: controlPlaneNamespace},
				Spec: maistrav2.ControlPlaneSpec{
					Version:  v.String(),
					Profiles: []string{"maistra"},
				},
			},
			cniDaemonSetName:        cniDaemonSetNames[v],
			crdCount:                19,
			unexpectedCNIDaemonSets: unexpectedCNIDaemonSetNames(cniDaemonSetNames, v),
		})
	}

	if testing.Verbose() {
		logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(os.Stderr), zap.Level(zapcore.Level(-5))))
	}

	for _, tc := range testCases {
		t.Run(tc.version.String(), func(t *testing.T) {
			assertions := ActionAssertions{
				// verify proper number of CRDs is created
				Assert("create").On("customresourcedefinitions").Version("v1").SeenCountIs(tc.crdCount),
			}
			assertions = append(assertions, tc.unexpectedCNIDaemonSets...)
			RunControllerTestCase(t, ControllerTestCase{
				Name:             "clean-install-cni-no-errors",
				ConfigureGlobals: InitializeGlobals(operatorNamespace),
				AddControllers:   []AddControllerFunc{Add},
				Resources: []runtime.Object{
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace}},
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNamespace}},
				},
				GroupResources: []*restmapper.APIGroupResources{
					CNIGroupResources,
				},
				StorageVersions: []schema.GroupVersion{maistrav2.SchemeGroupVersion},
				Events: []ControllerTestEvent{
					{
						Name: "bootstrap-clean-install-cni-no-errors",
						Execute: func(mgr *FakeManager, _ *EnhancedTracker) error {
							return mgr.GetClient().Create(context.TODO(), tc.smcp)
						},
						Verifier: VerifyActions(
							// add finalizer
							Verify("update").On("servicemeshcontrolplanes").Named(smcpName).In(controlPlaneNamespace).Passes(FinalizerAddedTest(common.FinalizerName)),
							// initialize status
							Verify("patch").On("servicemeshcontrolplanes/status").Named(smcpName).In(controlPlaneNamespace).Passes(initalStatusTest),
							// verify that a CRD is installed
							Verify("create").On("customresourcedefinitions").Version("v1").IsSeen(),
							// verify that the correct CNI daemonset is installed
							Verify("create").On("daemonsets").Named(tc.cniDaemonSetName).In(operatorNamespace).IsSeen(),
							// verify readiness check triggered daemon set creation
							VerifyReadinessCheckOccurs(controlPlaneNamespace),
						),
						Assertions: assertions,
						Reactors: []clienttesting.Reactor{
							ReactTo("list").On("daemonsets").In(operatorNamespace).With(
								SetDaemonSetStatus(tc.cniDaemonSetName, appsv1.DaemonSetStatus{
									NumberAvailable:   0,
									NumberUnavailable: 3,
								})),
						},
						Timeout: 10 * time.Second,
					},
				},
			})
		})
	}
}

func unexpectedCNIDaemonSetNames(cniDaemonSetNames map[versions.Version]string, currentVer versions.Version) []ActionAssertion {
	var unexpectedCNIDaemonSets []ActionAssertion
	for ver, daemonSetName := range cniDaemonSetNames {
		if ver != currentVer {
			unexpectedCNIDaemonSets = append(unexpectedCNIDaemonSets, Assert("create").On("daemonsets").Named(daemonSetName).IsNotSeen())
			unexpectedCNIDaemonSets = append(unexpectedCNIDaemonSets, Assert("update").On("daemonsets").Named(daemonSetName).IsNotSeen())
		}
	}
	return unexpectedCNIDaemonSets
}

func SetDaemonSetStatus(name string, status appsv1.DaemonSetStatus) ReactionFunc {
	return func(action clienttesting.Action, tracker clienttesting.ObjectTracker) (applied bool, handled bool, obj runtime.Object, err error) {
		applied = false
		handled = false
		if obj, err = tracker.Get(action.GetResource(), action.GetNamespace(), name); err != nil {
			return
		}
		applied = true
		daemonSet, ok := obj.(*appsv1.DaemonSet)
		if !ok {
			err = fmt.Errorf("object is not an appsv1.DaemonSet: %v", obj)
			return
		}
		status.DeepCopyInto(&daemonSet.Status)
		// update the status before returning (simulate a node becoming available)
		err = tracker.Update(action.GetResource(), daemonSet, action.GetNamespace())
		return
	}
}

func FinalizerAddedTest(finalizer string) VerifierTestFunc {
	return func(action clienttesting.Action) error {
		switch realAction := action.(type) {
		case clienttesting.UpdateAction:
			obj := realAction.GetObject()
			metaObj, err := meta.Accessor(obj)
			if err != nil {
				return errors.Wrapf(err, "finalizerAddedTest for %s failed: could not convert resource to metav1.Object", finalizer)
			}
			if sets.NewString(metaObj.GetFinalizers()...).Has(finalizer) {
				return nil
			}
			return fmt.Errorf("finalizerAddedTest failed: object %s/%s is missing finalizer %s", metaObj.GetNamespace(), metaObj.GetName(), finalizer)
		}
		return fmt.Errorf("finalizerAddedTest for %s failed: action is not an UpdateAction", finalizer)
	}
}

type jsonPatchOperation struct {
	Operation string                       `json:"op"`
	Path      string                       `json:"path"`
	Value     maistrav1.ControlPlaneStatus `json:"value,omitempty"`
}

func initalStatusTest(action clienttesting.Action) error {
	switch realAction := action.(type) {
	case clienttesting.PatchAction:
		patchAction := action.(clienttesting.PatchAction)
		switch patchAction.GetPatchType() {
		case types.JSONPatchType:
			jsonPatch := []jsonPatchOperation{}
			err := json.Unmarshal(realAction.GetPatch(), &jsonPatch)
			if err != nil {
				return fmt.Errorf("initialStatusTest failed: could not unmarshal patch data: %v", string(realAction.GetPatch()))
			}
			return validateNewStatus(&jsonPatch[0].Value)
		default:
			cp := &maistrav1.ServiceMeshControlPlane{}
			err := json.Unmarshal(realAction.GetPatch(), cp)
			if err != nil {
				return fmt.Errorf("initialStatusTest failed: could not unmarshal patch data: %v", string(realAction.GetPatch()))
			}
			return validateNewStatus(cp.Status.DeepCopy())
		}
	}
	return fmt.Errorf("initialStatusTest for failed: action is not a PatchAction")
}

func validateNewStatus(actual *maistrav1.ControlPlaneStatus) error {
	actual.LastAppliedConfiguration = maistrav1.ControlPlaneSpec{}
	for index := range actual.Conditions {
		actual.Conditions[index].LastTransitionTime = metav1.Time{}
	}
	expected := &maistrav1.ControlPlaneStatus{
		StatusBase: status.StatusBase{
			Annotations: map[string]string(nil),
		},
		StatusType: status.StatusType{
			Conditions: []status.Condition{
				{
					Type:               status.ConditionTypeInstalled,
					Status:             status.ConditionStatusFalse,
					Reason:             status.ConditionReasonResourceCreated,
					Message:            "Installing mesh generation 1",
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               status.ConditionTypeReconciled,
					Status:             status.ConditionStatusFalse,
					Reason:             status.ConditionReasonResourceCreated,
					Message:            "Installing mesh generation 1",
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               status.ConditionTypeReady,
					Status:             status.ConditionStatusFalse,
					Reason:             status.ConditionReasonResourceCreated,
					Message:            "Installing mesh generation 1",
					LastTransitionTime: metav1.Time{},
				},
			},
		},
		ObservedGeneration: 0,
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("initialStatusTest failed: updated status does not match expected status:\n\texpected: %#v\n\tactual: %#v", actual, expected)
	}
	return nil
}
