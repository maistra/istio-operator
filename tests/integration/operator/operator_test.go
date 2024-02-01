package installation_test

import (
	"bytes"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Operator", func() {
	BeforeEach(func() {
		// Add there code to run before each test
	})

	It("can be installed", func() {
		// Install the operator here with default profile
		By("using the helm chart with default values")
		GinkgoWriter.Print("Deploying Operator using default helm charts located in /chart folder\n")
		deployOperator()
		Expect("Operator Running").To(Equal("Hugo"))
	})

	It("can be unistalled", func() {

		Expect("Operator Uninstalled").To(Equal("Hugo"))
	})
})

func deployOperator() {
	if ocp == "true" {
		GinkgoWriter.Print("Deploying to OpenShift cluster\n")
		deployOpenShift()
	} else {
		GinkgoWriter.Print("Deploying to Kubernetes cluster\n")
		deployKubernetes()
	}
}

func deployKubernetes() error {
	// Generate deployment manifests
	cmd := exec.Command("kustomize", "build", "config/default")
	output, err := cmd.Output()
	print("******** Output ********\n")
	print(output)
	if err != nil {
		return err
	}

	// Apply deployment manifests to Kubernetes cluster
	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewBuffer(output)
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func deployOpenShift() error {

	if err := setControllerImage(); err != nil {
		fmt.Printf("Error setting controller image: %v\n", err)
		return err
	}

	if err := setNamespace(); err != nil {
		fmt.Printf("Error setting namespace: %v\n", err)
		return err
	}

	if err := applyYAMLWithKustomize(); err != nil {
		fmt.Printf("Error applying YAML manifests: %v\n", err)
		return err
	}

	return nil
}

func applyYAMLWithKustomize() error {
	output, err := exec.Command("kubectl", "apply", "-k", "../../../config/openshift").CombinedOutput()
	if err != nil {
		fmt.Printf("Error applying YAML manifests: %v\n", err)
		return err
	}
	fmt.Println(string(output))
	return nil
}

func setControllerImage() error {
	print("Setting Controller Image with Kustomize\n")
	cmd := exec.Command("kustomize", "edit", "set", "image", fmt.Sprintf("controller=%s", image))
	cmd.Dir = "../../../config/manager"
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error setting namespace in YAML manifests: %v\n", err)
		return err
	}

	fmt.Println(string(output))

	return nil
}

func setNamespace() error {
	print("Setting Namespace to be replaced\n")
	cmd := exec.Command("kustomize", "edit", "set", "namespace", namespace)
	cmd.Dir = "../../../config/default"
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error setting namespace in YAML manifests: %v\n", err)
		return err
	}

	fmt.Println(string(output))

	return nil
}
