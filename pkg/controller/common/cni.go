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
var CNIImage string

// CNIImagePullSecrets is the list of image pull secret names for the Istio CNI DaemonSet
var CNIImagePullSecrets []string

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
		if CNIImage, ok = os.LookupEnv("ISTIO_CNI_IMAGE"); !ok {
			return fmt.Errorf("ISTIO_CNI_IMAGE environment variable not set")
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
