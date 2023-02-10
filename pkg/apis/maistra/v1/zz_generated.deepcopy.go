//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneSpec) DeepCopyInto(out *ControlPlaneSpec) {
	*out = *in
	if in.Profiles != nil {
		in, out := &in.Profiles, &out.Profiles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Istio != nil {
		in, out := &in.Istio, &out.Istio
		*out = (*in).DeepCopy()
	}
	if in.ThreeScale != nil {
		in, out := &in.ThreeScale, &out.ThreeScale
		*out = (*in).DeepCopy()
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneSpec.
func (in *ControlPlaneSpec) DeepCopy() *ControlPlaneSpec {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneStatus) DeepCopyInto(out *ControlPlaneStatus) {
	*out = *in
	in.StatusBase.DeepCopyInto(&out.StatusBase)
	in.StatusType.DeepCopyInto(&out.StatusType)
	in.ComponentStatusList.DeepCopyInto(&out.ComponentStatusList)
	in.LastAppliedConfiguration.DeepCopyInto(&out.LastAppliedConfiguration)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneStatus.
func (in *ControlPlaneStatus) DeepCopy() *ControlPlaneStatus {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmValues.
func (in *HelmValues) DeepCopy() *HelmValues {
	if in == nil {
		return nil
	}
	out := new(HelmValues)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshControlPlane) DeepCopyInto(out *ServiceMeshControlPlane) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshControlPlane.
func (in *ServiceMeshControlPlane) DeepCopy() *ServiceMeshControlPlane {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshControlPlane)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceMeshControlPlane) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshControlPlaneList) DeepCopyInto(out *ServiceMeshControlPlaneList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceMeshControlPlane, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshControlPlaneList.
func (in *ServiceMeshControlPlaneList) DeepCopy() *ServiceMeshControlPlaneList {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshControlPlaneList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceMeshControlPlaneList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshControlPlaneRef) DeepCopyInto(out *ServiceMeshControlPlaneRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshControlPlaneRef.
func (in *ServiceMeshControlPlaneRef) DeepCopy() *ServiceMeshControlPlaneRef {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshControlPlaneRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMember) DeepCopyInto(out *ServiceMeshMember) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMember.
func (in *ServiceMeshMember) DeepCopy() *ServiceMeshMember {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMember)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceMeshMember) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberCondition) DeepCopyInto(out *ServiceMeshMemberCondition) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberCondition.
func (in *ServiceMeshMemberCondition) DeepCopy() *ServiceMeshMemberCondition {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberList) DeepCopyInto(out *ServiceMeshMemberList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceMeshMember, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberList.
func (in *ServiceMeshMemberList) DeepCopy() *ServiceMeshMemberList {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceMeshMemberList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberRoll) DeepCopyInto(out *ServiceMeshMemberRoll) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberRoll.
func (in *ServiceMeshMemberRoll) DeepCopy() *ServiceMeshMemberRoll {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberRoll)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceMeshMemberRoll) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberRollCondition) DeepCopyInto(out *ServiceMeshMemberRollCondition) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberRollCondition.
func (in *ServiceMeshMemberRollCondition) DeepCopy() *ServiceMeshMemberRollCondition {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberRollCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberRollList) DeepCopyInto(out *ServiceMeshMemberRollList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceMeshMemberRoll, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberRollList.
func (in *ServiceMeshMemberRollList) DeepCopy() *ServiceMeshMemberRollList {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberRollList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceMeshMemberRollList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberRollSpec) DeepCopyInto(out *ServiceMeshMemberRollSpec) {
	*out = *in
	if in.Members != nil {
		in, out := &in.Members, &out.Members
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.MemberSelector != nil {
		in, out := &in.MemberSelector, &out.MemberSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberRollSpec.
func (in *ServiceMeshMemberRollSpec) DeepCopy() *ServiceMeshMemberRollSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberRollSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberRollStatus) DeepCopyInto(out *ServiceMeshMemberRollStatus) {
	*out = *in
	in.StatusBase.DeepCopyInto(&out.StatusBase)
	if in.Members != nil {
		in, out := &in.Members, &out.Members
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ConfiguredMembers != nil {
		in, out := &in.ConfiguredMembers, &out.ConfiguredMembers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PendingMembers != nil {
		in, out := &in.PendingMembers, &out.PendingMembers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.TerminatingMembers != nil {
		in, out := &in.TerminatingMembers, &out.TerminatingMembers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ServiceMeshMemberRollCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.MemberStatuses != nil {
		in, out := &in.MemberStatuses, &out.MemberStatuses
		*out = make([]ServiceMeshMemberStatusSummary, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberRollStatus.
func (in *ServiceMeshMemberRollStatus) DeepCopy() *ServiceMeshMemberRollStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberRollStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberSpec) DeepCopyInto(out *ServiceMeshMemberSpec) {
	*out = *in
	out.ControlPlaneRef = in.ControlPlaneRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberSpec.
func (in *ServiceMeshMemberSpec) DeepCopy() *ServiceMeshMemberSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberStatus) DeepCopyInto(out *ServiceMeshMemberStatus) {
	*out = *in
	in.StatusBase.DeepCopyInto(&out.StatusBase)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ServiceMeshMemberCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberStatus.
func (in *ServiceMeshMemberStatus) DeepCopy() *ServiceMeshMemberStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMeshMemberStatusSummary) DeepCopyInto(out *ServiceMeshMemberStatusSummary) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ServiceMeshMemberCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMeshMemberStatusSummary.
func (in *ServiceMeshMemberStatusSummary) DeepCopy() *ServiceMeshMemberStatusSummary {
	if in == nil {
		return nil
	}
	out := new(ServiceMeshMemberStatusSummary)
	in.DeepCopyInto(out)
	return out
}
