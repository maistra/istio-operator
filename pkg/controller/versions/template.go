package versions

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	imagev1 "github.com/openshift/api/image/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

// GetChartsDir returns the location of the Helm charts. Similar layout to istio.io/istio/install/kubernetes/helm.
func (v Ver) GetChartsDir() string {
	if len(common.Config.Rendering.ChartsDir) == 0 {
		return path.Join(common.Config.Rendering.ResourceDir, "helm", v.String())
	}
	return path.Join(common.Config.Rendering.ChartsDir, v.String())
}

// GetTemplatesDir returns the location of the Operator templates files
func (v Ver) GetUserTemplatesDir() string {
	if len(common.Config.Rendering.UserTemplatesDir) == 0 {
		return path.Join(common.Config.Rendering.ResourceDir, "templates")
	}
	return common.Config.Rendering.UserTemplatesDir
}

// GetDefaultTemplatesDir returns the location of the Default Operator templates files
func (v Ver) GetDefaultTemplatesDir() string {
	if len(common.Config.Rendering.DefaultTemplatesDir) == 0 {
		return path.Join(common.Config.Rendering.ResourceDir, "default-templates", v.String())
	}
	return path.Join(common.Config.Rendering.DefaultTemplatesDir, v.String())
}

// common code for managing rendering
// mergeValues merges a map containing input values on top of a map containing
// base values, giving preference to the base values for conflicts
func mergeValues(base map[string]interface{}, input map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{}, 1)
	}

	for key, value := range input {
		// if the key doesn't already exist, add it
		if _, exists := base[key]; !exists {
			base[key] = value
			continue
		}

		// at this point, key exists in both input and base.
		// If both are maps, recurse.
		// If only input is a map, ignore it. We don't want to overrwrite base.
		// If both are values, again, ignore it since we don't want to overrwrite base.
		if baseKeyAsMap, baseOK := base[key].(map[string]interface{}); baseOK {
			if inputAsMap, inputOK := value.(map[string]interface{}); inputOK {
				base[key] = mergeValues(baseKeyAsMap, inputAsMap)
			}
		}
	}
	return base
}

func (v Ver) getSMCPProfile(name string, targetNamespace string) (*v1.ControlPlaneSpec, []string, error) {
	if strings.Contains(name, "/") {
		return nil, nil, fmt.Errorf("profile name contains invalid character '/'")
	}

	profileContent, err := os.ReadFile(path.Join(v.GetUserTemplatesDir(), name))
	if err != nil {
		// if we can't read from the user profile path, try from the default path
		// we use two paths because Kubernetes will not auto-update volume mounted
		// configmaps mounted in directories with pre-existing content
		defaultProfileContent, defaultErr := os.ReadFile(path.Join(v.GetDefaultTemplatesDir(), name))
		if defaultErr != nil {
			return nil, nil, fmt.Errorf("template cannot be loaded from user or default directory. Error from user: %s. Error from default: %s", err, defaultErr)
		}
		profileContent = defaultProfileContent
	}

	obj, gvk, err := decoder.Decode(profileContent, nil, nil)
	if err != nil || gvk == nil {
		return nil, nil, fmt.Errorf("failed to parse profile %s contents: %s", name, err)
	}

	switch smcp := obj.(type) {
	case *v1.ServiceMeshControlPlane:
		// ensure version is set so conversion works correctly
		smcp.Spec.Version = v.String()

		if len(smcp.Spec.Profiles) == 0 {
			if smcp.Spec.Template == "" {
				return &smcp.Spec, nil, nil
			}
			return &smcp.Spec, []string{smcp.Spec.Template}, nil
		}
		return &smcp.Spec, smcp.Spec.Profiles, nil
	case *v2.ServiceMeshControlPlane:
		// ensure version is set so conversion works correctly
		smcp.Spec.Version = v.String()

		smcpv1 := &v1.ServiceMeshControlPlane{}
		smcp.SetNamespace(targetNamespace)
		if err := smcpv1.ConvertFrom(smcp); err != nil {
			return nil, nil, err
		}
		return &smcpv1.Spec, smcp.Spec.Profiles, nil
	default:
		return nil, nil, fmt.Errorf("unsupported ServiceMeshControlPlane version: %s", gvk.String())
	}
}

// renderSMCPTemplates traverses and processes all of the references templates
func (v Ver) recursivelyApplyProfiles(
	ctx context.Context, smcp *v1.ControlPlaneSpec, targetNamespace string, profiles []string, visited sets.String,
) (v1.ControlPlaneSpec, error) {
	log := common.LogFromContext(ctx)

	for index := len(profiles) - 1; index >= 0; index-- {
		profileName := profiles[index]
		if visited.Has(profileName) {
			log.Info(fmt.Sprintf("smcp profile %s has already been applied", profileName))
			continue
		}
		log.Info(fmt.Sprintf("processing smcp profile %s", profileName))

		profile, profiles, err := v.getSMCPProfile(profileName, targetNamespace)
		if err != nil {
			return *smcp, err
		}

		if log.V(5).Enabled() {
			rawValues, _ := yaml.Marshal(profile)
			log.V(5).Info(fmt.Sprintf("profile values:\n%s\n", string(rawValues)))
			rawValues, _ = yaml.Marshal(smcp)
			log.V(5).Info(fmt.Sprintf("before applying profile values:\n%s\n", string(rawValues)))
		}

		// apply this profile first, then its children
		smcp.Istio = v1.NewHelmValues(mergeValues(smcp.Istio.GetContent(), profile.Istio.GetContent()))
		smcp.ThreeScale = v1.NewHelmValues(mergeValues(smcp.ThreeScale.GetContent(), profile.ThreeScale.GetContent()))

		if log.V(5).Enabled() {
			rawValues, _ := yaml.Marshal(smcp)
			log.V(5).Info(fmt.Sprintf("after applying profile values:\n%s\n", string(rawValues)))
		}

		*smcp, err = v.recursivelyApplyProfiles(ctx, smcp, targetNamespace, profiles, visited)
		if err != nil {
			log.Info(fmt.Sprintf("error applying profiles: %s\n", err))
			return *smcp, err
		}
	}

	return *smcp, nil
}

func (v Ver) updateImagesWithSHAs(ctx context.Context, cr *common.ControllerResources, smcpSpec v1.ControlPlaneSpec) (v1.ControlPlaneSpec, error) {
	log := common.LogFromContext(ctx)
	log.Info("updating image names for disconnected install")

	var err error
	if err = v.Strategy().SetImageValues(ctx, cr, &smcpSpec); err != nil {
		return smcpSpec, err
	}
	err = updateOauthProxyConfig(ctx, cr, &smcpSpec)
	return smcpSpec, err
}

func updateOauthProxyConfig(ctx context.Context, cr *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec) error {
	if !common.Config.OAuthProxy.Query || len(common.Config.OAuthProxy.Name) == 0 || len(common.Config.OAuthProxy.Namespace) == 0 {
		return nil
	}
	log := common.LogFromContext(ctx)
	is := &imagev1.ImageStream{}
	if err := cr.Client.Get(ctx, client.ObjectKey{Namespace: common.Config.OAuthProxy.Namespace, Name: common.Config.OAuthProxy.Name}, is); err == nil {
		foundTag := false
		for _, tag := range is.Status.Tags {
			if tag.Tag == common.Config.OAuthProxy.Tag {
				foundTag = true
				if len(tag.Items) > 0 && len(tag.Items[0].DockerImageReference) > 0 {
					common.Config.OAuthProxy.Image = tag.Items[0].DockerImageReference
				} else {
					log.Info(fmt.Sprintf("warning: dockerImageReference not set for tag '%s' in ImageStream %s/%s",
						common.Config.OAuthProxy.Tag, common.Config.OAuthProxy.Namespace, common.Config.OAuthProxy.Name))
				}
				break
			}
		}
		if !foundTag {
			log.Info(fmt.Sprintf("warning: could not find tag '%s' in ImageStream %s/%s",
				common.Config.OAuthProxy.Tag, common.Config.OAuthProxy.Namespace, common.Config.OAuthProxy.Name))
		}
	} else if !apierrors.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("unexpected error retrieving ImageStream %s/%s", common.Config.OAuthProxy.Namespace, common.Config.OAuthProxy.Name))
	}
	if len(common.Config.OAuthProxy.Image) == 0 {
		log.Info("global.oauthproxy.image will not be overridden")
		return nil
	}
	log.Info(fmt.Sprintf("using '%s' for global.oauthproxy.image", common.Config.OAuthProxy.Image))
	return updateImageField(smcpSpec.Istio, "global.oauthproxy.image", common.Config.OAuthProxy.Image)
}

func updateImageField(helmValues *v1.HelmValues, path, value string) error {
	if len(value) == 0 {
		return nil
	}
	return helmValues.SetField(path, value)
}

func (v Ver) ApplyProfiles(ctx context.Context, cr *common.ControllerResources,
	smcpSpec *v1.ControlPlaneSpec, targetNamespace string,
) (v1.ControlPlaneSpec, error) {
	log := common.LogFromContext(ctx)
	log.Info("applying profiles to ServiceMeshControlPlane")
	profiles := smcpSpec.Profiles
	if len(profiles) == 0 {
		if smcpSpec.Template == "" {
			profiles = []string{v1.DefaultTemplate}
			log.Info("No profiles specified, applying default profile")
		} else {
			profiles = []string{smcpSpec.Template}
		}
	}

	if smcpSpec.Istio == nil {
		smcpSpec.Istio = v1.NewHelmValues(make(map[string]interface{}))
	}
	if smcpSpec.ThreeScale == nil {
		smcpSpec.ThreeScale = v1.NewHelmValues(make(map[string]interface{}))
	}

	applyDisconnectedSettings := true
	if tag, _, _ := smcpSpec.Istio.GetString("global.tag"); tag != "" {
		// don't update anything
		applyDisconnectedSettings = false
	} else if hub, _, _ := smcpSpec.Istio.GetString("global.hub"); hub != "" {
		// don't update anything
		applyDisconnectedSettings = false
	}

	spec, err := v.recursivelyApplyProfiles(ctx, smcpSpec, targetNamespace, profiles, sets.NewString())
	if err != nil {
		return spec, err
	}

	if applyDisconnectedSettings {
		spec, err = v.updateImagesWithSHAs(ctx, cr, spec)
		if err != nil {
			log.Error(err, "warning: failed to apply image names to support disconnected install")

			return spec, err
		}
	}

	log.Info("finished updating ServiceMeshControlPlane", "Spec", spec)

	return spec, err
}

func isEnabled(spec *v1.HelmValues) bool {
	if enabled, found, _ := spec.GetBool("enabled"); found {
		return enabled
	}
	return false
}

func isComponentEnabled(spec *v1.HelmValues, path string) bool {
	if enabled, found, _ := spec.GetBool(path + ".enabled"); found {
		return enabled
	}
	return false
}
