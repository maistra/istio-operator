package common

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// IsCNIEnabled tells whether this cluster supports CNI or not
var IsCNIEnabled bool

// CNIImage is the full image name that should be deployed through the Istio CNI DaemonSet
var CNIImageV1_0 string
var CNIImageV1_1 string

// CNIImagePullSecrets is the list of image pull secret names for the Istio CNI DaemonSet
var CNIImagePullSecrets []string

// networkNameMap is a map of the CNI network name used by each version
var networkNameMap = map[string]string{
	"":     "istio-cni",
	"v1.0": "istio-cni",
	"v1.1": "v1-1-istio-cni",
}

var supportedVersions []string

func init() {
	for key := range networkNameMap {
		if len(key) > 0 {
			supportedVersions = append(supportedVersions, key)
		}
	}
}

// GetCNINetworkName returns the name of the CNI network used to configure routing rules for the mesh
func GetCNINetworkName(maistraVersion string) (name string, ok bool) {
	name, ok = networkNameMap[maistraVersion]
	return
}

// GetSupportedVersions returns a list of versions supported by this operator
func GetSupportedVersions() []string {
	return supportedVersions
}

// InitCNIStatus initializes the CNI support variable
func InitCNIStatus(m manager.Manager) error {
	_, err := m.GetRESTMapper().ResourcesFor(schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	})

	if err == nil {
		IsCNIEnabled = true

		var ok bool
		if CNIImageV1_0, ok = os.LookupEnv("ISTIO_CNI_IMAGE_V1_0"); !ok {
			return fmt.Errorf("ISTIO_CNI_IMAGE_V1_0 environment variable not set")
		}
		if CNIImageV1_1, ok = os.LookupEnv("ISTIO_CNI_IMAGE_V1_1"); !ok {
			return fmt.Errorf("ISTIO_CNI_IMAGE_V1_1 environment variable not set")
		}

		secret, _ := os.LookupEnv("ISTIO_CNI_IMAGE_PULL_SECRET")
		if secret != "" {
			CNIImagePullSecrets = append(CNIImagePullSecrets, secret)
		}

	} else if !meta.IsNoMatchError(err) {
		return err
	}

	log := logf.Log.WithName("controller_init")
	log.Info(fmt.Sprintf("CNI is enabled for this installation: %v", IsCNIEnabled))

	return nil
}
