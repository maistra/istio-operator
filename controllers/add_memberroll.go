package controller

import (
	"github.com/maistra/istio-operator/controllers/servicemesh/memberroll"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, memberroll.Add)
}
