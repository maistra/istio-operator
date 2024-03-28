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
	"reflect"
	"testing"
)

func TestGetString(t *testing.T) {
	testCases := []struct {
		name        string
		input       Values
		key         string
		expectFound bool
		expected    string
		expectErr   bool
	}{
		{
			name: "valid Key",
			input: Values{
				"foo": map[string]any{
					"bar": "baz",
				},
			},
			key:         "foo.bar",
			expectFound: true,
			expected:    "baz",
		},
		{
			name: "invalid Key",
			input: Values{
				"foo": "baz",
			},
			key:         "foo.bar",
			expectFound: false,
			expectErr:   true,
		},
		{
			name: "nonexistent key",
			input: Values{
				"foo": map[string]any{
					"bar": "baz",
				},
			},
			key:         "foo.baz",
			expectFound: false,
			expectErr:   false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			result, found, err := test.input.GetString(test.key)
			if test.expectErr {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got an error: %v", err)
				}
				if result != test.expected {
					t.Errorf("Expected %s, but got %s", test.expected, result)
				}
			}

			if found != test.expectFound {
				t.Errorf("Expected found to be %v, but it was %v", test.expectFound, found)
			}
		})
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		name      string
		input     Values
		key       string
		val       string
		expected  Values
		expectErr bool
	}{
		{
			name: "Valid Key",
			input: Values{
				"foo": map[string]any{
					"bar": "baz",
				},
			},
			key: "foo.bar",
			val: "newVal",
			expected: Values{
				"foo": map[string]any{
					"bar": "newVal",
				},
			},
		},
		{
			name: "Non-Existent Key",
			input: Values{
				"foo": map[string]any{
					"bar": "baz",
				},
			},
			key: "foo.baz",
			val: "newVal",
			expected: Values{
				"foo": map[string]any{
					"bar": "baz",
					"baz": "newVal",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.input.Set(test.key, test.val)
			if test.expectErr {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got an error: %v", err)
				}
				if !reflect.DeepEqual(test.input, test.expected) {
					t.Errorf("Expected %v, but got %v", test.expected, test.input)
				}
			}
		})
	}
}
