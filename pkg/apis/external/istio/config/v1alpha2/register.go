package v1alpha2

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// SchemeGroupVersion is group version used to register istio config objects
	SchemeGroupVersion = schema.GroupVersion{Group: "config.istio.io", Version: "v1alpha2"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

func init() {
	SchemeBuilder.SchemeBuilder.Register(
		registerTypeForKind("bypass", &Bypass{}, &BypassList{}),
		registerTypeForKind("circonus", &Circonus{}, &CirconusList{}),
		registerTypeForKind("denier", &Denier{}, &DenierList{}),
		registerTypeForKind("fluentd", &Fluentd{}, &FluentdList{}),
		registerTypeForKind("kubernetesenv", &Kubernetesenv{}, &KubernetesenvList{}),
		registerTypeForKind("listchecker", &Listchecker{}, &ListcheckerList{}),
		registerTypeForKind("memquota", &Memquota{}, &MemquotaList{}),
		registerTypeForKind("noop", &Noop{}, &NoopList{}),
		registerTypeForKind("opa", &Opa{}, &OpaList{}),
		registerTypeForKind("prometheus", &Prometheus{}, &PrometheusList{}),
		registerTypeForKind("rbac", &Rbac{}, &RbacList{}),
		registerTypeForKind("redisquota", &Redisquota{}, &RedisquotaList{}),
		registerTypeForKind("signalfx", &Signalfx{}, &SignalfxList{}),
		registerTypeForKind("solarwinds", &Solarwinds{}, &SolarwindsList{}),
		registerTypeForKind("stackdriver", &Stackdriver{}, &StackdriverList{}),
		registerTypeForKind("statsd", &Statsd{}, &StatsdList{}),
		registerTypeForKind("stdio", &Stdio{}, &StdioList{}),
		registerTypeForKind("apikey", &Apikey{}, &ApikeyList{}),
		registerTypeForKind("authorization", &Authorization{}, &AuthorizationList{}),
		registerTypeForKind("checknothing", &Checknothing{}, &ChecknothingList{}),
		registerTypeForKind("kubernetes", &Kubernetes{}, &KubernetesList{}),
		registerTypeForKind("listentry", &Listentry{}, &ListentryList{}),
		registerTypeForKind("logentry", &Logentry{}, &LogentryList{}),
		registerTypeForKind("edge", &Edge{}, &EdgeList{}),
		registerTypeForKind("metric", &Metric{}, &MetricList{}),
		registerTypeForKind("quota", &Quota{}, &QuotaList{}),
		registerTypeForKind("reportnothing", &Reportnothing{}, &ReportnothingList{}),
		registerTypeForKind("tracespan", &Tracespan{}, &TracespanList{}),
		registerTypeForKind("cloudwatch", &Cloudwatch{}, &CloudwatchList{}),
		registerTypeForKind("dogstatsd", &Dogstatsd{}, &DogstatsdList{}),
		registerTypeForKind("zipkin", &Zipkin{}, &ZipkinList{}),
	)
	SchemeBuilder.Register(
		&HTTPAPISpecBinding{}, &HTTPAPISpecBindingList{},
		&HTTPAPISpec{}, &HTTPAPISpecList{},
		&QuotaSpecBinding{}, &QuotaSpecBindingList{},
		&QuotaSpec{}, &QuotaSpecList{},
	)
}

func registerTypeForKind(kind string, single, list runtime.Object) func(*runtime.Scheme) error {
	return func(s *runtime.Scheme) error {
		s.AddKnownTypeWithName(SchemeGroupVersion.WithKind(kind), single)
		s.AddKnownTypeWithName(SchemeGroupVersion.WithKind(kind+"List"), list)
		return nil
	}
}
