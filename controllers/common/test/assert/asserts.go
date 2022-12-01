package assert

import (
	"reflect"
	"testing"
)

func Nil(actual interface{}, message string, t *testing.T) {
	t.Helper()
	if actual != nil {
		t.Fatalf("%s.\nExpected: nil\n  actual: %v", message, actual)
	}
}

func True(actual bool, message string, t *testing.T) {
	t.Helper()
	if !actual {
		t.Fatal(message)
	}
}

func False(actual bool, message string, t *testing.T) {
	t.Helper()
	True(!actual, message, t)
}

func Success(err error, functionName string, t *testing.T) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected %s to succeed, but it failed: %v", functionName, err)
	}
}

func Failure(err error, functionName string, t *testing.T) {
	t.Helper()
	if err == nil {
		t.Fatalf("Expected %s to fail, but it didn't", functionName)
	}
}

func Equals(actual interface{}, expected interface{}, message string, t *testing.T) {
	t.Helper()
	if actual != expected {
		t.Fatalf("%s.\nExpected: %v\n  actual: %v", message, expected, actual)
	}
}

func DeepEquals(actual interface{}, expected interface{}, message string, t *testing.T) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%s.\nExpected: %#v\n  actual: %#v", message, expected, actual)
	}
}

func StringArrayEmpty(actual []string, message string, t *testing.T) {
	t.Helper()
	if len(actual) != 0 {
		t.Fatalf("%s. Actual: %v", message, actual)
	}
}

func StringArrayNotEmpty(actual []string, message string, t *testing.T) {
	t.Helper()
	if len(actual) == 0 {
		t.Fatalf("%s. Actual: %v", message, actual)
	}
}

func StringArrayContains(actual []string, containedString, message string, t *testing.T) {
	t.Helper()
	for _, str := range actual {
		if str == containedString {
			return
		}
	}
	t.Fatalf("%s: %s not found in %v", message, containedString, actual)
}
