package controller

import (
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhookca"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, webhookca.Add)
}
