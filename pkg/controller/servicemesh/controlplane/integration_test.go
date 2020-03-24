package controlplane

import (
	"context"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
)

func TestBootstrapping(t *testing.T) {
	t.Skip("https://issues.redhat.com/browse/MAISTRA-1224")
	const (
		operatorNamespace     = "istio-operator"
		controlPlaneNamespace = "test"
		smcpName              = "test"
		cniDaemonSetName      = "istio-node"
	)

	if testing.Verbose() {
		logf.SetLogger(logf.ZapLogger(true))
	}
	RunControllerTestCases(t, ControllerTestCase{
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
			ControllerTestEvent{
				Name: "bootstrap-clean-install-cni-no-errors",
				Execute: func(mgr *FakeManager, _ *EnhancedTracker) error {
					return mgr.GetClient().Create(context.TODO(), &maistrav1.ServiceMeshControlPlane{
						ObjectMeta: metav1.ObjectMeta{Name: smcpName, Namespace: controlPlaneNamespace},
						Spec: maistrav1.ControlPlaneSpec{
							Version: "v1.1",
							Template: "maistra",
						},
					})
				},
				Verifier: &VerifyActions{
					// add finalizer
					Verify("update").On("servicemeshcontrolplanes").Named(smcpName).In(controlPlaneNamespace).IsSeen(),
					// initialize status
					Verify("update").On("servicemeshcontrolplanes/status").Named(smcpName).In(controlPlaneNamespace).IsSeen(),
					// verify that a CRD is installed
					Verify("create").On("customresourcedefinitions").IsSeen(),
					// verify that CNI is installed
					Verify("create").On("daemonsets").Named(cniDaemonSetName).In(operatorNamespace).IsSeen(),
					// verify CNI readiness check during reconcile
					Verify("list").On("daemonsets").In(operatorNamespace).IsSeen(),
					// verify readiness check triggered daemon set creation
					VerifyReadinessCheckOccurs(controlPlaneNamespace, operatorNamespace),
				},
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
				Timeout: 5 * time.Second,
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
