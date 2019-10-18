package assert

import (
	"reflect"
	"testing"
)

func True(actual bool, message string, t *testing.T) {
	if !actual {
		t.Fatal(message)
	}
}

func False(actual bool, message string, t *testing.T) {
	True(!actual, message, t)
}

func Equals(actual interface{}, expected interface{}, message string, t *testing.T) {
	if actual != expected {
		t.Fatalf("%s. Expected: %v, actual: %v", message, expected, actual)
	}
}

func DeepEquals(actual interface{}, expected interface{}, message string, t *testing.T) {
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%s. Expected: %v, actual: %v", message, expected, actual)
	}
}

func StringArrayEmpty(actual []string, message string, t *testing.T) {
	if len(actual) != 0 {
		t.Fatalf("%s. Actual: %v", message, actual)
	}
}

func StringArrayNotEmpty(actual []string, message string, t *testing.T) {
	if len(actual) == 0 {
		t.Fatalf("%s. Actual: %v", message, actual)
	}
}
