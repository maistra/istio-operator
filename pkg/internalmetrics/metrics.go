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
	MemberCounter *prometheus.GaugeVec
}

// Metrics contains all of istio-operator's own internal metrics.
// These metrics can be accessed directly to update their values, or
// you can use available utility functions defined below.
var Metrics = MetricsType{
	MemberCounter: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "servicemesh_member_count",
			Help: "Counts the total number of Service Mesh Control Plane Namespaces.",
		},
		[]string{labelSMCPNamespace, labelSMCPVersion, labelSMCPMode},
	),
}

// RegisterMetrics must be called at startup to prepare the Prometheus scrape endpoint.
func RegisterMetrics() {
	metrics.Registry.MustRegister(
		Metrics.MemberCounter,
	)
}

func GetMemberCounter(smcpNamespace, smcpVersion, smcpMode string) prometheus.Gauge {
	return Metrics.MemberCounter.With(prometheus.Labels{
		labelSMCPNamespace: smcpNamespace,
		labelSMCPVersion:   smcpVersion,
		labelSMCPMode:      smcpMode,
	})
}

func ResetMemberCounter() {
	Metrics.MemberCounter.Reset()
}
