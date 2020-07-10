package versions

import (
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

func errForEnabledValue(obj *v1.HelmValues, path string, disallowed bool) error {
	val, ok, _ := obj.GetFieldNoCopy(path)
	if ok {
		switch typedVal := val.(type) {
		case string:
			if strconv.FormatBool(disallowed) == strings.ToLower(typedVal) {
				return fmt.Errorf("%s=%t is not supported", path, disallowed)
			}
		case bool:
			if disallowed == typedVal {
				return fmt.Errorf("%s=%t is not supported", path, disallowed)
			}
		}
	}
	return nil
}

func errForStringValue(obj *v1.HelmValues, path string, allowedValues sets.String) error {
	val, ok, _ := unstructured.NestedFieldNoCopy(obj.GetContent(), strings.Split(path, ".")...)
	if ok {
		switch typedVal := val.(type) {
		case string:
			if !allowedValues.Has(typedVal) {
				return fmt.Errorf("%s=%s is not allowed", path, typedVal)
			}
		default:
			return fmt.Errorf("expected string value at %s", path)
		}
	}
	return nil
}

func getMapKeys(obj *v1.HelmValues, path string) []string {
	val, ok, err := obj.GetFieldNoCopy(path)
	if err != nil || !ok {
		return []string{}
	}
	mapVal, ok := val.(map[string]interface{})
	if !ok {
		return []string{}
	}
	keys := make([]string, len(mapVal))
	for k := range mapVal {
		keys = append(keys, k)
	}
	return keys
}
