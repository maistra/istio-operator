// Copyright Istio Authors
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

package helm

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Values map[string]any

// GetBool returns the bool value of a nested field.
// Returns false if value is not found and an error if not a bool.
func (h *Values) GetBool(key string) (bool, bool, error) {
	return unstructured.NestedBool(*h, toKeys(key)...)
}

// GetString returns the string value of a nested field.
// Returns false if value is not found and an error if not a string.
func (h *Values) GetString(key string) (string, bool, error) {
	return unstructured.NestedString(*h, toKeys(key)...)
}

// Set sets the value of a nested field to a deep copy of the value provided.
// Returns an error if value cannot be set because one of the nesting levels is not a map[string]any.
func (h *Values) Set(key string, val any) error {
	return unstructured.SetNestedField(*h, val, toKeys(key)...)
}

func toKeys(key string) []string {
	return strings.Split(key, ".")
}

func FromValues(values any) Values {
	var obj Values
	data, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, &obj); err != nil {
		panic(err)
	}
	return obj
}

func ToValues[V any](helmValues Values, values V) (V, error) {
	data, err := json.Marshal(helmValues)
	if err != nil {
		return values, err
	}

	decoder := json.NewDecoder(strings.NewReader(string(data)))
	err = decoder.Decode(&values)
	if err != nil {
		return values, fmt.Errorf("failed to unmarshal into Values struct: %v:\n%v", err, string(data))
	}
	return values, nil
}
