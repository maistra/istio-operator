package controller

import (
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/controlplane"
	"github.com/maistra/istio-operator/pkg/controller/legacy"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, controlplane.Add, legacy.Add)
}
