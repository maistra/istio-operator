package main

import (
	"fmt"

	"github.com/spf13/pflag"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-tools/pkg/genall"
	"sigs.k8s.io/controller-tools/pkg/markers"

	"github.com/maistra/istio-operator/tools/pkg/doc"
)

var log = logf.Log.WithName("cmd")

// optionsRegistry contains all the marker definitions used to process command line options
var optionsRegistry = &markers.Registry{}

func main() {
	pflag.Parse()

	args := append([]string{"doc"}, pflag.Args()...)
	rt, err := genall.FromOptions(optionsRegistry, args)
	if err != nil {
		panic(err)
	}
	if len(rt.Generators) == 0 {
		panic(fmt.Errorf("no generators specified"))
	}

	if hadErrs := rt.Run(); hadErrs {
		panic(hadErrs)
	}
}

func init() {
	genName := "doc"
	generator := doc.Generator{}
	ruleName := "dir"
	rule := genall.OutputToDirectory("")
	defn := markers.Must(markers.MakeDefinition(genName, markers.DescribesPackage, generator))
	if err := optionsRegistry.Register(defn); err != nil {
		panic(err)
	}
	ruleMarker := markers.Must(markers.MakeDefinition(fmt.Sprintf("output:%s:%s", genName, ruleName), markers.DescribesPackage, rule))
	if err := optionsRegistry.Register(ruleMarker); err != nil {
		panic(err)
	}
	ruleMarker = markers.Must(markers.MakeDefinition("output:"+ruleName, markers.DescribesPackage, rule))
	if err := optionsRegistry.Register(ruleMarker); err != nil {
		panic(err)
	}
	if err := genall.RegisterOptionsMarkers(optionsRegistry); err != nil {
		panic(err)
	}
}
