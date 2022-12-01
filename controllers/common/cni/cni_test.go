package cni

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/maistra/istio-operator/controllers/common"
	"github.com/maistra/istio-operator/controllers/common/test/assert"
)

func TestInitConfig_disablingCNI(t *testing.T) {
	operatorNamespace := "istio-operator"
	originalCniValue := InitializeGlobals(operatorNamespace)

	var m manager.Manager
	config, err := InitConfig(m)

	assert.Equals(err, nil, "", t)
	assert.Equals(config.Enabled, false, "", t)

	// Quick test cleanup
	common.Config.OLM.CNIEnabled = originalCniValue
}

func InitializeGlobals(operatorNamespace string) (originalValue bool) {
	originalValue = common.Config.OLM.CNIEnabled
	common.Config.OLM.CNIEnabled = false
	return originalValue
}

func TestIsCNIConfigEnabledByDefault(t *testing.T) {
	assert.Equals(common.Config.OLM.CNIEnabled, true, "", t)
}
