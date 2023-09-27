package strategy

import v1 "maistra.io/istio-operator/api/v1alpha1"

type VersionStrategy interface {
	ApplyDefaults(istio *v1.Istio) error
}
