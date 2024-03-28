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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/magiconair/properties"
)

var (
	Config     = OperatorConfig{}
	_, b, _, _ = runtime.Caller(0)

	// Root folder of this project
	// This relies on the fact this file is 2 levels up from the root; if this changes, adjust the path below
	RepositoryRoot = filepath.Join(filepath.Dir(b), "../../")
)

type OperatorConfig struct {
	ImageDigests map[string]IstioImageConfig `properties:"images"`
}

type IstioImageConfig struct {
	IstiodImage  string `properties:"istiod"`
	ProxyImage   string `properties:"proxy"`
	CNIImage     string `properties:"cni"`
	ZTunnelImage string `properties:"ztunnel"`
}

func Read(configFile string) error {
	p, err := properties.LoadFile(configFile, properties.UTF8)
	if err != nil {
		return err
	}
	// remove quotes
	for _, key := range p.Keys() {
		val, _ := p.Get(key)
		_, _, _ = p.Set(key, strings.Trim(val, `"`))
	}
	err = p.Decode(&Config)
	if err != nil {
		return err
	}
	// replace "_" in versions with "." (e.g. v1_20_0 => v1.20.0)
	newImageDigests := make(map[string]IstioImageConfig, len(Config.ImageDigests))
	for k, v := range Config.ImageDigests {
		newImageDigests[strings.Replace(k, "_", ".", -1)] = v
	}
	Config.ImageDigests = newImageDigests
	return nil
}
