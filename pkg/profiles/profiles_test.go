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

package profiles

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
)

func TestGetValuesFromProfiles(t *testing.T) {
	const version = "my-version"
	resourceDir := t.TempDir()
	profilesDir := path.Join(resourceDir, version, "profiles")
	Must(t, os.MkdirAll(profilesDir, 0o755))

	writeProfileFile := func(t *testing.T, path string, values ...string) {
		yaml := `
apiVersion: operator.istio.io/v1alpha1
kind: IstioProfile
spec:
  values:`
		for i, val := range values {
			if val != "" {
				yaml += fmt.Sprintf(`
    value%d: %s`, i+1, val)
			}
		}
		Must(t, os.WriteFile(path, []byte(yaml), 0o644))
	}

	writeProfileFile(t, path.Join(profilesDir, "default.yaml"), "1-from-default", "2-from-default")
	writeProfileFile(t, path.Join(profilesDir, "overlay.yaml"), "", "2-from-overlay")
	writeProfileFile(t, path.Join(profilesDir, "custom.yaml"), "1-from-custom")
	writeProfileFile(t, path.Join(resourceDir, version, "not-in-profiles-dir.yaml"), "should-not-be-accessible")

	tests := []struct {
		name         string
		profiles     []string
		expectValues helm.HelmValues
		expectErr    bool
	}{
		{
			name:         "nil default profiles",
			profiles:     nil,
			expectValues: helm.HelmValues{},
		},
		{
			name:     "default profile only",
			profiles: []string{"default"},
			expectValues: helm.HelmValues{
				"value1": "1-from-default",
				"value2": "2-from-default",
			},
		},
		{
			name:     "default and overlay",
			profiles: []string{"default", "overlay"},
			expectValues: helm.HelmValues{
				"value1": "1-from-default",
				"value2": "2-from-overlay",
			},
		},
		{
			name:     "default and overlay and custom",
			profiles: []string{"default", "overlay", "custom"},
			expectValues: helm.HelmValues{
				"value1": "1-from-custom",
				"value2": "2-from-overlay",
			},
		},
		{
			name:      "default profile empty",
			profiles:  []string{""},
			expectErr: true,
		},
		{
			name:      "profile not found",
			profiles:  []string{"invalid"},
			expectErr: true,
		},
		{
			name:      "path-traversal-attack",
			profiles:  []string{"../not-in-profiles-dir"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getValuesFromProfiles(profilesDir, tt.profiles)
			if (err != nil) != tt.expectErr {
				t.Errorf("applyProfile() error = %v, expectErr %v", err, tt.expectErr)
			}

			if err == nil {
				if diff := cmp.Diff(tt.expectValues, actual); diff != "" {
					t.Errorf("profile wasn't applied properly; diff (-expected, +actual):\n%v", diff)
				}
			}
		})
	}
}

func TestMergeOverwrite(t *testing.T) {
	testCases := []struct {
		name                    string
		overrides, base, expect map[string]any
	}{
		{
			name:      "both empty",
			base:      make(map[string]any),
			overrides: make(map[string]any),
			expect:    make(map[string]any),
		},
		{
			name:      "nil overrides",
			base:      map[string]any{"key1": 42, "key2": "value"},
			overrides: nil,
			expect:    map[string]any{"key1": 42, "key2": "value"},
		},
		{
			name:      "nil base",
			base:      nil,
			overrides: map[string]any{"key1": 42, "key2": "value"},
			expect:    map[string]any{"key1": 42, "key2": "value"},
		},
		{
			name: "adds toplevel keys",
			base: map[string]any{
				"key2": "from base",
			},
			overrides: map[string]any{
				"key1": "from overrides",
			},
			expect: map[string]any{
				"key1": "from overrides",
				"key2": "from base",
			},
		},
		{
			name: "adds nested keys",
			base: map[string]any{
				"key1": map[string]any{
					"nested2": "from base",
				},
			},
			overrides: map[string]any{
				"key1": map[string]any{
					"nested1": "from overrides",
				},
			},
			expect: map[string]any{
				"key1": map[string]any{
					"nested1": "from overrides",
					"nested2": "from base",
				},
			},
		},
		{
			name: "overrides overrides base",
			base: map[string]any{
				"key1": "from base",
				"key2": map[string]any{
					"nested1": "from base",
				},
			},
			overrides: map[string]any{
				"key1": "from overrides",
				"key2": map[string]any{
					"nested1": "from overrides",
				},
			},
			expect: map[string]any{
				"key1": "from overrides",
				"key2": map[string]any{
					"nested1": "from overrides",
				},
			},
		},
		{
			name: "mismatched types",
			base: map[string]any{
				"key1": map[string]any{
					"desc": "key1 is a map in base",
				},
				"key2": "key2 is a string in base",
			},
			overrides: map[string]any{
				"key1": "key1 is a string in overrides",
				"key2": map[string]any{
					"desc": "key2 is a map in overrides",
				},
			},
			expect: map[string]any{
				"key1": "key1 is a string in overrides",
				"key2": map[string]any{
					"desc": "key2 is a map in overrides",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeOverwrite(tc.base, tc.overrides)
			if diff := cmp.Diff(tc.expect, result); diff != "" {
				t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
