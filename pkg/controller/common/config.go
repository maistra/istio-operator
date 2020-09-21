package common

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

// Config is the config used to drive the operator
var Config = &config{}

// config for the operator
type config struct {
	OLM        olm              `json:"olm,omitempty"`
	OAuthProxy oauthProxy       `json:"oauth-proxy,omitempty"`
	Rendering  renderingOptions `json:"rendering,omitempty"`
	Controller controller       `json:"controller,omitempty"`
}

// OLM is intermediate struct for serialization
type olm struct {
	Images images `json:"relatedImage,omitempty"`
}

// Images for various versions
type images struct {
	V1_0 v1_0ImageNames `json:"v1_0,omitempty"`
	V1_1 v1_1ImageNames `json:"v1_1,omitempty"`
	V2_0 v2_0ImageNames `json:"v2_0,omitempty"`
}

// V1_0ImageNames used by deployments
type v1_0ImageNames struct {
	ThreeScale      string `json:"3scale-istio-adapter,omitempty"`
	Citadel         string `json:"citadel,omitempty"`
	CNI             string `json:"cni,omitempty"`
	Galley          string `json:"galley,omitempty"`
	Grafana         string `json:"grafana,omitempty"`
	Mixer           string `json:"mixer,omitempty"`
	Pilot           string `json:"pilot,omitempty"`
	Prometheus      string `json:"prometheus,omitempty"`
	ProxyInit       string `json:"proxy-init,omitempty"`
	ProxyV2         string `json:"proxyv2,omitempty"`
	SidecarInjector string `json:"sidecar-injector,omitempty"`
}

// V1_1ImageNames used by deployments
type v1_1ImageNames struct {
	ThreeScale      string `json:"3scale-istio-adapter,omitempty"`
	Citadel         string `json:"citadel,omitempty"`
	CNI             string `json:"cni,omitempty"`
	Galley          string `json:"galley,omitempty"`
	Grafana         string `json:"grafana,omitempty"`
	IOR             string `json:"ior,omitempty"`
	Mixer           string `json:"mixer,omitempty"`
	Pilot           string `json:"pilot,omitempty"`
	Prometheus      string `json:"prometheus,omitempty"`
	ProxyInit       string `json:"proxy-init,omitempty"`
	ProxyV2         string `json:"proxyv2,omitempty"`
	SidecarInjector string `json:"sidecar-injector,omitempty"`
}

// V2_0ImageNames used by deployments
type v2_0ImageNames struct {
	ThreeScale string `json:"3scale-istio-adapter,omitempty"`
	CNI        string `json:"cni,omitempty"`
	Grafana    string `json:"grafana,omitempty"`
	Mixer      string `json:"mixer,omitempty"`
	Pilot      string `json:"pilot,omitempty"`
	Prometheus string `json:"prometheus,omitempty"`
	ProxyInit  string `json:"proxy-init,omitempty"`
	ProxyV2    string `json:"proxyv2,omitempty"`
	WASMCacher string `json:"wasm-cacher,omitempty"`
}

type oauthProxy struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Query     bool   `json:"query,omitempty"`
	Tag       string `json:"tag,omitempty"`
	Image     string `json:"-"`
}

// Rendering settings used during chart rendering
type renderingOptions struct {
	// ResourceDir is the base dir to helm charts and templates files.
	ResourceDir string `json:"resourceDir,omitempty"`
	// ChartsDir is the base dir to helm charts.
	ChartsDir string `json:"chartsDir,omitempty"`
	// DefaultTemplatesDir is the base dir to default templates files.
	DefaultTemplatesDir string `json:"defaultTemplatesDir,omitempty"`
	// TemplatesDir is the base dir to user supplied templates files.
	UserTemplatesDir string `json:"userTemplatesDir,omitempty"`
}

// Controller configuration
type controller struct {
	// Number of concurrent reconcilers for each controller
	ControlPlaneReconcilers int `json:"controlPlaneReconcilers,omitempty"`
	MemberRollReconcilers   int `json:"memberRollReconcilers,omitempty"`
	MemberReconcilers       int `json:"memberReconcilers,omitempty"`

	// The number of API requests the operator can make before throttling
	APIBurst int `json:"apiBurst,omitempty"`

	// Then maximum rate of API requests when throttling is active
	APIQPS float32 `json:"apiQPS,omitempty"`
}

// NewViper returns a new viper.Viper configured with all the common.Config keys
// Note, environment variables cannot be used to override command line defaults.
func NewViper() (*viper.Viper, error) {
	delimiter := "."
	replacer := strings.NewReplacer(".", "_", "-", "_", "_", "__")
	v := viper.NewWithOptions(viper.EnvKeyReplacer(replacer), viper.KeyDelimiter(delimiter))
	return v, bindEnvForType(v, Config, delimiter, replacer)
}

// bindEnvForType ensures that env keys are bound for all paths in the type.
func bindEnvForType(v *viper.Viper, t interface{}, delimiter string, replacer *strings.Replacer) error {
	val := reflect.ValueOf(t)
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		return bindEnvForType(v, val.Elem().Interface(), delimiter, replacer)
	case reflect.Struct:
		if len(delimiter) == 0 {
			delimiter = "."
		}
		if replacer == nil {
			replacer = strings.NewReplacer(delimiter, "_")
		}
		return bindType(v, val, "", delimiter, replacer)
	}
	return fmt.Errorf("type to bind must be struct or ptr to a struct")
}

func bindType(v *viper.Viper, val reflect.Value, path string, delimiter string, replacer *strings.Replacer) error {
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		return bindType(v, val.Elem(), path, delimiter, replacer)
	case reflect.Struct:
		structType := val.Type()
		for i := 0; i < structType.NumField(); i++ {
			field := structType.Field(i)
			name := field.Name
			tags := strings.Split(field.Tag.Get("json"), ",")
			tagName := tags[0]
			if tagName != "" {
				name = tagName
			}
			if name == "-" && len(tags) == 1 {
				// field is not serialized
				continue
			}
			if len(path) > 0 {
				inlined := false
				for _, tag := range tags {
					if tag == "inline" {
						inlined = true
						break
					}
				}
				if !inlined {
					name = fmt.Sprintf("%s%s%s", path, delimiter, name)
				}
			}
			err := bindType(v, val.Field(i), name, delimiter, replacer)
			if err != nil {
				return err
			}
		}
	default:
		// simply alias the field to itself
		v.BindEnv(path, strings.ToUpper(replacer.Replace(path)))
		return nil
	}
	return nil
}
