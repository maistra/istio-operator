package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
)

func TestBootstrapping(t *testing.T) {
	const (
		operatorNamespace     = "istio-operator"
		controlPlaneNamespace = "test"
		smcpName              = "test"
		cniDaemonSetName      = "istio-node"
	)

	if testing.Verbose() {
		logf.SetLogger(logf.ZapLogger(true))
	}
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
		Events: []ControllerTestEvent{
			{
				Name: "bootstrap-clean-install-cni-no-errors",
				Execute: func(mgr *FakeManager, _ *EnhancedTracker) error {
					return mgr.GetClient().Create(context.TODO(), &maistrav1.ServiceMeshControlPlane{
						ObjectMeta: metav1.ObjectMeta{Name: smcpName, Namespace: controlPlaneNamespace},
						Spec: maistrav1.ControlPlaneSpec{
							Version:  "v1.1",
							Template: "maistra",
						},
					})
				},
				Verifier: VerifyActions(
					// add finalizer
					Verify("update").On("servicemeshcontrolplanes").Named(smcpName).In(controlPlaneNamespace).Passes(FinalizerAddedTest(common.FinalizerName)),
					// initialize status
					Verify("patch").On("servicemeshcontrolplanes/status").Named(smcpName).In(controlPlaneNamespace).Passes(initalStatusTest),
					// verify that a CRD is installed
					Verify("create").On("customresourcedefinitions").IsSeen(),
					// verify that CNI is installed
					Verify("create").On("daemonsets").Named(cniDaemonSetName).In(operatorNamespace).IsSeen(),
					// verify CNI readiness check during reconcile
					Verify("list").On("daemonsets").In(operatorNamespace).IsSeen(),
					// verify readiness check triggered daemon set creation
					VerifyReadinessCheckOccurs(controlPlaneNamespace, operatorNamespace),
				),
				Assertions: ActionAssertions{
					// verify proper number of CRDs is created
					Assert("create").On("customresourcedefinitions").SeenCountIs(28),
				},
				Reactors: []clienttesting.Reactor{
					ReactTo("list").On("daemonsets").In(operatorNamespace).With(
						SetDaemonSetStatus(cniDaemonSetName, appsv1.DaemonSetStatus{
							NumberAvailable:   0,
							NumberUnavailable: 3,
						})),
				},
				Timeout: 10 * time.Second,
			},
		},
	})
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

func FinalizerAddedTest(finalizer string) test.VerifierTestFunc {
	return func(action clienttesting.Action) error {
		switch realAction := action.(type) {
		case clienttesting.UpdateAction:
			obj := realAction.GetObject()
			metaObj, err := meta.Accessor(obj)
			if err != nil {
				return errors.Wrapf(err, "FinalizerAddedTest for %s failed: could not convert resource to metav1.Object", finalizer)
			}
			if sets.NewString(metaObj.GetFinalizers()...).Has(finalizer) {
				return nil
			}
			return fmt.Errorf("FinalizerAddedTest failed: object %s/%s is missing finalizer %s", metaObj.GetNamespace(), metaObj.GetName(), finalizer)
		}
		return fmt.Errorf("FinalizerAddedTest for %s failed: action is not an UpdateAction", finalizer)
	}
}

func initalStatusTest(action clienttesting.Action) error {
	switch realAction := action.(type) {
	case clienttesting.PatchAction:
		cp := &maistrav1.ServiceMeshControlPlane{}
		err := json.Unmarshal(realAction.GetPatch(), cp)
		if err != nil {
			return fmt.Errorf("InitialStatusTest failed: could not unmarshal patch data")
		}
		actual := cp.Status.DeepCopy()
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
			return fmt.Errorf("InitialStatusTest failed: updated status does not match expected status:\n\texpected: %#v\n\tactual: %#v", actual, expected)
		}
		return nil
	}
	return fmt.Errorf("InitialStatusTest for failed: action is not a PatchAction")
}
