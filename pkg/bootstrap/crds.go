package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/helm/pkg/releaseutil"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/hacks"
)

var (
	crdMutex        sync.Mutex // ensure two workers don't deploy CRDs at same time
	badVersionRegex *regexp.Regexp
)

func init() {
	badVersionRegex = regexp.MustCompile(`^(v?)([0-9]+\.[0-9]+\.[0-9]+)(\.([0-9]+))$`)
}

// InstallCRDs makes sure all CRDs from the specified chartsDir have been
// installed. CRDs are loaded from chartsDir/istio-init/files
func InstallCRDs(ctx context.Context, cl client.Client, chartsDir string) error {
	// we only try to install them once.  if there's an error, we should probably
	// panic, as there's no way to recover.  for now, we just pass the error along.
	crdMutex.Lock()
	defer crdMutex.Unlock()

	log := common.LogFromContext(ctx)
	log.Info("ensuring CRDs are installed")
	crdPath := path.Join(chartsDir, "istio-init", "files")
	crdDir, err := os.Stat(crdPath)
	if err != nil || !crdDir.IsDir() {
		return fmt.Errorf("cannot locate any CRD files in %s", crdPath)
	}
	err = filepath.Walk(crdPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		return processCRDFile(ctx, cl, path)
	})
	if err != nil {
		return err
	}

	return installCRDRole(ctx, cl)
}

func installCRDRole(ctx context.Context, cl client.Client) error {
	aggregateRoles := []struct {
		role  string
		verbs []string
	}{
		{
			role:  "admin",
			verbs: []string{rbacv1.VerbAll},
		},
		{
			role: "edit",
			verbs: []string{
				"create",
				"update",
				"patch",
				"delete",
			},
		},
		{
			role: "view",
			verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
	for _, role := range aggregateRoles {
		crdRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("istio-%s", role.role),
				Labels: map[string]string{
					fmt.Sprintf("rbac.authorization.k8s.io/aggregate-to-%s", role.role): "true",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{
						"authentication.istio.io",
						"config.istio.io",
						"networking.istio.io",
						"rbac.istio.io",
						"security.istio.io",
						"authentication.maistra.io",
						"rbac.maistra.io",
					},
					Resources: []string{rbacv1.ResourceAll},
					Verbs:     role.verbs,
				},
				{
					APIGroups: []string{
						"maistra.io",
					},
					Resources: []string{
						"servicemeshextensions",
					},
					Verbs: role.verbs,
				},
			},
		}
		existingRole := crdRole.DeepCopy()
		if err := cl.Get(ctx, client.ObjectKey{Name: crdRole.Name}, existingRole); err == nil {
			if !reflect.DeepEqual(existingRole.Rules, crdRole.Rules) {
				existingRole.Rules = crdRole.Rules
				if err := cl.Update(ctx, existingRole); err != nil {
					return err
				}
			}
		} else if errors.IsNotFound(err) {
			if err := cl.Create(ctx, crdRole); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func processCRDFile(ctx context.Context, cl client.Client, fileName string) error {
	log := common.LogFromContext(ctx)
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := &bytes.Buffer{}
	_, err = buf.ReadFrom(file)
	if err != nil {
		return err
	}

	allErrors := []error{}
	for index, raw := range releaseutil.SplitManifests(buf.String()) {
		crd, err := decodeCRD(common.NewContextWithLog(ctx, log.WithValues("file", fileName, "index", index)), raw)
		if err != nil {
			allErrors = append(allErrors, err)
		} else if crd != nil { // crd is nil when the object in the file isn't a CRD
			err = createCRD(common.NewContextWithLog(ctx, log.WithValues("CRD", crd.GetName())), cl, crd)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
	}
	return utilerrors.NewAggregate(allErrors)
}

func decodeCRD(ctx context.Context, raw string) (*apiextensionsv1.CustomResourceDefinition, error) {
	log := common.LogFromContext(ctx)
	rawJSON, err := yaml.YAMLToJSON([]byte(raw))
	if err != nil {
		log.Error(err, "unable to convert raw data to JSON")
		return nil, err
	}
	v1beta1obj := &apiextensionsv1beta1.CustomResourceDefinition{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(rawJSON, nil, v1beta1obj)
	if err != nil {
		v1obj := &apiextensionsv1.CustomResourceDefinition{}
		_, _, err = unstructured.UnstructuredJSONScheme.Decode(rawJSON, nil, v1obj)
		if err != nil {
			log.Error(err, "unable to decode object into CustomResourceDefinition")
			return nil, err
		}
		if v1obj.GroupVersionKind().GroupKind().String() == "CustomResourceDefinition.apiextensions.k8s.io" {
			return v1obj, nil
		}
		log.Error(err, "decoded object is not a CustomResourceDefinition: %s", v1obj.GroupVersionKind().String())
		return nil, nil
	}
	if v1beta1obj.GroupVersionKind().GroupKind().String() != "CustomResourceDefinition.apiextensions.k8s.io" {
		log.Error(err, "decoded object is not a CustomResourceDefinition: %s", v1beta1obj.GroupVersionKind().String())
		return nil, nil
	}

	// make sure we have an object that will result in a valid v1 object after conversion
	hacks.PatchUpV1beta1CRDs(v1beta1obj)

	internalobj := &apiextensions.CustomResourceDefinition{}
	err = apiextensionsv1beta1.Convert_v1beta1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(v1beta1obj, internalobj, nil)
	if err != nil {
		return nil, err
	}
	v1obj := &apiextensionsv1.CustomResourceDefinition{}
	err = apiextensionsv1.Convert_apiextensions_CustomResourceDefinition_To_v1_CustomResourceDefinition(internalobj, v1obj, nil)
	if err != nil {
		return nil, err
	}
	hacks.PreserveUnknownFields(v1obj)
	return v1obj, nil
}

func createCRD(ctx context.Context, cl client.Client, crd *apiextensionsv1.CustomResourceDefinition) error {
	log := common.LogFromContext(ctx)
	existingCrd := &apiextensionsv1.CustomResourceDefinition{}
	existingCrd.SetName(crd.GetName())
	err := cl.Get(ctx, client.ObjectKey{Name: crd.GetName()}, existingCrd)
	if err == nil {
		if isManagedByOLM(existingCrd) {
			log.Info("CRD is managed by OLM, skipping update")
			return nil
		}
		newVersion, err := getMaistraVersion(crd)
		if err != nil {
			return fmt.Errorf("could not determine version of new CRD %s: %v", crd.GetName(), err)
		}
		existingVersion, err := getMaistraVersion(existingCrd)
		if err != nil {
			log.Info("Could not determine version of existing CRD", "error", err)
			existingVersion = nil
		}
		if existingVersion == nil || existingVersion.LessThan(newVersion) {
			log.Info("CRD exists, but is old or has no version label. Replacing with newer version.")

			crd.ResourceVersion = existingCrd.ResourceVersion
			err = cl.Update(ctx, crd)
			if err != nil {
				log.Error(err, "error updating CRD")
				return err
			}

		} else {
			log.V(2).Info("CRD exists")
		}
		return nil

	} else if errors.IsNotFound(err) {
		log.Info("creating CRD")
		err = cl.Create(ctx, crd)
		if err != nil {
			log.Error(err, "error creating CRD")
			return err
		}
		return nil
	}
	return err
}

func isManagedByOLM(crd *apiextensionsv1.CustomResourceDefinition) bool {
	return crd.Labels["olm.managed"] == "true"
}

func getMaistraVersion(crd *apiextensionsv1.CustomResourceDefinition) (*semver.Version, error) {
	versionLabel := crd.Labels["maistra-version"]
	if versionLabel == "" {
		return nil, fmt.Errorf("label maistra-version not found")
	}
	versionLabel = badVersionRegex.ReplaceAllString(versionLabel, "$1$2-$4")
	if !strings.Contains(versionLabel, "-") {
		// for proper comparisons, all versions must have - suffix
		versionLabel += "-0"
	}
	return semver.NewVersion(versionLabel)
}
