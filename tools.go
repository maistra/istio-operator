//go:build tools
// +build tools

package tools

import (
	_ "github.com/mikefarah/yq/v4"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
