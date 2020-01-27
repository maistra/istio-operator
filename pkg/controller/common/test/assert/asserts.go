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

func Success(err error, functionName string, t *testing.T) {
	if err != nil {
		t.Fatalf("Expected %s to succeed, but it failed: %v", functionName, err)
	}
}

func Failure(err error, functionName string, t *testing.T) {
	if err == nil {
		t.Fatalf("Expected %s to fail, but it didn't", functionName)
	}
}

func Equals(actual interface{}, expected interface{}, message string, t *testing.T) {
	if actual != expected {
		t.Fatalf("%s.\nExpected: %v\n  actual: %v", message, expected, actual)
	}
}

func DeepEquals(actual interface{}, expected interface{}, message string, t *testing.T) {
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%s.\nExpected: %v\n  actual: %v", message, expected, actual)
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

func StringArrayContains(actual []string, containedString, message string, t *testing.T) {
	for _, str := range actual {
		if str == containedString {
			return
		}
	}
	t.Fatalf("%s: %s not found in %v", message, containedString, actual)
}
