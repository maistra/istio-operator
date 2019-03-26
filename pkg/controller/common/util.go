package common

import (
	"github.com/go-logr/logr"

    "sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceManager struct {
	Client client.Client
	Log    logr.Logger
}

func IndexOf(l []string, s string) int {
	for i, elem := range l {
		if elem == s {
			return i
		}
	}
	return -1
}
