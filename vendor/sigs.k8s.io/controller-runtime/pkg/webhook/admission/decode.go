/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admission

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type decoder struct {
	codecs serializer.CodecFactory
}

// NewDecoder creates a Decoder given the runtime.Scheme
func NewDecoder(scheme *runtime.Scheme) (types.Decoder, error) {
	return decoder{codecs: serializer.NewCodecFactory(scheme)}, nil
}

// Decode decodes the inlined object in the AdmissionRequest into the passed-in runtime.Object.
func (d decoder) Decode(req types.Request, into runtime.Object) error {
	return d.DecodeRaw(req.AdmissionRequest.Object, into)
}

// DecodeRaw decodes a RawExtension object into the passed-in runtime.Object.
// It errors out if rawObj is empty i.e. containing 0 raw bytes.
func (d decoder) DecodeRaw(rawObj runtime.RawExtension, into runtime.Object) error {
	// we error out if rawObj is an empty object.
	if len(rawObj.Raw) == 0 {
		return fmt.Errorf("there is no content to decode")
	}
	deserializer := d.codecs.UniversalDeserializer()
	return runtime.DecodeInto(deserializer, rawObj.Raw, into)
}
