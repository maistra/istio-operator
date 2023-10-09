package versions

import (
	"fmt"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func validatePrometheusEnabledWhenDefaultKialiEnabled(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.IsKialiEnabled() && !spec.IsCustomKialiConfigured() && !spec.IsPrometheusEnabled() {
		return append(allErrors, fmt.Errorf(".spec.addons.prometheus.enabled must be true when "+
			".spec.addons.kiali.enabled is true and spec.addons.kiali.name is not specified"))
	}
	return allErrors
}
