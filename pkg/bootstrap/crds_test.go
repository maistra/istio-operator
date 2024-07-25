package bootstrap

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

var ctx = context.Background()

func TestMultipleFilesInDir(t *testing.T) {
	// Play nice with common.GetOperatorNamespace()
	os.Setenv("POD_NAMESPACE", "istio-operator")

	dir := createTempDirectoryWithCRDFiles(
		newCRDYAML("test", "1.0.7"),
		newCRDYAML("test2", "1.0.7"))
	defer deleteDir(dir)

	cl, _ := test.CreateClient()
	assert.Success(InstallCRDs(ctx, cl, dir), "InstallCRDs", t)

	crdNames := extractNames(listCRDs(cl))
	assert.DeepEquals(crdNames, sets.NewString("test", "test2"), "Unexpected CRDs", t)
}

func TestMultipleCRDsInSameFile(t *testing.T) {
	crdFileContents := newCRDYAML("test", "1.0.7") + "\n---\n" +
		newCRDYAML("test2", "1.0.7")

	dir := createTempDirectoryWithCRDFiles(crdFileContents)
	defer deleteDir(dir)

	cl, _ := test.CreateClient()
	assert.Success(InstallCRDs(ctx, cl, dir), "InstallCRDs", t)

	crdNames := extractNames(listCRDs(cl))
	assert.DeepEquals(crdNames, sets.NewString("test", "test2"), "Unexpected CRDs", t)
}

func TestNonCRDObjectsAreIgnored(t *testing.T) {
	file1 := newCRDYAML("test", "1.0.7") + "\n---\n" +
		newPodYAML("pod1")
	file2 := newPodYAML("pod2")
	dir := createTempDirectoryWithCRDFiles(file1, file2)
	defer deleteDir(dir)

	cl, tracker := test.CreateClient()
	assert.Success(InstallCRDs(ctx, cl, dir), "InstallCRDs", t)

	crdNames := extractNames(listCRDs(cl))
	assert.DeepEquals(crdNames, sets.NewString("test"), "Unexpected CRDs", t)

	test.AssertNumberOfWriteActions(t, tracker.Actions(), 4) // CRD and the three ClusterRoles, but nothing else
}

func TestAdminClusterRoleIsCreated(t *testing.T) {
	dir := createTempDirectoryWithCRDFiles(newCRDYAML("test", "1.0.7"))
	defer deleteDir(dir)

	cl, _ := test.CreateClient()
	assert.Success(InstallCRDs(ctx, cl, dir), "InstallCRDs", t)

	test.AssertObjectExists(ctx, cl,
		types.NamespacedName{Name: "istio-admin"},
		&rbacv1.ClusterRole{},
		"istio-admin ClusterRole was not created", t)
}

func TestNewerCRDVersionAlwaysWinsRegardlessDeploymentOrder(t *testing.T) {
	olderCRD := newCRDYAML("test", "1.0.7")
	olderBadCRD := newCRDYAML("test", "1.0.7.1")
	newerCRD := newCRDYAML("test", "1.1.0")
	noVersionCRD := newCRDYAML("test", "")

	testCases := []struct {
		name        string
		deployOrder []string
		newCRD      string
	}{
		{
			name:        "older-then-newer",
			deployOrder: []string{olderCRD, newerCRD},
			newCRD:      "1.1.0",
		},
		{
			name:        "newer-then-older",
			deployOrder: []string{newerCRD, olderCRD},
			newCRD:      "1.1.0",
		},
		{
			name:        "existing-without-version",
			deployOrder: []string{noVersionCRD, newerCRD},
			newCRD:      "1.1.0",
		},
		{
			name:        "newer-then-older-bad",
			deployOrder: []string{newerCRD, olderBadCRD},
			newCRD:      "1.1.0",
		},
		{
			name:        "newer-bad-then-older",
			deployOrder: []string{olderBadCRD, olderCRD},
			newCRD:      "1.0.7.1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl, _ := test.CreateClient()

			for _, crdFileContents := range tc.deployOrder {
				dir := createTempDirectoryWithCRDFiles(crdFileContents)
				defer deleteDir(dir)

				assert.Success(InstallCRDs(ctx, cl, dir), "InstallCRDs", t)
			}

			crdList := listCRDs(cl)
			assert.Equals(len(crdList.Items), 1, "Unexpected number of CRDs", t)

			crd := crdList.Items[0]
			assert.Equals(crd.Name, "test", "Unexpected CRD name", t)
			assert.Equals(crd.Labels["maistra-version"], tc.newCRD, "Expected newer version of CRD", t)
		})
	}
}

func TestSkipsCRDManagedByOLM(t *testing.T) {
	olmCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"olm.managed": "true",
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "istio.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind: "Test",
			},
			Scope: "Namespaced",
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "rbacv1",
				},
			},
		},
	}

	myCRD := newCRDYAML("test", "2.6.0")

	cl, tracker := test.CreateClient()
	tracker.AddReactor("update", "customresourcedefinitions", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Fatal("code tried to update the OLM-managed CRD")
		return false, nil, nil
	})
	assert.Success(cl.Create(ctx, olmCRD), "cl.Create", t)

	dir := createTempDirectoryWithCRDFiles(myCRD)
	defer deleteDir(dir)

	assert.Success(InstallCRDs(ctx, cl, dir), "InstallCRDs", t)

	crdList := listCRDs(cl)
	assert.Equals(len(crdList.Items), 1, "Unexpected number of CRDs", t)

	crd := crdList.Items[0]
	assert.Equals(crd.Name, "test", "Unexpected CRD name", t)
	assert.Equals(crd.Labels["maistra-version"], "", "Expected OLM-managed CRD to be preserved", t)
}

// Older versions of OpenShift/Kubernetes reject CRDs that contain the
// type:object field in the CRD's OpenAPI schema. This test checks if
// we remove these fields and retry the create/update operation.
func TestRemoveTypeObjectFromOpenAPISchema(t *testing.T) {
	t.Skip("maistra no longer supports running on OpenShift 3.11")
	CRDnoSchema10 := newCRDYAML("test", "1.0.0")
	CRDnoSchema11 := newCRDYAML("test", "1.1.0")
	CRDwithSchemaWithTypeObject := CRDnoSchema11 + `
  validation:
    openAPIV3Schema:
      properties:
        spec: 
          properties:
            simple:
              type: object
            ingress:
              type: array
              items:
              - type: object
                properties:
                  foo:
                    type: object
              additionalItems:
                type: object
            egress:
              type: array
              items:
                type: object
                properties:
                  bind:
                    format: string
                    type: string
                  port:
                    type: object
                    properties:
                      protocol:
                        format: string
                        type: string
          patternProperties:
            "^[A-Z]$":
              type: object
          allOf:
          - type: object
            properties:
              foo: 
                type: object
          anyOf:
          - type: object
            properties:
              foo: 
                type: object
          oneOf:
          - type: object
            properties:
              foo: 
                type: object
          not:
            type: object
            properties:
              foo: 
                type: object
      additionalProperties:
        type: object 
      definitions:
        foo:
          type: object 
      dependencies:
        foo:
          type: object 

`

	testCases := []struct {
		name        string
		deployOrder []string
	}{
		{
			name:        "create",
			deployOrder: []string{CRDwithSchemaWithTypeObject},
		},
		{
			name: "update",
			// must first deploy older version, otherwise the update won't be performed
			deployOrder: []string{CRDnoSchema10, CRDwithSchemaWithTypeObject},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl, tracker := test.CreateClient()
			crdRejected := false
			rejectTypeObject := func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				var obj runtime.Object
				switch a := action.(type) {
				case clienttesting.CreateAction: // clienttesting.UpdateAction also matches this case due to having the same signature
					obj = a.GetObject()
				}

				yml, err := yaml.Marshal(obj)
				test.PanicOnError(err)

				if strings.Contains(string(yml), "type: object") {
					crdRejected = true
					return true, nil, fmt.Errorf("invalid CRD schema: must only have \"properties\", "+
						"\"required\" or \"description\" at the root if the status subresource is enabled\n%s", yml)
				}
				return false, nil, nil
			}
			tracker.AddReactor("create", "customresourcedefinitions", rejectTypeObject)
			tracker.AddReactor("update", "customresourcedefinitions", rejectTypeObject)

			for _, crdFileContents := range tc.deployOrder {
				dir := createTempDirectoryWithCRDFiles(crdFileContents)
				defer deleteDir(dir)

				assert.Success(InstallCRDs(ctx, cl, dir), "InstallCRDs", t)
			}

			if !crdRejected {
				panic("test was supposed to force rejection of the CRD, but it didn't")
			}

			// CRD must exist despite being rejected the first time
			crdList := listCRDs(cl)
			assert.Equals(len(crdList.Items), 1, "CRD doesn't exist", t)

			crd := crdList.Items[0]
			assert.Equals(crd.Name, "test", "Unexpected CRD name", t)
		})
	}
}

func createTempDirectoryWithCRDFiles(crdFileContents ...string) (dirPath string) {
	dir, err := ioutil.TempDir("", "crds_test_charts")
	test.PanicOnError(err)

	istioInitFilesDir := path.Join(dir, "istio-init", "files")
	err = os.MkdirAll(istioInitFilesDir, os.ModePerm)
	test.PanicOnError(err)
	for i, contents := range crdFileContents {
		filename := fmt.Sprintf("crd-%d.yaml", i)
		err := ioutil.WriteFile(path.Join(istioInitFilesDir, filename), []byte(contents), os.ModePerm)
		test.PanicOnError(err)
	}
	return dir
}

func listCRDs(cl client.Client) apiextensionsv1.CustomResourceDefinitionList {
	crdList := apiextensionsv1.CustomResourceDefinitionList{}
	err := cl.List(ctx, &crdList, &client.ListOptions{})
	test.PanicOnError(err)
	return crdList
}

func extractNames(crdList apiextensionsv1.CustomResourceDefinitionList) sets.String {
	crdNames := sets.NewString()
	for _, crd := range crdList.Items {
		crdNames.Insert(crd.Name)
	}
	return crdNames
}

func deleteDir(dir string) {
	test.PanicOnError(os.RemoveAll(dir))
}

func newCRDYAML(name, maistraVersion string) string {
	template := `apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: %s
  labels:
    maistra-version: %s
spec:
  group: istio.io
  names:
    kind: Test
  scope: Namespaced
  version: rbacv1`

	return fmt.Sprintf(template, name, maistraVersion)
}

func newPodYAML(name string) string {
	template := `apiVersion: v1
kind: Pod
metadata:
  name: %s
spec:
  containers:
  - name: main
    image: my-image`

	return fmt.Sprintf(template, name)
}
