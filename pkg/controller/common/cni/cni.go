package cni

import (
	"fmt"
	"os"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

var (
	once   sync.Once
	config Config
)

type Config struct {
	// Enabled tells whether this cluster supports CNI or not
	Enabled bool

	// UseMultus specifies whether the Istio CNI plugin should be called via Multus CNI
	UseMultus bool

	// ImagePullSecrets is the list of image pull secret names for the Istio CNI DaemonSet
	ImagePullSecrets []string
}

// GetConfig initializes the CNI support variable
func GetConfig(m manager.Manager) Config {
	once.Do(func() {
		log := logf.Log.WithName("controller_init")

		if !common.Config.OLM.CNIEnabled {
			config.Enabled = false
			log.Info(fmt.Sprintf("CNI is disabled for this installation: %v", config.Enabled))
			return
		}
		log.Info(fmt.Sprintf("CNI is enabled for this installation: %v", config.Enabled))

		config.Enabled = true

		_, err := m.GetRESTMapper().ResourcesFor(schema.GroupVersionResource{
			Group:    "k8s.cni.cncf.io",
			Version:  "v1",
			Resource: "network-attachment-definitions",
		})

		if err == nil {
			config.UseMultus = true

			secret, _ := os.LookupEnv("ISTIO_CNI_IMAGE_PULL_SECRET")
			if secret != "" {
				config.ImagePullSecrets = append(config.ImagePullSecrets, secret)
			}

		} else if !meta.IsNoMatchError(err) {
			config.UseMultus = false
		}
	})
	return config
}
