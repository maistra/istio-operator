package v2

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ControlPlaneRuntimeConfig configures execution parameters for control plane
// componets.
type ControlPlaneRuntimeConfig struct {
	// Citadel configures overrides for citadel deployment/pods
	// .Values.security.resources, e.g.
	// +optional
	Citadel *ComponentRuntimeConfig `json:"citadel,omitempty"`
	// Galley configures overrides for galley deployment/pods
	// .Values.galley.resources, e.g.
	// +optional
	Galley *ComponentRuntimeConfig `json:"galley,omitempty"`
	// Pilot configures overrides for pilot/istiod deployment/pods
	// .Values.pilot.resources, e.g.
	// +optional
	Pilot *ComponentRuntimeConfig `json:"pilot,omitempty"`
	// Defaults will be merged into specific component config.
	// .Values.global.defaultResources, e.g.
	// +optional
	Defaults *DefaultRuntimeConfig `json:"defaults,omitempty"`
}

// ComponentRuntimeConfig allows for partial customization of a component's
// runtime configuration (Deployment, PodTemplate, auto scaling, pod disruption, etc.)
// XXX: not sure if this needs a separate Container field for component container defaults, e.g. image name, etc.
type ComponentRuntimeConfig struct {
	// Deployment specific overrides
	// +optional
	Deployment DeploymentRuntimeConfig `json:"deployment,omitempty"`
	// Pod specific overrides
	// +optional
	Pod PodRuntimeConfig `json:"pod,omitempty"`
}

// DeploymentRuntimeConfig allow customization of a component's Deployment
// resource, including additional labels/annotations, replica count, autoscaling,
// rollout strategy, etc.
type DeploymentRuntimeConfig struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	// .Values.*.replicaCount
	Replicas *int32 `json:"replicas,omitempty"`

	// The deployment strategy to use to replace existing pods with new ones.
	// +optional
	// +patchStrategy=retainKeys
	// .Values.*.rollingMaxSurge, rollingMaxUnavailable, etc.
	Strategy *appsv1.DeploymentStrategy `json:"strategy,omitempty" patchStrategy:"retainKeys"`

	// Autoscaling specifies the configuration for a HorizontalPodAutoscaler
	// to be applied to this deployment.  Null indicates no auto scaling.
	// .Values.*.autoscale* fields
	// +optional
	AutoScaling *AutoScalerConfig `json:"autoScaling,omitempty"`
}

// CommonDeploymentRuntimeConfig represents deployment settings common to both
// default and component specific settings
type CommonDeploymentRuntimeConfig struct {
	// .Values.global.podDisruptionBudget.enabled, if not null
	// XXX: this is currently a global setting, not per component.  perhaps
	// this should only be available on the defaults?
	// +optional
	Disruption *PodDisruptionBudget `json:"disruption,omitempty"`
}

// AutoScalerConfig is used to configure autoscaling for a deployment
type AutoScalerConfig struct {
	// lower limit for the number of pods that can be set by the autoscaler, default 1.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// upper limit for the number of pods that can be set by the autoscaler; cannot be smaller than MinReplicas.
	// +optional
	MaxReplicas *int32 `json:"maxReplicas"`
	// target average CPU utilization (represented as a percentage of requested CPU) over all the pods;
	// if not specified the default autoscaling policy will be used.
	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`
}

// PodRuntimeConfig is used to customize pod configuration for a component
type PodRuntimeConfig struct {
	CommonPodRuntimeConfig `json:",inline"`

	// Metadata allows additional annotations/labels to be applied to the pod
	// .Values.*.podAnnotations
	// XXX: currently, additional lables are not supported
	// +optional
	Metadata MetadataConfig `json:"metadata,omitempty"`

	// If specified, the pod's scheduling constraints
	// +optional
	// .Values.podAntiAffinityLabelSelector, podAntiAffinityTermLabelSelector, nodeSelector
	// NodeAffinity is not supported at this time
	// PodAffinity is not supported at this time
	Affinity *Affinity `json:"affinity,omitempty"`

	// XXX: is it too cheesy to use 'default' name for defaults?  default would apply to all containers
	// .Values.*.resource, imagePullPolicy, etc.
	// +optional
	Containers map[string]ContainerConfig `json:"containers,omitempty"`
}

// CommonPodRuntimeConfig represents pod settings common to both defaults and
// component specific configuration
type CommonPodRuntimeConfig struct {
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	// .Values.nodeSelector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// If specified, the pod's tolerations.
	// +optional
	// .Values.tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// .Values.global.priorityClassName
	// XXX: currently, this is only a global setting.  maybe only allow setting in global runtime defaults?
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
}

// Affinity is the structure used by Istio for specifying Pod affinity
// XXX: istio does not support full corev1.Affinity settings, hence the special
// types here.
type Affinity struct {
	// XXX: use corev1.PodAntiAffinity instead, the only things not supported are namespaces and weighting
	// +optional
	PodAntiAffinity PodAntiAffinity `json:"podAntiAffinity,omitempty"`
}

// PodAntiAffinity configures anti affinity for pod scheduling
type PodAntiAffinity struct {
	// +optional
	RequiredDuringScheduling  []PodAntiAffinityTerm `json:"requiredDuringScheduling,omitempty"`
	// +optional
	PreferredDuringScheduling []PodAntiAffinityTerm `json:"preferredDuringScheduling,omitempty"`
}

// PodAntiAffinityTerm is a simplified version of corev1.PodAntiAffinityTerm
type PodAntiAffinityTerm struct {
	metav1.LabelSelectorRequirement `json:",inline"`
	// This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching
	// the labelSelector in the specified namespaces, where co-located is defined as running on a node
	// whose value of the label with key topologyKey matches that of any node on which any of the
	// selected pods is running.
	// Empty topologyKey is not allowed.
	// +optional
	TopologyKey string `json:"topologyKey"`
}

// ContainerConfig to be applied to containers in a pod, in a deployment
type ContainerConfig struct {
	CommonContainerConfig `json:",inline"`
	// +optional
	Image                 string `json:"image,omitempty"`
}

// CommonContainerConfig represents container settings common to both defaults
// and component specific configuration.
type CommonContainerConfig struct {
	// +optional
	ImageRegistry    string                        `json:"imageRegistry,omitempty"`
	// +optional
	ImageTag         string                        `json:"imageTag,omitempty"`
	// +optional
	ImagePullPolicy  corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// +optional
	Resources        *corev1.ResourceRequirements  `json:"resources,omitempty"`
}

// PodDisruptionBudget details
// XXX: currently only configurable globally (i.e. no component values.yaml equivalent)
type PodDisruptionBudget struct {
	// +optional
	MinAvailable   *intstr.IntOrString `json:"minAvailable,omitempty"`
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// DefaultRuntimeConfig specifies default execution parameters to apply to
// control plane deployments/pods when no specific component overrides have been
// specified.  These settings will be merged with component specific settings.
type DefaultRuntimeConfig struct {
	// Deployment defaults
	// +optional
	Deployment *CommonDeploymentRuntimeConfig `json:"deployment,omitempty"`
	// Pod defaults
	// +optional
	Pod *CommonPodRuntimeConfig `json:"pod,omitempty"`
	// Container overrides to be merged with component specific overrides.
	// +optional
	Container *CommonContainerConfig `json:"container,omitempty"`
}

// MetadataConfig represents additional metadata to be applied to resources
type MetadataConfig struct {
	// +optional
	Labels      map[string]string `json:"labels,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ComponentServiceConfig is used to customize the service associated with a component.
type ComponentServiceConfig struct {
	// Metadata represents addtional annotations/labels to be applied to the
	// component's service.
	// +optional
	Metadata MetadataConfig `json:"metadata,omitempty"`
	// NodePort specifies a NodePort for the component's Service.
	// .Values.prometheus.service.nodePort.port, ...enabled is true if not null
	// +optional
	NodePort *int32 `json:"nodePort,omitempty"`
	// Ingress specifies details for accessing the component's service through
	// a k8s Ingress or OpenShift Route.
	// +optional
	Ingress *ComponentIngressConfig `json:"ingress,omitempty"`
}

// ComponentIngressConfig is used to customize a k8s Ingress or OpenShift Route
// for the service associated with a component.
type ComponentIngressConfig struct {
	Enablement `json:",inline"`
	// Metadata represents additional metadata to be applied to the ingress/route.
	// +optional
	Metadata MetadataConfig `json:"metadata,omitempty"`
	// Hosts represents a list of host names to configure.  Note, OpenShift route
	// only supports a single host name per route.  An empty host name implies
	// a default host name for the Route.
	// XXX: is a host name required for k8s Ingress?
	// +optional
	Hosts []string `json:"hosts,omitempty"`
	// ContextPath represents the context path to the service.
	// +optional
	ContextPath string `json:"contextPath,omitempty"`
	// TLS is used to configure TLS for the Ingress/Route
	// XXX: should this be something like RawExtension, as the configuration differs between Route and Ingress?
	// +optional
	TLS *v1.HelmValues `json:"tls,omitempty"`
}

// ComponentPersistenceConfig is used to configure persistance for a component.
type ComponentPersistenceConfig struct {
	Enablement `json:",inline"`
	// StorageClassName for the PersistentVolumeClaim
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
	// AccessMode for the PersistentVolumeClaim
	// +optional
	AccessMode corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`
	// Resources to request for the PersistentVolumeClaim
	// +optional
	Resources *corev1.ResourceRequirements `json:"capacity,omitempty"`
}
