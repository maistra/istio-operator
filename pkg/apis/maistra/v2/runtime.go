package v2

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ControlPlaneRuntimeConfig configures execution parameters for control plane
// componets.
type ControlPlaneRuntimeConfig struct {
	// Citadel configures overrides for citadel deployment/pods
	// .Values.security.resources, e.g.
	Citadel *ComponentRuntimeConfig `json:"citadel,omitempty"`
	// Galley configures overrides for galley deployment/pods
	// .Values.galley.resources, e.g.
	Galley *ComponentRuntimeConfig `json:"galley,omitempty"`
	// Pilot configures overrides for pilot/istiod deployment/pods
	// .Values.pilot.resources, e.g.
	Pilot *ComponentRuntimeConfig `json:"pilot,omitempty"`
	// Defaults will be merged into specific component config.
	// .Values.global.defaultResources, e.g.
	Defaults *DefaultRuntimeConfig `json:"defaults,omitempty"`
}

// ComponentRuntimeConfig allows for partial customization of a component's
// runtime configuration (Deployment, PodTemplate, auto scaling, pod disruption, etc.)
type ComponentRuntimeConfig struct {
	// Deployment specific overrides
	Deployment DeploymentRuntimeConfig `json:"deployment,omitempty"`
	// Pod specific overrides
	Pod PodRuntimeConfig `json:"pod,omitempty"`
}

// DeploymentRuntimeConfig allow customization of a component's Deployment
// resource, including additional labels/annotations, replica count, autoscaling,
// rollout strategy, etc.
type DeploymentRuntimeConfig struct {
	// Metadata specifies additional labels and annotations to be applied to the deployment
	Metadata MetadataConfig `json:"metadata,omitempty"`
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

	// The number of old ReplicaSets to retain to allow rollback.
	// This is a pointer to distinguish between explicit zero and not specified.
	// Defaults to 10.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// Autoscaling specifies the configuration for a HorizontalPodAutoscaler
	// to be applied to this deployment.  Null indicates no auto scaling.
	// .Values.*.autoscale* fields
	AutoScaling *AutoScalerConfig `json:"autoScaling,omitempty"`

	// .Values.global.podDisruptionBudget.enabled, if not null
	// XXX: this is currently a global setting, not per component.  perhaps
	// this should only be available on the defaults?
	Disruption *PodDisruptionBudget `json:"disruption,omitempty"`
}

// AutoScalerConfig is used to configure autoscaling for a deployment
type AutoScalerConfig struct {
	// lower limit for the number of pods that can be set by the autoscaler, default 1.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// upper limit for the number of pods that can be set by the autoscaler; cannot be smaller than MinReplicas.
	MaxReplicas int32 `json:"maxReplicas"`
	// target average CPU utilization (represented as a percentage of requested CPU) over all the pods;
	// if not specified the default autoscaling policy will be used.
	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`
}

// PodRuntimeConfig is used to customize pod configuration for a component
type PodRuntimeConfig struct {
	// Metadata allows additional annotations/labels to be applied to the pod
	// .Values.*.podAnnotations
	// XXX: currently, additional lables are not supported
	Metadata MetadataConfig `json:"metadata,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	// .Values.nodeSelector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// If specified, the pod's scheduling constraints
	// +optional
	// .Values.podAffinityLabelSelector, podAntiAffinityLabelSelector, nodeSelector
	// XXX: this is more descriptive than what is currently exposed (i.e. only pod affinities and nodeSelector)
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// If specified, the pod will be dispatched by specified scheduler.
	// If not specified, the pod will be dispatched by default scheduler.
	// +optional
	// XXX: not currently supported
	SchedulerName string `json:"schedulerName,omitempty"`

	// If specified, the pod's tolerations.
	// +optional
	// .Values.tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// .Values.global.priorityClassName
	// XXX: currently, this is only a global setting.  maybe only allow setting in global runtime defaults?
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// XXX: is it too cheesy to use 'default' name for defaults?  default would apply to all containers
	// .Values.*.resource, imagePullPolicy, etc.
	Containers map[string]ContainerConfig `json:"containers,omitempty"`
}

// ContainerConfig to be applied to containers in a pod, in a deployment
type ContainerConfig struct {
	ImagePullPolicy  corev1.PullPolicy                      `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference          `json:"imagePullSecrets,omitempty"`
	Resources        map[string]corev1.ResourceRequirements `json:"resources,omitempty"`
}

// PodDisruptionBudget details
// XXX: currently not configurable (i.e. no values.yaml equivalent)
type PodDisruptionBudget struct {
	MinAvailable   *intstr.IntOrString `json:"minAvailable,omitempty"`
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// DefaultRuntimeConfig specifies default execution parameters to apply to
// control plane deployments/pods when no specific component overrides have been
// specified.  These settings will be merged with component specific settings.
type DefaultRuntimeConfig struct {
	// Metadata to apply to all components.
	Metadata MetadataConfig `json:"metadata,omitempty"`
	// Container overrides to be merged with component specific overrides.
	Container *ContainerConfig `json:"container,omitempty"`
}

// MetadataConfig represents additional metadata to be applied to resources
type MetadataConfig struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ComponentServiceConfig is used to customize the service associated with a component.
type ComponentServiceConfig struct {
	// Metadata represents addtional annotations/labels to be applied to the
	// component's service.
	Metadata MetadataConfig `json:"metadata,omitempty"`
	// NodePort specifies a NodePort for the component's Service.
	// .Values.prometheus.service.nodePort.port, ...enabled is true if not null
	NodePort *int32 `json:"nodePort,omitempty"`
	// Ingress specifies details for accessing the component's service through
	// a k8s Ingress or OpenShift Route.
	Ingress *ComponentIngressConfig `json:"ingress,omitempty"`
}

// ComponentIngressConfig is used to customize a k8s Ingress or OpenShift Route
// for the service associated with a component.
type ComponentIngressConfig struct {
	// Metadata represents additional metadata to be applied to the ingress/route.
	Metadata MetadataConfig `json:"metadata,omitempty"`
	// Hosts represents a list of host names to configure.  Note, OpenShift route
	// only supports a single host name per route.  An empty host name implies
	// a default host name for the Route.
	// XXX: is a host name required for k8s Ingress?
	Hosts []string `json:"hosts,omitempty"`
	// ContextPath represents the context path to the service.
	ContextPath string `json:"contextPath,omitempty"`
	// TLS is used to configure TLS for the Ingress/Route
	// XXX: should this be something like RawExtension, as the configuration differs between Route and Ingress?
	TLS map[string]string `json:"tls,omitempty"`
}

// ComponentPersistenceConfig is used to configure persistance for a component.
type ComponentPersistenceConfig struct {
	// StorageClassName for the PersistentVolume
	StorageClassName string `json:"storageClassName,omitempty"`
	// AccessModes for the PersistentVolume
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	// Capacity to request for the PersistentVolume
	Capacity corev1.ResourceList `json:"capacity,omitempty"`
}
