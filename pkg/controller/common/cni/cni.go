package cni

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

type Config struct {
	// Enabled tells whether this cluster supports CNI or not
	Enabled bool

	// ImagePullSecrets is the list of image pull secret names for the Istio CNI DaemonSet
	ImagePullSecrets []string
}

// InitConfig initializes the CNI support variable
func InitConfig(m manager.Manager) (Config, error) {
	config := Config{}

	if !common.Config.OLM.CNIEnabled {
		config.Enabled = false
		return config, nil
	}
	_, err := m.GetRESTMapper().ResourcesFor(schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	})

	if err == nil {
		config.Enabled = true

		if len(common.Config.OLM.Images.V1_0.CNI) == 0 {
			return config, fmt.Errorf("configuration olm.relatedImage.v1_0.cni must be set")
		}
		if len(common.Config.OLM.Images.V1_1.CNI) == 0 {
			return config, fmt.Errorf("configuration olm.relatedImage.v1_1.cni must be set")
		}
		if len(common.Config.OLM.Images.V2_0.CNI) == 0 {
			return config, fmt.Errorf("configuration olm.relatedImage.v2_0.cni must be set")
		}
		if len(common.Config.OLM.Images.V2_1.CNI) == 0 {
			return config, fmt.Errorf("configuration olm.relatedImage.v2_1.cni must be set")
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
