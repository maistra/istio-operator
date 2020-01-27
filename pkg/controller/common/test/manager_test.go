package test

import (
	"testing"
	"time"

	"github.com/maistra/istio-operator/pkg/apis"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var voidReconciler = reconcile.Func(func(request reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
})

// test fake manager
func TestFakeManager(t *testing.T) {
	// setup manager
	scheme := runtime.NewScheme()
	err := apis.AddToScheme(scheme)
	if err != nil {
		t.Errorf("Unexpected error creating scheme: %v", err)
	}
	codecs := serializer.NewCodecFactory(scheme)
	tracker := NewEnhancedTracker(clienttesting.NewObjectTracker(scheme, codecs.UniversalDecoder()), scheme)
	mgr, err := NewManager(scheme, tracker)
	if err != nil {
		t.Errorf("Unexpected error creating manager: %v", err)
	}

	// channel that records invocations on the reconciler
	invocationChannel := make(chan ReconcilerInvocation)

	// create test controller
	c, err := controller.New("test-controller", mgr, controller.Options{Reconciler: WrapReconciler(voidReconciler, invocationChannel)})
	if err != nil {
		t.Errorf("Unexpected error creating controller: %v", err)
	}

	// create watch
	if err := c.Watch(&source.Kind{Type: &maistrav1.ServiceMeshControlPlane{}}, &handler.EnqueueRequestForObject{}); err != nil {
		t.Errorf("Unexpected error creating watch: %v", err)
	}

	// start the manager.  at this point, any resources added to the tracker could trigger a reconcile, if the type is being watched
	stop := StartManager(mgr, t)
	defer stop()

	// create a resource.  this should force an event through the controller
	testName := types.NamespacedName{}
	tracker.Add(&maistrav1.ServiceMeshControlPlane{ObjectMeta: metav1.ObjectMeta{Name: testName.Name, Namespace: testName.Namespace}})

	// wait for an invocation
	select {
	case invocation := <-invocationChannel:
		if invocation.Error != nil {
			t.Errorf("Unexpected error reconciling resource: %v", invocation.Error)
		} else if invocation.NamespacedName.String() != testName.String() {
			t.Errorf("Failed to see expected resource: %s", testName.String())
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Timout waiting for Reconcile()")
	}

	// create another resource.  this type isn't watched, so no event should go through the controller
	tracker.Add(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: testName.Name, Namespace: testName.Namespace}})

	// wait for an invocation
	select {
	case <-invocationChannel:
		t.Errorf("Unexpected reconcile: added resource was not being watched")
	case <-time.After(1 * time.Second):
		t.Logf("Reconcile not invoked after 1s")
	}

	// create new watch.  the watch should be started, as the manager is already started
	if err := c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForObject{}); err != nil {
		t.Errorf("Unexpected error creating watch: %v", err)
	}

	// create another resource.  this type is now being watched, so an event should go through the controller
	tracker.Add(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: testName.Name, Namespace: testName.Namespace}})

	// wait for an invocation
	select {
	case invocation := <-invocationChannel:
		if invocation.Error != nil {
			t.Errorf("Unexpected error reconciling resource: %v", invocation.Error)
		} else if invocation.NamespacedName.String() != testName.String() {
			t.Errorf("Failed to see expected resource: %s", testName.String())
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Timout waiting for Reconcile()")
	}
}
