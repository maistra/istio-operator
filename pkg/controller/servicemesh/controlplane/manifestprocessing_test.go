package controlplane

import (
	"testing"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/releaseutil"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestReadinessWhenCacheNotSynced(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Spec.Istio = maistrav1.HelmValuesType{}
	controlPlane.Spec.Template = "maistra"
	controlPlane.Status.ComponentStatus = []maistrav1.ComponentStatus{
		{
			StatusType: maistrav1.StatusType{
				Resource:           "security",
				ObservedGeneration: 0,
				Conditions:         nil,
			},
		},
	}

	operatorNamespace := "istio-operator"
	InitializeGlobals(operatorNamespace)()

	cl, tracker := test.CreateClient()
	fakeEventRecorder := &record.FakeRecorder{}

	instanceReconciler := NewControlPlaneInstanceReconciler(
		common.ControllerResources{
			Client:            cl,
			Scheme:            scheme.Scheme,
			EventRecorder:     fakeEventRecorder,
			OperatorNamespace: operatorNamespace,
		},
		controlPlane,
		cni.Config{Enabled: true}).(*controlPlaneInstanceReconciler)

	// emulate cache desync (deployment not yet in cache)
	tracker.AddReactor("list", "deployments", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1.DeploymentList{}, nil
	})

	instanceReconciler.renderings = map[string][]manifest.Manifest{
		"security": {
			{
				Name: "deployment.yaml",
				Content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: cp-namespace
spec:`,
				Head: &releaseutil.SimpleHead{
					Version: "apps/v1",
					Kind:    "Deployment",
				},
			},
		},
	}

	hasReadiness, err := instanceReconciler.processComponentManifests(ctx, "security")
	if err != nil {
		t.Fatalf("Unexpected error in processComponentManifests: %v", err)
	}

	assert.True(hasReadiness, "expected component to have readiness", t)

	_, unreadyComponents, err := instanceReconciler.calculateComponentReadiness(ctx)
	if err != nil {
		t.Fatalf("Unexpected error in calculateComponentReadiness: %v", err)
	}

	assert.False(unreadyComponents.Has("security"), "expected component to not be ready", t)
}
