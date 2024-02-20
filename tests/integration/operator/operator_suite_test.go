package integration_operator

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Install Operator Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")
	if ocp == "true" {
		GinkgoWriter.Println("Running on OCP cluster")
		GinkgoWriter.Printf("Absolute Path: %s", wd)
		//TODO: Add there more setup code specific to OpenShift if there is any
	} else {
		GinkgoWriter.Println("Running on Kubernetes")
		GinkgoWriter.Printf("Absolute Path: %s", wd)
		//TODO: Add there more setup code specific to Kubernetes if there is any
	}

}
