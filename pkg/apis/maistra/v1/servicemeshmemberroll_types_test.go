package v1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsMember(t *testing.T) {
	systemNamespaces := []metav1.ObjectMeta{
		{Name: "kube"},
		{Name: "kube-something"},
		{Name: "openshift"},
		{Name: "openshift-something"},
		{Name: "ibm-something"},
	}

	cases := []struct {
		name                string
		spec                ServiceMeshMemberRollSpec
		matchedNamespaces   []metav1.ObjectMeta
		unmatchedNamespaces []metav1.ObjectMeta
	}{
		{
			name: "no members no selector matches nothing",
			spec: ServiceMeshMemberRollSpec{},
			unmatchedNamespaces: []metav1.ObjectMeta{
				{Name: "ns1"},
			},
		},
		{
			name: "empty members array matches nothing",
			spec: ServiceMeshMemberRollSpec{
				Members: []string{},
			},
			unmatchedNamespaces: append(systemNamespaces, metav1.ObjectMeta{Name: "ns1"}),
		},
		{
			name: "empty selectors array matches nothing",
			spec: ServiceMeshMemberRollSpec{
				MemberSelectors: []metav1.LabelSelector{},
			},
			unmatchedNamespaces: append(systemNamespaces, metav1.ObjectMeta{Name: "ns1"}),
		},
		{
			name: "members asterisk matches all namespaces except system",
			spec: ServiceMeshMemberRollSpec{
				Members: []string{"*"},
			},
			matchedNamespaces: []metav1.ObjectMeta{
				{Name: "ns1"},
				{Name: "ns2"},
				{Name: "ns3"},
			},
			unmatchedNamespaces: systemNamespaces,
		},
		{
			name: "empty selector matches all namespaces except system",
			spec: ServiceMeshMemberRollSpec{
				MemberSelectors: []metav1.LabelSelector{
					{},
				},
			},
			matchedNamespaces: []metav1.ObjectMeta{
				{Name: "ns1"},
				{Name: "ns2"},
				{Name: "ns3"},
			},
			unmatchedNamespaces: systemNamespaces,
		},
		{
			name: "match name",
			spec: ServiceMeshMemberRollSpec{
				Members: []string{"ns1", "ns2"},
			},
			matchedNamespaces: []metav1.ObjectMeta{
				{Name: "ns1"},
				{Name: "ns2"},
			},
			unmatchedNamespaces: []metav1.ObjectMeta{
				{Name: "ns3"},
			},
		},
		{
			name: "match selector",
			spec: ServiceMeshMemberRollSpec{
				MemberSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"foo": "bar"}},
				},
			},
			matchedNamespaces: []metav1.ObjectMeta{
				{Name: "ns1", Labels: map[string]string{"foo": "bar"}},
				{Name: "ns2", Labels: map[string]string{"foo": "bar"}},
			},
			unmatchedNamespaces: []metav1.ObjectMeta{
				{Name: "ns3", Labels: map[string]string{"foo": "foo"}},
			},
		},
		{
			name: "selectors are ORed",
			spec: ServiceMeshMemberRollSpec{
				MemberSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"foo": "123"}},
					{MatchLabels: map[string]string{"baz": "456"}},
				},
			},
			matchedNamespaces: []metav1.ObjectMeta{
				{Name: "ns1", Labels: map[string]string{"foo": "123"}},
				{Name: "ns2", Labels: map[string]string{"baz": "456"}},
			},
		},
		{
			name: "system namespaces excluded even if specified in members",
			spec: ServiceMeshMemberRollSpec{
				Members: []string{
					"kube", "kube-system", "openshift", "openshift-something", "ibm-something",
				},
			},
			unmatchedNamespaces: systemNamespaces,
		},
		{
			name: "system namespaces excluded even if matching selector",
			spec: ServiceMeshMemberRollSpec{
				MemberSelectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{"foo": "bar"},
					},
				},
			},
			unmatchedNamespaces: addLabels(systemNamespaces, map[string]string{"foo": "bar"}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			smmr := ServiceMeshMemberRoll{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
				Spec:       tc.spec,
			}

			for _, nsMeta := range tc.matchedNamespaces {
				ns := &corev1.Namespace{ObjectMeta: nsMeta}
				if !smmr.IsMember(ns) {
					t.Fatalf("expected namespace %q with labels %v to match, but it didn't", nsMeta.Name, nsMeta.Labels)
				}
			}

			for _, nsMeta := range tc.unmatchedNamespaces {
				ns := &corev1.Namespace{ObjectMeta: nsMeta}
				if smmr.IsMember(ns) {
					t.Fatalf("expected namespace %q with labels %v not to match, but it did", nsMeta.Name, nsMeta.Labels)
				}
			}
		})
	}
}

func addLabels(namespaces []metav1.ObjectMeta, labels map[string]string) []metav1.ObjectMeta {
	labelledNamespaces := []metav1.ObjectMeta{}
	for _, ns := range namespaces {
		ns = *ns.DeepCopy()
		ns.Labels = labels
		labelledNamespaces = append(labelledNamespaces, ns)
	}
	return labelledNamespaces
}

func TestSelectorMatches(t *testing.T) {
	// test case with empty selector
	t.Run("empty selector", func(t *testing.T) {
		if !selectorMatches(metav1.LabelSelector{}, map[string]string{}) {
			t.Errorf("Expected selectorMatches to return true for empty selector but got false")
		}
	})

	// other test cases
	selector := metav1.LabelSelector{
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
