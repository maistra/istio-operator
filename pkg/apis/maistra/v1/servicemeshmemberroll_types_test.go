package v1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSelectorMatches(t *testing.T) {
	// test case with nil selector
	t.Run("nil selector", func(t *testing.T) {
		if selectorMatches(nil, map[string]string{}) {
			t.Errorf("Expected selectorMatches to return false for nil selector but got true")
		}
	})

	// test case with empty selector
	t.Run("empty selector", func(t *testing.T) {
		if !selectorMatches(&metav1.LabelSelector{}, map[string]string{}) {
			t.Errorf("Expected selectorMatches to return true for empty selector but got false")
		}
	})

	// other test cases
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "test",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "version",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"v1", "v2"},
			},
			{
				Key:      "owner",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"user1", "user2"},
			},
			{
				Key:      "team",
				Operator: metav1.LabelSelectorOpExists,
			},
			{
				Key:      "env",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			},
		},
	}

	testCases := []struct {
		name   string
		labels map[string]string
		expect bool
	}{
		{
			name: "non-matching MatchLabels",
			labels: map[string]string{
				"app": "test2",
			},
			expect: false,
		},
		{
			name: "matching labels with operator In",
			labels: map[string]string{
				"app":     "test",
				"version": "v1",
				"team":    "a",
			},
			expect: true,
		},
		{
			name: "non-matching labels with operator In",
			labels: map[string]string{
				"app":     "test",
				"version": "v3",
				"team":    "a",
			},
			expect: false,
		},
		{
			name: "matching labels with operator NotIn",
			labels: map[string]string{
				"app":     "test",
				"version": "v1",
				"owner":   "user3",
				"team":    "a",
			},
			expect: true,
		},
		{
			name: "non-matching labels with operator NotIn",
			labels: map[string]string{
				"app":     "test",
				"version": "v1",
				"owner":   "user1",
				"team":    "a",
			},
			expect: false,
		},
		{
			name: "matching labels with operator Exists",
			labels: map[string]string{
				"app":     "test",
				"version": "v1",
				"team":    "dev",
			},
			expect: true,
		},
		{
			name: "non-matching labels with operator Exists",
			labels: map[string]string{
				"app":     "test",
				"version": "v1",
			},
			expect: false,
		},
		{
			name: "matching labels with operator DoesNotExist",
			labels: map[string]string{
				"app":     "test",
				"version": "v1",
				"team":    "a",
			},
			expect: true,
		},
		{
			name: "non-matching labels with operator DoesNotExist",
			labels: map[string]string{
				"app":     "test",
				"version": "v1",
				"team":    "a",
				"env":     "prod",
			},
			expect: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if selectorMatches(selector, tc.labels) != tc.expect {
				t.Errorf("Expected selectorMatches to return %v for labels %+v, but got %v", tc.expect, tc.labels, !tc.expect)
			}
		})
	}
}
