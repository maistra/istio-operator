package internalmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	ControlPlaneModeValueClusterScoped = "ClusterWide"
	ControlPlaneModeValueMultiTenant   = "MultiTenant"
)

// These constants define the different label names for the different metric timeseries
const (
	labelSMCPNamespace = "smcp_namespace"
	labelSMCPVersion   = "smcp_version"
	labelSMCPMode      = "smcp_mode"
)

// MetricsType defines all of istio-operator's metrics.
type MetricsType struct {
	MeshMembers *prometheus.GaugeVec
}

// Metrics contains all of istio-operator's own internal metrics.
// These metrics can be accessed directly to update their values, or
// you can use available utility functions defined below.
var Metrics = MetricsType{
	MeshMembers: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "servicemesh_members",
			Help: "Number of SMCP members per namespace, mesh mode and version.",
		},
		[]string{labelSMCPNamespace, labelSMCPVersion, labelSMCPMode},
	),
}

// RegisterMetrics must be called at startup to prepare the Prometheus scrape endpoint.
func RegisterMetrics() {
	metrics.Registry.MustRegister(
		Metrics.MeshMembers,
	)
}

func GetMeshMembers(smcpNamespace, smcpVersion, smcpMode string) prometheus.Gauge {
	return Metrics.MeshMembers.With(prometheus.Labels{
		labelSMCPNamespace: smcpNamespace,
		labelSMCPVersion:   smcpVersion,
		labelSMCPMode:      smcpMode,
	})
}

func DeleteMeshMembersWithLabelsValues(smcpNamespace, smcpVersion, smcpMode string) bool {
	return Metrics.MeshMembers.Delete(prometheus.Labels{
		labelSMCPNamespace: smcpNamespace,
		labelSMCPVersion:   smcpVersion,
		labelSMCPMode:      smcpMode,
	})
}
