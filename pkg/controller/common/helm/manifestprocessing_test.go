package helm

import (
	"context"
	"testing"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/releaseutil"
)

func TestEmptyYAMLBlocks(t *testing.T) {
	manifest := manifest.Manifest{
		Name: "bad.yaml",
		Content: `
---

# comment section

--- 

  ---
`,
		Head: &releaseutil.SimpleHead{},
	}

	processor := NewManifestProcessor(common.ControllerResources{}, &PatchFactory{}, "app", "version", "owner", nil, nil)

	err := processor.ProcessManifest(context.TODO(), manifest, "bad")

    if len(err) > 0 {
        t.Errorf("expected empty yaml blocks to process without error")
    }
}
