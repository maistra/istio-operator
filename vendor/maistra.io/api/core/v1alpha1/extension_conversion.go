// Copyright Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"fmt"
	"os"

	apiconversion "k8s.io/apimachinery/pkg/conversion"
	v1 "maistra.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	// Key used in v1's Spec.Config map to store the raw string value from v1alpha1's Spec.Config field when the conversion is not successfull
	RawV1Alpha1Config = "raw_v1alpha1_config"
)

// ConvertTo converts this SME to the Hub version (v1)
func (src *ServiceMeshExtension) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.ServiceMeshExtension)
	return Convert_v1alpha1_ServiceMeshExtension_To_v1_ServiceMeshExtension(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1) to this version.
func (dst *ServiceMeshExtension) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.ServiceMeshExtension)
	return Convert_v1_ServiceMeshExtension_To_v1alpha1_ServiceMeshExtension(src, dst, nil)
}

func Convert_v1_ServiceMeshExtensionSpec_To_v1alpha1_ServiceMeshExtensionSpec(src *v1.ServiceMeshExtensionSpec, dst *ServiceMeshExtensionSpec, scope apiconversion.Scope) error {
	if err := autoConvert_v1_ServiceMeshExtensionSpec_To_v1alpha1_ServiceMeshExtensionSpec(src, dst, nil); err != nil {
		return err
	}

	if raw, ok := src.Config.Data[RawV1Alpha1Config]; ok {
		dst.Config = raw.(string)
	} else {
		res, err := src.Config.MarshalJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "config field could not be converted to string: %v", err)
		}
		dst.Config = string(res)
	}

	return nil
}

func Convert_v1alpha1_ServiceMeshExtensionSpec_To_v1_ServiceMeshExtensionSpec(src *ServiceMeshExtensionSpec, dst *v1.ServiceMeshExtensionSpec, scope apiconversion.Scope) error {
	if err := autoConvert_v1alpha1_ServiceMeshExtensionSpec_To_v1_ServiceMeshExtensionSpec(src, dst, nil); err != nil {
		return err
	}

	if err := dst.Config.UnmarshalJSON([]byte(src.Config)); err != nil {
		fmt.Fprintf(os.Stderr, "v1alpha1 config field (value: %q) could not be converted to v1 json: %v\n", src.Config, err)
		if dst.Config.Data == nil {
			dst.Config.Data = map[string]interface{}{}
		}
		dst.Config.Data[RawV1Alpha1Config] = src.Config
	}

	return nil
}
