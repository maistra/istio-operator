package v1

import (
	meshv1alpha1 "istio.io/api/mesh/v1alpha1"
)

// DeepCopyInto is a custom deepcopy function for MeshNetworksType, copying the receiver, writing into out. in must be non-nil.
func (in MeshNetworksType) DeepCopyInto(out *MeshNetworksType) {
	{
		in := &in
		*out = make(MeshNetworksType, len(*in))
		for key, val := range *in {
			(*out)[key] = deepCopyMeshNetwork(val)
		}
		return
	}
}

func deepCopyMeshNetwork(in meshv1alpha1.Network) meshv1alpha1.Network {
	data, err := in.Marshal()
	if err != nil {
		// panic???
		return meshv1alpha1.Network{}
	}
	out := meshv1alpha1.Network{}
	err = out.Unmarshal(data)
	if err != nil {
		// panic???
		return out
	}
	return out
}
