package common

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
	Images3_0 ImageConfig3_0 `properties:"images3_0"`
}

type ImageConfig3_0 struct {
	Istiod string `properties:"istiod"`
	Proxy  string `properties:"proxy"`
	CNI    string `properties:"cni"`
}

func ReadConfig(configFile string) error {
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
	return nil
}
