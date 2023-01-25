package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/helm/pkg/releaseutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"
)

var log = logf.Log.WithName("cmd")

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling flag.Parse().
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	logf.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	files := pflag.Args()

	written := sets.NewString()
	for _, file := range files {
		log.Info(fmt.Sprintf("processing file %s", file))
		processFile(file, written)
	}
	log.Info("completed")
}

func processFile(fileName string, written sets.String) {
	if fileInfo, err := os.Stat(fileName); err == nil {
		if fileInfo.IsDir() {
			if file, err := os.Open(fileName); err == nil {
				if files, err := file.Readdirnames(-1); err == nil {
					for _, dirFile := range files {
						processFile(path.Join(fileName, dirFile), written)
					}
				} else {
					log.Error(err, fmt.Sprintf("error reading files in dir %s", fileName))
					panic(err)
				}
			} else {
				panic(err)
			}
		} else {
			dirName := path.Dir(fileName)
			rawYaml := readFile(fileName)
			objects := releaseutil.SplitManifests(rawYaml)
			for _, object := range objects {
				if rawJSON, err := yaml.YAMLToJSON([]byte(object)); err == nil {
					parsed := make(map[string]interface{})
					if err := json.Unmarshal(rawJSON, &parsed); err != nil {
						log.Error(err, fmt.Sprintf("%s contains invalid YAML: \"%s\"", fileName, object))
						panic(err)
					}
					if kind, ok, _ := unstructured.NestedString(parsed, "kind"); ok && kind == "CustomResourceDefinition" {
						if name, ok, err := unstructured.NestedString(parsed, "metadata", "name"); ok && !written.Has(name) {
							written.Insert(name)
							outFileName := path.Join(dirName, fmt.Sprintf("%s.crd.yaml", name))
							if _, err := os.Stat(outFileName); err == nil {
								panic(fmt.Errorf("output file exists: %s", outFileName))
							} else if os.IsNotExist(err) {
								log.Info(fmt.Sprintf("writing CRD %s to %s", name, outFileName))
								if err := os.WriteFile(outFileName, []byte(object), 0o664); err != nil {
									log.Error(err, fmt.Sprintf("error writing CRD to %s", outFileName))
									panic(err)
								}
							} else {
								panic(err)
							}
						} else if !ok {
							log.Error(err, "error retrieving name from CRD")
							panic(fmt.Errorf("CRD is missing name field: \n%s", parsed))
						} else {
							log.Info(fmt.Sprintf("skipping CRD %s", name))
						}
					} else {
						log.Info(fmt.Sprintf("skipping resource \"%s\" in %s", kind, fileName))
					}
				} else {
					log.Error(err, fmt.Sprintf("error converting raw yaml to json: %s", object))
				}
			}
		}
	} else {
		panic(err)
	}
}

func readFile(name string) string {
	data, err := os.ReadFile(name)
	if err != nil {
		log.Error(err, fmt.Sprintf("error reading file: %s", name))
		panic(err)
	}
	return string(data)
}
