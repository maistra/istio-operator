package external

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

type testType struct {
	Base `json:",inline"`
}

func (in *testType) DeepCopyInto(out *testType) {
	*out = *in
	in.Base.DeepCopyInto(&out.Base)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Apikey.
func (in *testType) DeepCopy() *testType {
	if in == nil {
		return nil
	}
	out := new(testType)
	in.DeepCopyInto(out)
	return out
}

func TestDeepCopy(t *testing.T) {
	source := &testType{
		Base: Base{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "simple/v1",
				Kind:       "TestType",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:              "dummy-name",
				Namespace:         "dummy-namespace",
				CreationTimestamp: metav1.Now(),
				Labels: map[string]string{
					"label": "value",
				},
				Annotations: map[string]string{
					"annotation": "value",
				},
			},
			Spec: v1.NewHelmValues(map[string]interface{}{
				"string": "some-value",
				"int":    int64(42),
				"float":  float64(4.2),
				"bool":   true,
				"nestedType": map[string]interface{}{
					"nestedString": "nested-value",
					"nestedInt":    int64(25),
					"nestedFloat":  float64(2.5),
					"nestedBool":   true,
				},
			}),
		},
	}
	sourceCopy := source.DeepCopy()
	if !reflect.DeepEqual(source, sourceCopy) {
		t.Errorf("DeepCopy() failed:\n\texpected:\n\t\t%#v\n\tactual:\n\t\t%#v", source, sourceCopy)
	}
}