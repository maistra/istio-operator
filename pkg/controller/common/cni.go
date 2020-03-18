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

	// ImageV1_0 is the full image name that should be deployed through the Istio
	// CNI DaemonSet for v1.0
	ImageV1_0 string

	// ImageV1_1 is the full image name that should be deployed through the Istio
	// CNI DaemonSet for v1.1
	ImageV1_1 string

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

		var ok bool
		if config.ImageV1_0, ok = os.LookupEnv("ISTIO_CNI_IMAGE_V1_0"); !ok {
			return config, fmt.Errorf("ISTIO_CNI_IMAGE_V1_0 environment variable not set")
		}
		if config.ImageV1_1, ok = os.LookupEnv("ISTIO_CNI_IMAGE_V1_1"); !ok {
			return config, fmt.Errorf("ISTIO_CNI_IMAGE_V1_1 environment variable not set")
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
