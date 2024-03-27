package matchers

import (
	"fmt"
	"reflect"

	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HaveConditionMatcher checks for a specific condition and status in a Kubernetes object.
type HaveConditionMatcher struct {
	conditionType      string
	conditionStatus    metav1.ConditionStatus
	lastSeenConditions []string // To store the last seen conditions for error reporting
}

// HaveCondition creates a new HaveConditionMatcher.
func HaveCondition[T ~string](conditionType T, conditionStatus metav1.ConditionStatus) types.GomegaMatcher {
	return &HaveConditionMatcher{
		conditionType:   string(conditionType),
		conditionStatus: conditionStatus,
	}
}

// Match checks if the actual object has the specified condition and status.
func (matcher *HaveConditionMatcher) Match(actual interface{}) (success bool, err error) {
	matcher.lastSeenConditions = []string{}

	val := reflect.ValueOf(actual)
	if val.Kind() == reflect.Ptr && !val.IsNil() {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return false, fmt.Errorf("expected a struct but got a %s; the object might be empty or not correctly passed", val.Kind())
	}

	status := val.FieldByName("Status")
	if !status.IsValid() {
		return false, fmt.Errorf("'Status' field not found in the object")
	}

	conditions := status.FieldByName("Conditions")
	if conditions.Kind() != reflect.Slice {
		return false, fmt.Errorf("'Conditions' is not a slice")
	}

	for i := 0; i < conditions.Len(); i++ {
		condition := conditions.Index(i).Interface()

		// Assuming the condition is of a type that has Type and Status fields
		// Adjust this part if your condition items are of a different type
		conditionVal := reflect.ValueOf(condition)
		if conditionVal.Kind() != reflect.Struct {
			continue // Skip if it's not a struct; this shouldn't happen
		}

		typeField := conditionVal.FieldByName("Type")
		statusField := conditionVal.FieldByName("Status")

		// Record the condition's current state for error reporting
		if typeField.IsValid() && statusField.IsValid() {
			matcher.lastSeenConditions = append(matcher.lastSeenConditions, fmt.Sprintf("%s: %s", typeField, statusField))
		}

		if typeField.IsValid() && statusField.IsValid() &&
			typeField.String() == matcher.conditionType &&
			statusField.String() == string(matcher.conditionStatus) {
			return true, nil
		}
	}

	// If we get here, no matching condition was found
	return false, nil
}

// FailureMessage is the message returned on matcher failure.
func (matcher *HaveConditionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected object to have condition %s with status %s but last seen conditions were: %v",
		matcher.conditionType, matcher.conditionStatus, matcher.lastSeenConditions)
}

// NegatedFailureMessage is the message returned on negated matcher failure.
func (matcher *HaveConditionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected object not to have condition %s with status %s", matcher.conditionType, matcher.conditionStatus)
}
