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
		istioStatus    *IstioRevisionStatus
		conditionType  IstioRevisionConditionType
		expectedResult IstioRevisionCondition
	}{
		{
			name: "condition found",
			istioStatus: &IstioRevisionStatus{
				Conditions: []IstioRevisionCondition{
					{
						Type:   IstioRevisionConditionTypeReconciled,
						Status: metav1.ConditionTrue,
					},
					{
						Type:   IstioRevisionConditionTypeReady,
						Status: metav1.ConditionFalse,
					},
				},
			},
			conditionType: IstioRevisionConditionTypeReady,
			expectedResult: IstioRevisionCondition{
				Type:   IstioRevisionConditionTypeReady,
				Status: metav1.ConditionFalse,
			},
		},
		{
			name: "condition not found",
			istioStatus: &IstioRevisionStatus{
				Conditions: []IstioRevisionCondition{
					{
						Type:   IstioRevisionConditionTypeReconciled,
						Status: metav1.ConditionTrue,
					},
				},
			},
			conditionType: IstioRevisionConditionTypeReady,
			expectedResult: IstioRevisionCondition{
				Type:   IstioRevisionConditionTypeReady,
				Status: metav1.ConditionUnknown,
			},
		},
		{
			name:          "nil IstioRevisionStatus",
			istioStatus:   (*IstioRevisionStatus)(nil),
			conditionType: IstioRevisionConditionTypeReady,
			expectedResult: IstioRevisionCondition{
				Type:   IstioRevisionConditionTypeReady,
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
		existing  []IstioRevisionCondition
		condition IstioRevisionCondition
		expected  []IstioRevisionCondition
	}{
		{
			name: "add",
			existing: []IstioRevisionCondition{
				{
					Type:   IstioRevisionConditionTypeReconciled,
					Status: metav1.ConditionTrue,
				},
			},
			condition: IstioRevisionCondition{
				Type:   IstioRevisionConditionTypeReady,
				Status: metav1.ConditionFalse,
			},
			expected: []IstioRevisionCondition{
				{
					Type:   IstioRevisionConditionTypeReconciled,
					Status: metav1.ConditionTrue,
				},
				{
					Type:               IstioRevisionConditionTypeReady,
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.NewTime(truncatedCurrTime),
				},
			},
		},
		{
			name: "update with status change",
			existing: []IstioRevisionCondition{
				{
					Type:               IstioRevisionConditionTypeReconciled,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               IstioRevisionConditionTypeReady,
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
			},
			condition: IstioRevisionCondition{
				Type:   IstioRevisionConditionTypeReady,
				Status: metav1.ConditionTrue,
			},
			expected: []IstioRevisionCondition{
				{
					Type:               IstioRevisionConditionTypeReconciled,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               IstioRevisionConditionTypeReady,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(truncatedCurrTime),
				},
			},
		},
		{
			name: "update without status change",
			existing: []IstioRevisionCondition{
				{
					Type:               IstioRevisionConditionTypeReconciled,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               IstioRevisionConditionTypeReady,
					Status:             metav1.ConditionFalse,
					Reason:             IstioRevisionConditionReasonIstiodNotReady,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
			},
			condition: IstioRevisionCondition{
				Type:   IstioRevisionConditionTypeReady,
				Status: metav1.ConditionFalse, // same as previous status
				Reason: IstioRevisionConditionReasonCNINotReady,
			},
			expected: []IstioRevisionCondition{
				{
					Type:               IstioRevisionConditionTypeReconciled,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               IstioRevisionConditionTypeReady,
					Status:             metav1.ConditionFalse,
					Reason:             IstioRevisionConditionReasonCNINotReady,
					LastTransitionTime: metav1.NewTime(prevTime), // original lastTransitionTime must be preserved
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := IstioRevisionStatus{
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
