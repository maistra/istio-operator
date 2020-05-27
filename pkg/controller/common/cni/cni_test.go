package cni

import (
	"testing"

	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func TestNetworkNameMap(t *testing.T) {
    for _, v := range versions.GetSupportedVersions() {
        if _, ok := networkNameMap[v]; !ok {
            t.Errorf("missing network name for control plane version %s", v.String())
        }
    }
}
