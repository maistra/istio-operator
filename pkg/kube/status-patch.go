// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"encoding/json"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusPatch struct {
	status interface{}
}

func NewStatusPatch(status interface{}) *StatusPatch {
	return &StatusPatch{
		status: status,
	}
}

func (p *StatusPatch) Type() types.PatchType {
	return types.JSONPatchType
}

func (p *StatusPatch) Data(obj client.Object) ([]byte, error) {
	data := []jsonpatch.Operation{
		{
			Operation: "replace",
			Path:      "/status",
			Value:     p.status,
		},
	}
	statusJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return statusJSON, nil
}
