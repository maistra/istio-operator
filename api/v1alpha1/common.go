package v1alpha1

import (
	"encoding/json"
	"time"

	"maistra.io/istio-operator/pkg/helm"
)

// testTime is only in unit tests to pin the time to a fixed value
var testTime *time.Time

func toHelmValues(rawMessage json.RawMessage) helm.HelmValues {
	var vals helm.HelmValues
	err := json.Unmarshal(rawMessage, &vals)
	if err != nil {
		return nil
	}
	return vals
}
