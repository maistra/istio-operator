package versions

import (
	"testing"
)

func TestNetworkNameMap(t *testing.T) {
    for _, v := range GetSupportedVersions() {
        if v.GetCNINetworkName() == "" {
            t.Errorf("missing network name for control plane version %s", v.String())
        }
    }
}
