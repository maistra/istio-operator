package controller

import (
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/validation"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, validation.Add)
}
