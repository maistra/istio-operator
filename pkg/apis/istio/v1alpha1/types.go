package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Installation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              *InstallationSpec   `json:"spec,omitempty"`
	Status            *InstallationStatus `json:"status,omitempty"`
}

type InstallationSpec struct {
	DeploymentType *string `json:"deployment_type,omitempty"`    // "origin"
	Istio      *IstioSpec      `json:"istio,omitempty"`
	Jaeger     *JaegerSpec     `json:"jaeger,omitempty"`
	Kiali      *KialiSpec      `json:"kiali,omitempty"`
	Launcher   *LauncherSpec   `json:"launcher,omitempty"`
	ThreeScale *ThreeScaleSpec `json:"threeScale,omitempty"`
}

type IstioSpec struct {
	Authentication *bool   `json:"authentication,omitempty"`
	Community      *bool   `json:"community,omitempty"`
	Prefix         *string `json:"prefix,omitempty"`             // "maistra/"
	Version        *string `json:"version,omitempty"`            // "0.1.0"
}

type JaegerSpec struct {
	Prefix              *string `json:"prefix,omitempty"`
	Version             *string `json:"version,omitempty"`
	ElasticsearchMemory *string `json:"elasticsearch_memory,omitempty"`  // 1Gi
}

type KialiSpec struct {
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
	Prefix   *string `json:"prefix,omitempty"`    // "kiali/"
	Version  *string `json:"version,omitempty"`   // "v0.5.0"
}

type LauncherSpec struct {
	OpenShift *OpenShiftSpec `json:"openshift,omitempty"`
	GitHub    *GitHubSpec    `json:"github,omitempty"`
	Catalog   *CatalogSpec   `json:"catalog,imitempty"`
}

type OpenShiftSpec struct {
	User     *string `json:"user,omitempty"`
	Password *string `json:"password,omitempty"`
}

type GitHubSpec struct {
	Username *string `json:"username,omitempty"`
	Token    *string `json:"token,omitempty"`
}

type CatalogSpec struct {
	Filter *string `json:"filter,omitempty"`
	Branch *string `json:"branch,omitempty"`
	Repo   *string `json:"repo,omitempty"`
}

type ThreeScaleSpec struct {
	Prefix   *string                `json:"prefix,omitempty"`
	Version  *string                `json:"version,omitempty"`
	Enabled  *bool                  `json:"enabled,omitempty"`
	Adapter  *ThreeScaleAdapterSpec `json:"adapter,omitempty"`
}

type ThreeScaleAdapterSpec struct {
	ListenAddr           *int32  `json:"listenAddr,omitempty"`
	LogLevel             *string `json:"logLevel,omitempty"`
	LogJSON              *bool   `json:"logJSON,omitempty"`
	ReportMetrics        *bool   `json:"reportMetrics,omitempty"`
	MetricsPort          *int32  `json:"metricsPort,omitempty"`
	CacheTTLSeconds      *int    `json:"cacheTTLSeconds,omitempty"`
	CacheRefreshSeconds  *int    `json:"cacheRefreshSeconds,omitempty"`
	CacheEntriesMax      *int    `json:"cacheEntriesMax,omitempty"`
	CacheRefreshRetries  *int    `json:"cacheRefreshRetries,omitempty"`
	AllowInsecureConn    *bool   `json:"allowInsecureConn,omitempty"`
	ClientTimeoutSeconds *int    `json:"clientTimeoutSeconds,omitempty"`
}

type InstallationStatus struct {
	State *string `json:"state,omitempty"`
	Spec              *InstallationSpec   `json:"spec,omitempty"`
}