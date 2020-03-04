package common

import (
	"testing"

	"github.com/maistra/istio-operator/pkg/apis/maistra"
)

func TestNetworkNameMap(t *testing.T) {
    for _, v := range maistra.GetSupportedVersions() {
        if _, ok := networkNameMap[v]; !ok {
            t.Errorf("missing network name for control plane version %s", v.String())
        }
    }
}
