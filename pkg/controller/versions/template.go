package versions

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/ghodss/yaml"
	imagev1 "github.com/openshift/api/image/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

// GetChartsDir returns the location of the Helm charts. Similar layout to istio.io/istio/install/kubernetes/helm.
func (v version) GetChartsDir() string {
	if len(common.Config.Rendering.ChartsDir) == 0 {
		return path.Join(common.Config.Rendering.ResourceDir, "helm", v.String())
	}
	return path.Join(common.Config.Rendering.ChartsDir, v.String())
}

// GetTemplatesDir returns the location of the Operator templates files
func (v version) GetUserTemplatesDir() string {
	if len(common.Config.Rendering.UserTemplatesDir) == 0 {
		return path.Join(common.Config.Rendering.ResourceDir, "templates")
	}
	return common.Config.Rendering.UserTemplatesDir
}

// GetDefaultTemplatesDir returns the location of the Default Operator templates files
func (v version) GetDefaultTemplatesDir() string {
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

func (v version) getSMCPTemplate(name string) (v1.ControlPlaneSpec, error) {
	if strings.Contains(name, "/") {
		return v1.ControlPlaneSpec{}, fmt.Errorf("template name contains invalid character '/'")
	}

	templateContent, err := ioutil.ReadFile(path.Join(v.GetUserTemplatesDir(), name))
	if err != nil {
		//if we can't read from the user template path, try from the default path
		//we use two paths because Kubernetes will not auto-update volume mounted
		//configmaps mounted in directories with pre-existing content
		defaultTemplateContent, defaultErr := ioutil.ReadFile(path.Join(v.GetDefaultTemplatesDir(), name))
		if defaultErr != nil {
			return v1.ControlPlaneSpec{}, fmt.Errorf("template cannot be loaded from user or default directory. Error from user: %s. Error from default: %s", err, defaultErr)
		}
		templateContent = defaultTemplateContent
	}

	var template v1.ServiceMeshControlPlane
	if err = yaml.Unmarshal(templateContent, &template); err != nil {
		return v1.ControlPlaneSpec{}, fmt.Errorf("failed to parse template %s contents: %s", name, err)
	}
	return template.Spec, nil
}

//renderSMCPTemplates traverses and processes all of the references templates
func (v version) recursivelyApplyTemplates(ctx context.Context, smcp v1.ControlPlaneSpec, visited sets.String) (v1.ControlPlaneSpec, error) {
	log := common.LogFromContext(ctx)
	if smcp.Template == "" {
		return smcp, nil
	}
	log.Info(fmt.Sprintf("processing smcp template %s", smcp.Template))

	if visited.Has(smcp.Template) {
		return smcp, fmt.Errorf("SMCP templates form cyclic dependency. Cannot proceed")
	}

	template, err := v.getSMCPTemplate(smcp.Template)
	if err != nil {
		return smcp, err
	}

	template, err = v.recursivelyApplyTemplates(ctx, template, visited)
	if err != nil {
		log.Info(fmt.Sprintf("error rendering SMCP templates: %s\n", err))
		return smcp, err
	}

	visited.Insert(smcp.Template)

	smcp.Istio = v1.NewHelmValues(mergeValues(smcp.Istio.GetContent(), template.Istio.GetContent()))
	smcp.ThreeScale = v1.NewHelmValues(mergeValues(smcp.ThreeScale.GetContent(), template.ThreeScale.GetContent()))
	return smcp, nil
}

func (v version) updateImagesWithSHAs(ctx context.Context, cr *common.ControllerResources, smcpSpec v1.ControlPlaneSpec) (v1.ControlPlaneSpec, error) {
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
					log.Info(fmt.Sprintf("warning: dockerImageReference not set for tag '%s' in ImageStream %s/%s", common.Config.OAuthProxy.Tag, common.Config.OAuthProxy.Namespace, common.Config.OAuthProxy.Name))
				}
				break
			}
		}
		if !foundTag {
			log.Info(fmt.Sprintf("warning: could not find tag '%s' in ImageStream %s/%s", common.Config.OAuthProxy.Tag, common.Config.OAuthProxy.Namespace, common.Config.OAuthProxy.Name))
		}
	} else if !(apierrors.IsNotFound(err) || apierrors.IsGone(err)) {
		log.Error(err, fmt.Sprintf("unexpected error retrieving ImageStream %s/%s", common.Config.OAuthProxy.Namespace, common.Config.OAuthProxy.Name))
	}
	if len(common.Config.OAuthProxy.Image) == 0 {
		log.Info("global.oauthproxy.image will not be overridden")
		return nil
	}
	log.Info(fmt.Sprintf("using '%s' for global.oauthproxy.image", common.Config.OAuthProxy.Image))
	updateImageField(smcpSpec.Istio, "global.oauthproxy.image", common.Config.OAuthProxy.Image)
	return nil
}

func updateImageField(helmValues *v1.HelmValues, path, value string) error {
	if len(value) == 0 {
		return nil
	}
	return helmValues.SetField(path, value)
}

func (v version) applyTemplates(ctx context.Context, cr *common.ControllerResources, smcpSpec v1.ControlPlaneSpec) (v1.ControlPlaneSpec, error) {
	log := common.LogFromContext(ctx)
	log.Info("updating servicemeshcontrolplane with templates")
	if smcpSpec.Template == "" {
		smcpSpec.Template = v1.DefaultTemplate
		log.Info("No template provided. Using default")
	}

	applyDisconnectedSettings := true
	if tag, _, _ := smcpSpec.Istio.GetString("global.tag"); tag != "" {
		// don't update anything
		applyDisconnectedSettings = false
	} else if hub, _, _ := smcpSpec.Istio.GetString("global.hub"); hub != "" {
		// don't update anything
		applyDisconnectedSettings = false
	}

	spec, err := v.recursivelyApplyTemplates(ctx, smcpSpec, sets.NewString())

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
