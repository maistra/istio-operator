//go:build tools
// +build tools

package tools

import (
	_ "github.com/mikefarah/yq/v4"
	_ "github.com/operator-framework/operator-sdk/cmd/operator-sdk"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
