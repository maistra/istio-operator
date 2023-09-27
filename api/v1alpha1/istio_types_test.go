package v1alpha1

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetCondition(t *testing.T) {
	testCases := []struct {
		name           string
		istioStatus    *IstioStatus
		conditionType  IstioConditionType
		expectedResult IstioCondition
	}{
		{
			name: "condition found",
			istioStatus: &IstioStatus{
				Conditions: []IstioCondition{
					{
						Type:   ConditionTypeReconciled,
						Status: metav1.ConditionTrue,
					},
					{
						Type:   ConditionTypeReady,
						Status: metav1.ConditionFalse,
					},
				},
			},
			conditionType: ConditionTypeReady,
			expectedResult: IstioCondition{
				Type:   ConditionTypeReady,
				Status: metav1.ConditionFalse,
			},
		},
		{
			name: "condition not found",
			istioStatus: &IstioStatus{
				Conditions: []IstioCondition{
					{
						Type:   ConditionTypeReconciled,
						Status: metav1.ConditionTrue,
					},
				},
			},
			conditionType: ConditionTypeReady,
			expectedResult: IstioCondition{
				Type:   ConditionTypeReady,
				Status: metav1.ConditionUnknown,
			},
		},
		{
			name:          "nil IstioStatus",
			istioStatus:   (*IstioStatus)(nil),
			conditionType: ConditionTypeReady,
			expectedResult: IstioCondition{
				Type:   ConditionTypeReady,
				Status: metav1.ConditionUnknown,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.istioStatus.GetCondition(tc.conditionType)
			if !reflect.DeepEqual(tc.expectedResult, result) {
				t.Errorf("Expected condition:\n    %+v,\n but got:\n    %+v", tc.expectedResult, result)
			}
		})
	}
}

func TestSetCondition(t *testing.T) {
	prevTime := time.Date(2023, 9, 26, 9, 0, 0, 0, time.UTC)
	currTime := time.Date(2023, 9, 26, 12, 0, 5, 123456, time.UTC)
	truncatedCurrTime := currTime.Truncate(time.Second)

	testCases := []struct {
		name      string
		existing  []IstioCondition
		condition IstioCondition
		expected  []IstioCondition
	}{
		{
			name: "add",
			existing: []IstioCondition{
				{
					Type:   ConditionTypeReconciled,
					Status: metav1.ConditionTrue,
				},
			},
			condition: IstioCondition{
				Type:   ConditionTypeReady,
				Status: metav1.ConditionFalse,
			},
			expected: []IstioCondition{
				{
					Type:   ConditionTypeReconciled,
					Status: metav1.ConditionTrue,
				},
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.NewTime(truncatedCurrTime),
				},
			},
		},
		{
			name: "update with status change",
			existing: []IstioCondition{
				{
					Type:               ConditionTypeReconciled,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
			},
			condition: IstioCondition{
				Type:   ConditionTypeReady,
				Status: metav1.ConditionTrue,
			},
			expected: []IstioCondition{
				{
					Type:               ConditionTypeReconciled,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(truncatedCurrTime),
				},
			},
		},
		{
			name: "update without status change",
			existing: []IstioCondition{
				{
					Type:               ConditionTypeReconciled,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					Reason:             ConditionReasonIstiodNotReady,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
			},
			condition: IstioCondition{
				Type:   ConditionTypeReady,
				Status: metav1.ConditionFalse, // same as previous status
				Reason: ConditionReasonCNINotReady,
			},
			expected: []IstioCondition{
				{
					Type:               ConditionTypeReconciled,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					Reason:             ConditionReasonCNINotReady,
					LastTransitionTime: metav1.NewTime(prevTime), // original lastTransitionTime must be preserved
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := IstioStatus{
				Conditions: tc.existing,
			}

			testTime = &currTime // force SetCondition() to use fake currTime instead of real time
			status.SetCondition(tc.condition)

			if !reflect.DeepEqual(tc.expected, status.Conditions) {
				t.Errorf("Expected condition:\n    %+v,\n but got:\n    %+v", tc.expected, status.Conditions)
			}
		})
	}
}
