package common

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// IsCNIEnabled tells whether this cluster supports CNI or not
var IsCNIEnabled bool

// InitCNIStatus initializes the CNI support variable
func InitCNIStatus(m manager.Manager) error {
	_, err := m.GetRESTMapper().ResourcesFor(schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	})

	if err == nil {
		IsCNIEnabled = true
	} else if !meta.IsNoMatchError(err) {
		return err
	}

	log := logf.Log.WithName("controller_init")
	log.Info(fmt.Sprintf("CNI is enabled for this installation: %v", IsCNIEnabled))

	return nil
}
