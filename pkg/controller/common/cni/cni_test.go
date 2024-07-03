package cni

import (
	"sync"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestInitConfig_disablingCNI(t *testing.T) {
	originalCniValue := common.Config.OLM.CNIEnabled
	common.Config.OLM.CNIEnabled = false

	// reset the global vars in cni.go so that GetConfig will actually run in this test
	resetCNIConfig()

	var m manager.Manager
	config := GetConfig(m)

	assert.Equals(config.Enabled, false, "", t)

	// Quick test cleanup
	common.Config.OLM.CNIEnabled = originalCniValue

	// reset the global vars in cni.go so that GetConfig will run again when called by other tests
	resetCNIConfig()
	once = sync.Once{}
}

func resetCNIConfig() {
	once = sync.Once{}
	config = Config{}
}

func TestIsCNIConfigEnabledByDefault(t *testing.T) {
	assert.Equals(common.Config.OLM.CNIEnabled, true, "", t)
}
