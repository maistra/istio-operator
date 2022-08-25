package memberroll

import (
	"context"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

var ctx = common.NewContextWithLog(context.Background(), logf.Log)
