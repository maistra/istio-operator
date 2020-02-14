package controlplane

import (
	"os"
	"path"
	goruntime "runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/restmapper"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
)

// InitializeGlobals returns a function which initializes global variables used
// by the system under test.  operatorNamespace is the namespace within which
// the operator is installed.
func InitializeGlobals(operatorNamespace string) func() {
	return func() {
		// make sure globals are initialized for testing
		os.Setenv("ISTIO_CNI_IMAGE_V1_0", "istio-cni-test-1_0")
		os.Setenv("ISTIO_CNI_IMAGE_V1_1", "istio-cni-test-1_1")
		os.Setenv("POD_NAMESPACE", operatorNamespace)
		common.GetOperatorNamespace()
		if _, filename, _, ok := goruntime.Caller(0); ok {
			common.Options.ResourceDir = path.Join(path.Dir(filename), "../../../../resources")
			common.Options.ChartsDir = path.Join(common.Options.ResourceDir, "helm")
			common.Options.DefaultTemplatesDir = path.Join(common.Options.ResourceDir, "smcp-templates")
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
			metav1.GroupVersionForDiscovery{Version: "v1"},
		},
	},
	VersionedResources: map[string][]metav1.APIResource{
		"v1": []metav1.APIResource{
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
	return &test.VerifyActions{
		test.Verify("list").On("deployments").In(controlPlaneNamespace).IsSeen(),
		test.Verify("list").On("statefulsets").In(controlPlaneNamespace).IsSeen(),
		test.Verify("list").On("daemonsets").In(controlPlaneNamespace).IsSeen(),
		test.Verify("list").On("daemonsets").In(operatorNamespace).IsSeen(),
	}
}
