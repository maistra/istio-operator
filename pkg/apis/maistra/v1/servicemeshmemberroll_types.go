package v1

import (
	"strings"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

func init() {
	SchemeBuilder.Register(&ServiceMeshMemberRoll{}, &ServiceMeshMemberRollList{})
}

// The ServiceMeshMemberRoll object configures which namespaces belong to a
// service mesh. Only namespaces listed in the ServiceMeshMemberRoll will be
// affected by the control plane. Any number of namespaces can be added, but a
// namespace may not exist in more than one service mesh. The
// ServiceMeshMemberRoll object must be created in the same namespace as
// the ServiceMeshControlPlane object and must be named "default".
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=smmr,categories=maistra-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.annotations.configuredMemberCount",description="How many of the total number of member namespaces are configured"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].reason",description="Whether all member namespaces have been configured or why that's not the case"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"
// +kubebuilder:printcolumn:name="Members",type="string",JSONPath=".status.members",description="Namespaces that are members of this Control Plane",priority=1
type ServiceMeshMemberRoll struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired list of members of the service mesh.
	// +kubebuilder:validation:Required
	Spec ServiceMeshMemberRollSpec `json:"spec"`

	// The current status of this ServiceMeshMemberRoll. This data may be out
	// of date by some window of time.
	Status ServiceMeshMemberRollStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshMemberRollList contains a list of ServiceMeshMemberRoll
type ServiceMeshMemberRollList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceMeshMemberRoll `json:"items"`
}

// ServiceMeshMemberRollSpec is the specification of the desired list of
// members of the service mesh.
type ServiceMeshMemberRollSpec struct {
	//  List of namespaces that should be members of the service mesh.
	// +optional
	// +nullable
	Members []string `json:"members,omitempty"`

	// Include namespaces with label keys and values matching this selector.
	// +optional
	MemberSelector *metav1.LabelSelector `json:"memberSelector,omitempty"`

	// List of namespaces that should be excluded, even if matched by a wildcard
	// in spec.members or by the memberSelector.
	// +optional
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`
}

func (smmr *ServiceMeshMemberRoll) IsWildcard(ns string) bool {
	return strings.Contains(ns, "*")
}

func (smmr *ServiceMeshMemberRoll) IsMember(ns *core.Namespace) bool {
	return smmr.isIncluded(ns) && !smmr.isExcluded(ns)
}

func (smmr *ServiceMeshMemberRoll) isIncluded(ns *core.Namespace) bool {
	// check if namespace name matches any name or wildcard in spec.members
	if matchAny(ns.Name, smmr.Spec.Members) {
		return true
	}

	// check if namespace labels match the label selector
	return selectorMatches(smmr.Spec.MemberSelector, ns.Labels)
}

func (smmr *ServiceMeshMemberRoll) isExcluded(ns *core.Namespace) bool {
	return ns.Name == smmr.Namespace || matchAny(ns.Name, smmr.Spec.ExcludeNamespaces)
}

// MatchesNamespacesDynamically returns true if the SMMR contains wildcards in
// spec.members or defines a member selector. In either case, the list of members
// is dynamic, as the member namespace list can change with no change to the SMMR.
func (smmr *ServiceMeshMemberRoll) MatchesNamespacesDynamically() bool {
	if smmr.Spec.MemberSelector != nil {
		return true
	}
	for _, m := range smmr.Spec.Members {
		if smmr.IsWildcard(m) {
			return true
		}
	}
	return false
}

func matchAny(str string, patterns []string) bool {
	for _, pattern := range patterns {
		if match(str, pattern) {
			return true
		}
	}
	return false
}

func match(str string, pattern string) bool {
	if pattern == "*" || str == pattern {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		if strings.HasPrefix(str, prefix) {
			return true
		}
	}
	return false
}

func selectorMatches(selector *metav1.LabelSelector, labels map[string]string) bool {
	if selector == nil {
		return false
	}

	for k, v := range selector.MatchLabels {
		if labels[k] != v {
			return false
		}
	}

	for _, requirement := range selector.MatchExpressions {
		value, exists := labels[requirement.Key]
		switch requirement.Operator {
		case metav1.LabelSelectorOpIn:
			values := sets.NewString(requirement.Values...)
			if !values.Has(value) {
				return false
			}
		case metav1.LabelSelectorOpNotIn:
			for _, v := range requirement.Values {
				if value == v {
					return false
				}
			}
		case metav1.LabelSelectorOpExists:
			if !exists {
				return false
			}
		case metav1.LabelSelectorOpDoesNotExist:
			if exists {
				return false
			}
		}
	}
	return true
}

// ServiceMeshMemberRollStatus represents the current state of a ServiceMeshMemberRoll.
type ServiceMeshMemberRollStatus struct {
	status.StatusBase `json:",inline"`

	// The generation observed by the controller during the most recent
	// reconciliation. The information in the status pertains to this particular
	// generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The generation of the ServiceMeshControlPlane object observed by the
	// controller during the most recent reconciliation of this
	// ServiceMeshMemberRoll.
	ServiceMeshGeneration int64 `json:"meshGeneration,omitempty"`

	// The reconciled version of the ServiceMeshControlPlane object observed by
	// the controller during the most recent reconciliation of this
	// ServiceMeshMemberRoll.
	ServiceMeshReconciledVersion string `json:"meshReconciledVersion,omitempty"`

	// Complete list of namespaces that are configured as members of the service
	// mesh	- this includes namespaces specified in spec.members and those that
	// contain a ServiceMeshMember object
	// +optional
	// +nullable
	Members []string `json:"members"`

	// List of namespaces that are configured as members of the service mesh.
	// +optional
	// +nullable
	ConfiguredMembers []string `json:"configuredMembers"`

	// List of namespaces that haven't been configured as members of the service
	// mesh yet.
	// +optional
	// +nullable
	PendingMembers []string `json:"pendingMembers"`

	// List of namespaces that are being removed as members of the service
	// mesh.
	// +optional
	// +nullable
	TerminatingMembers []string `json:"terminatingMembers"`

	// Represents the latest available observations of this ServiceMeshMemberRoll's
	// current state.
	// +optional
	// +nullable
	Conditions []ServiceMeshMemberRollCondition `json:"conditions"`

	// Represents the latest available observations of each member's
	// current state.
	// +optional
	// +nullable
	MemberStatuses []ServiceMeshMemberStatusSummary `json:"memberStatuses"`
}

// ServiceMeshMemberStatusSummary represents a summary status of a ServiceMeshMember.
type ServiceMeshMemberStatusSummary struct {
	Namespace  string                       `json:"namespace"`
	Conditions []ServiceMeshMemberCondition `json:"conditions"`
}

// ServiceMeshMemberRollConditionType represents the type of the condition.  Condition types are:
// Reconciled, NamespaceConfigured
type ServiceMeshMemberRollConditionType string

const (
	// ConditionTypeMemberRollReady signifies whether the namespace has been configured
	// to use the mesh
	ConditionTypeMemberRollReady ServiceMeshMemberRollConditionType = "Ready"
)

type ServiceMeshMemberRollConditionReason string

const (
	// ConditionReasonConfigured indicates that all namespaces were configured
	ConditionReasonConfigured ServiceMeshMemberRollConditionReason = "Configured"
	// ConditionReasonReconcileError indicates that one of the namespaces to configure could not be configured
	ConditionReasonReconcileError ServiceMeshMemberRollConditionReason = "ReconcileError"
	// ConditionReasonSMCPMissing indicates that the ServiceMeshControlPlane resource does not exist
	ConditionReasonSMCPMissing ServiceMeshMemberRollConditionReason = "ErrSMCPMissing"
	// ConditionReasonMultipleSMCP indicates that multiple ServiceMeshControlPlane resources exist in the namespace
	ConditionReasonMultipleSMCP ServiceMeshMemberRollConditionReason = "ErrMultipleSMCPs"
	// ConditionReasonInvalidName indicates that the ServiceMeshMemberRoll name is invalid (only "default" is allowed)
	ConditionReasonInvalidName ServiceMeshMemberRollConditionReason = "ErrInvalidName"
	// ConditionReasonSMCPNotReconciled indicates that reconciliation of the SMMR was skipped because the SMCP has not been reconciled
	ConditionReasonSMCPNotReconciled ServiceMeshMemberRollConditionReason = "SMCPReconciling"
)

// Condition represents a specific condition on a resource
type ServiceMeshMemberRollCondition struct {
	Type               ServiceMeshMemberRollConditionType   `json:"type,omitempty"`
	Status             core.ConditionStatus                 `json:"status,omitempty"`
	LastTransitionTime metav1.Time                          `json:"lastTransitionTime,omitempty"`
	Reason             ServiceMeshMemberRollConditionReason `json:"reason,omitempty"`
	Message            string                               `json:"message,omitempty"`
}

// GetCondition removes a condition for the list of conditions
func (s *ServiceMeshMemberRollStatus) GetCondition(conditionType ServiceMeshMemberRollConditionType) ServiceMeshMemberRollCondition {
	if s == nil {
		return ServiceMeshMemberRollCondition{Type: conditionType, Status: core.ConditionUnknown}
	}
	for i := range s.Conditions {
		if s.Conditions[i].Type == conditionType {
			return s.Conditions[i]
		}
	}
	return ServiceMeshMemberRollCondition{Type: conditionType, Status: core.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *ServiceMeshMemberRollStatus) SetCondition(condition ServiceMeshMemberRollCondition) *ServiceMeshMemberRollStatus {
	if s == nil {
		return nil
	}
	now := metav1.Now()
	for i := range s.Conditions {
		if s.Conditions[i].Type == condition.Type {
			if s.Conditions[i].Status != condition.Status {
				condition.LastTransitionTime = now
			} else {
				condition.LastTransitionTime = s.Conditions[i].LastTransitionTime
			}
			s.Conditions[i] = condition
			return s
		}
	}

	// If the condition does not exist,
	// initialize the lastTransitionTime
	condition.LastTransitionTime = now
	s.Conditions = append(s.Conditions, condition)
	return s
}
