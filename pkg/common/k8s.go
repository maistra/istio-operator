package common

import (
	"os"
	"sync"
)

var (
	initOperatorNamespace sync.Once
	operatorNamespace     string
)

// GetOperatorNamespace initializes and caches this operator's namespace
func GetOperatorNamespace() string {
	initOperatorNamespace.Do(func() {
		operatorNamespace = os.Getenv("POD_NAMESPACE")
	})
	return operatorNamespace
}
