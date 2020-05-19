package common

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type statusPatch struct {
	status interface{}
}

func NewStatusPatch(status interface{}) *statusPatch {
	return &statusPatch{
		status: status,
	}
}

func (p *statusPatch) Type() types.PatchType {
	return types.MergePatchType
}

func (p *statusPatch) Data(obj runtime.Object) ([]byte, error) {
	data := map[string]interface{}{
		"status": p.status,
	}
	statusJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return statusJSON, nil
}
