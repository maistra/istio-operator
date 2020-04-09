package common

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/apis/maistra"
)

type CNIConfig struct {
	// Enabled tells whether this cluster supports CNI or not
	Enabled bool

	// ImagePullSecrets is the list of image pull secret names for the Istio CNI DaemonSet
	ImagePullSecrets []string
}

// networkNameMap is a map of the CNI network name used by each version
var networkNameMap = map[maistra.Version]string{
	maistra.UndefinedVersion: "istio-cni",
	maistra.V1_0:             "istio-cni",
	maistra.V1_1:             "v1-1-istio-cni",
}

// GetCNINetworkName returns the name of the CNI network used to configure routing rules for the mesh
func GetCNINetworkName(maistraVersion string) (name string, ok bool) {
	if v, err := maistra.ParseVersion(maistraVersion); err == nil {
		name, ok = networkNameMap[v.Version()]
	}
	return
}

// InitCNIConfig initializes the CNI support variable
func InitCNIConfig(m manager.Manager) (CNIConfig, error) {
	config := CNIConfig{}

	_, err := m.GetRESTMapper().ResourcesFor(schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	})

	if err == nil {
		config.Enabled = true

		if len(Config.OLM.Images.V1_0.CNI) == 0 {
			return config, fmt.Errorf("configuration olm.relatedImage.v1_0.cni must be set")
		}
		if len(Config.OLM.Images.V1_1.CNI) == 0 {
			return config, fmt.Errorf("configuration olm.relatedImage.v1_1.cni must be set")
		}

		secret, _ := os.LookupEnv("ISTIO_CNI_IMAGE_PULL_SECRET")
		if secret != "" {
			config.ImagePullSecrets = append(config.ImagePullSecrets, secret)
		}

	} else if !meta.IsNoMatchError(err) {
		return config, err
	}

	log := logf.Log.WithName("controller_init")
	log.Info(fmt.Sprintf("CNI is enabled for this installation: %v", config.Enabled))

	return config, nil
}
