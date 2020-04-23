package versions

import (
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

func errForEnabledValue(obj *v1.HelmValues, path string, disallowed bool) error {
	val, ok, _ := obj.GetField(path)
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
