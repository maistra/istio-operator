package cni

import (
	"testing"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestInitConfig_disablingCNI(t *testing.T) {
	operatorNamespace := "istio-operator"
	InitializeGlobals(operatorNamespace)()
	var m manager.Manager
	config, err := InitConfig(m)
	assert.Equals(err, nil, "", t)
	assert.Equals(config.Enabled, false, "", t)

	// Quick test cleanup
	common.Config.OLM.CNIEnabled = true
}

func InitializeGlobals(operatorNamespace string) func() {
	return func() {
		// make sure globals are initialized for testing
		common.Config.OLM.CNIEnabled = false
	}
}

func TestIsCNIConfigEnabledByDefault(t *testing.T) {
	assert.Equals(common.Config.OLM.CNIEnabled, true, "", t)
}
