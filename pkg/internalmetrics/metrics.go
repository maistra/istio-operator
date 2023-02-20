package internalmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// These constants define the different label names for the different metric timeseries
const (
	labelVersion      = "version"
	labelNamespace    = "namespace"
	labelSMCPVersion  = "smcp_version"
	labelSMCPTopology = "smcp_topology"
)

// MetricsType defines all of istio-operator's metrics.
type MetricsType struct {
	SMCPCounter *prometheus.CounterVec
}

// Metrics contains all of istio-operator's own internal metrics.
// These metrics can be accessed directly to update their values, or
// you can use available utility functions defined below.
var Metrics = MetricsType{
	SMCPCounter: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "servicemesh_member_count",
			Help: "Counts the total number of Service Mesh Control Plane Namespaces.",
		},
		[]string{labelSMCPVersion, labelSMCPTopology},
	),
}

// RegisterMetrics must be called at startup to prepare the Prometheus scrape endpoint.
func RegisterMetrics() {
	metrics.Registry.MustRegister(
		Metrics.SMCPCounter,
	)
}

func GetSMCPCount(smcpVersion, smcpTopology string) prometheus.Counter {
	return Metrics.SMCPCounter.With(prometheus.Labels{
		labelSMCPVersion:  smcpVersion,
		labelSMCPTopology: smcpTopology,
	})
}
