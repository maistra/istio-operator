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

package config

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var testImages = IstioImageConfig{
	IstiodImage:  "istiod-test",
	ProxyImage:   "proxy-test",
	CNIImage:     "cni-test",
	ZTunnelImage: "ztunnel-test",
}

func TestReadConfig(t *testing.T) {
	testCases := []struct {
		name           string
		configFile     string
		expectedConfig OperatorConfig
		success        bool
	}{
		{
			name: "single-version",
			configFile: `
images.v1_20_0.istiod=istiod-test
images.v1_20_0.proxy=proxy-test
images.v1_20_0.cni=cni-test
images.v1_20_0.ztunnel=ztunnel-test
`,
			expectedConfig: OperatorConfig{
				ImageDigests: map[string]IstioImageConfig{
					"v1.20.0": testImages,
				},
			},
			success: true,
		},
		{
			name: "multiple-versions",
			configFile: `
images.v1_20_0.istiod=istiod-test
images.v1_20_0.proxy=proxy-test
images.v1_20_0.cni=cni-test
images.v1_20_0.ztunnel=ztunnel-test
images.v1_20_1.istiod=istiod-test
images.v1_20_1.proxy=proxy-test
images.v1_20_1.cni=cni-test
images.v1_20_1.ztunnel=ztunnel-test
images.latest.istiod=istiod-test
images.latest.proxy=proxy-test
images.latest.cni=cni-test
images.latest.ztunnel=ztunnel-test
`,
			expectedConfig: OperatorConfig{
				ImageDigests: map[string]IstioImageConfig{
					"v1.20.0": testImages,
					"v1.20.1": testImages,
					"latest":  testImages,
				},
			},
			success: true,
		},
		{
			name: "missing-proxy",
			configFile: `
images.v1_20_0.istiod=istiod-test
images.v1_20_0.cni=cni-test
images.v1_20_0.ztunnel=ztunnel-test
`,
			success: false,
		},
	}
	for _, tc := range testCases {
		file, err := os.CreateTemp("", "operator-unit-")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			err = file.Close()
			if err != nil {
				t.Fatal(err)
			}
			os.Remove(file.Name())
		}()

		file.WriteString(tc.configFile)
		err = ReadConfig(file.Name())
		if !tc.success {
			if err != nil {
				return
			}
			t.Fatal("expected error but got:", err)
		} else if err != nil {
			t.Fatal("expected no error but got:", err)
		}
		if diff := cmp.Diff(Config, tc.expectedConfig); diff != "" {
			t.Fatal("config did not match expectation:\n\n", diff)
		}
	}
}
