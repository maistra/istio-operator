package ginkgo_utils

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

// Success func will print the success message using the provided message
// Arguments:
// - string: message to be printed
func Success(message string) {
	GinkgoWriter.Println(fmt.Sprintf("* %s", message))
}
