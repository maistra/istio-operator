package common

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

    "sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ResourceManager) AddUsersToSCC(sccName string, users ...string) ([]string, error) {
    added := make([]string, 0, len(users))
	scc := &unstructured.Unstructured{}
	scc.SetAPIVersion("security.openshift.io/v1")
	scc.SetKind("SecurityContextConstraints")
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: sccName}, scc)

	if err == nil {
		existing, exists, _ := unstructured.NestedStringSlice(scc.UnstructuredContent(), "users")
		if !exists {
			existing = []string{}
        }
        modified := false
        for _, user := range users {
            if IndexOf(existing, user) < 0 {
                r.Log.Info("Adding ServiceAccount to SecurityContextConstraints", "ServiceAccount", user, "SecurityContextConstraints", sccName)
                existing = append(existing, user)
                added = append(added, user)
                modified = true
            }    
        }
        if modified {
			unstructured.SetNestedStringSlice(scc.UnstructuredContent(), existing, "users")
			err = r.Client.Update(context.TODO(), scc)
		}
	}
	return added, err
}

func (r ResourceManager) RemoveUsersFromSCC(sccName string, users ...string) error {
	scc := &unstructured.Unstructured{}
	scc.SetAPIVersion("security.openshift.io/v1")
	scc.SetKind("SecurityContextConstraints")
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: sccName}, scc)

	if err == nil {
		existing, exists, _ := unstructured.NestedStringSlice(scc.UnstructuredContent(), "users")
		if !exists {
			return nil
        }
        modified := false
        for _, user := range users {
            if index := IndexOf(existing, user); index >= 0 {
                r.Log.Info("Removing ServiceAccount from SecurityContextConstraints", "ServiceAccount", user, "SecurityContextConstraints", sccName)
                existing = append(existing[:index], existing[index+1:]...)
                modified = true
            }
        }
        if modified {
			unstructured.SetNestedStringSlice(scc.UnstructuredContent(), existing, "users")
			err = r.Client.Update(context.TODO(), scc)
		}
	}
	return err
}
