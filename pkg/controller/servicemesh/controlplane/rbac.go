package controlplane

import (
	"context"
	"reflect"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

const (
	roleMeshUser         = "mesh-user"
	roleBindingMeshUsers = "mesh-users"
)

func (r *controlPlaneInstanceReconciler) reconcileRBAC(ctx context.Context) error {
	err := r.reconcileMeshUserRole(ctx)
	if err != nil {
		return err
	}

	err = r.reconcileMeshUsersRoleBinding(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *controlPlaneInstanceReconciler) reconcileMeshUserRole(ctx context.Context) error {
	meshNamespace := r.Instance.Namespace
	key := types.NamespacedName{Namespace: meshNamespace, Name: roleMeshUser}
	rule := rbacv1.PolicyRule{
		APIGroups: []string{maistrav1.APIGroup},
		Resources: []string{"servicemeshcontrolplanes"},
		Verbs:     []string{"use"},
	}
	log := common.LogFromContext(ctx).WithValues("Role", key)

	role := rbacv1.Role{}
	err := r.Client.Get(ctx, key, &role)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Creating Role")
			role = rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleMeshUser,
					Namespace: meshNamespace,
				},
				Rules: []rbacv1.PolicyRule{rule},
			}
			r.setOwnerReferenceOn(&role)
			return r.Client.Create(ctx, &role)
		}
		return err
	}

	if !metav1.IsControlledBy(&role, r.Instance) {
		// the role wasn't created by this controller, se we shouldn't touch it
		log.Info("Ignoring Role, because it wasn't created by this controller.")
		return nil
	}

	if !containsRule(role, rule) {
		log.Info("Adding rule to Role.")
		role.Rules = append(role.Rules, rule)
		err := r.Client.Update(ctx, &role)
		return err
	}
	return nil
}

func containsRule(role rbacv1.Role, rule rbacv1.PolicyRule) bool {
	for _, r := range role.Rules {
		if reflect.DeepEqual(r, rule) {
			return true
		}
	}
	return false
}

func (r *controlPlaneInstanceReconciler) reconcileMeshUsersRoleBinding(ctx context.Context) error {
	meshNamespace := r.Instance.Namespace
	key := types.NamespacedName{Namespace: meshNamespace, Name: roleBindingMeshUsers}
	expectedBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingMeshUsers,
			Namespace: meshNamespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleMeshUser,
		},
	}
	r.setOwnerReferenceOn(&expectedBinding)

	log := common.LogFromContext(ctx).WithValues("RoleBinding", key)

	binding := rbacv1.RoleBinding{}
	err := r.Client.Get(ctx, key, &binding)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Creating RoleBinding")
			return r.Client.Create(ctx, &expectedBinding)
		}
		return err
	}

	if !metav1.IsControlledBy(&binding, r.Instance) {
		// the binding wasn't created by this controller, se we shouldn't touch it
		log.Info("Ignoring RoleBinding, because it wasn't created by this controller.")
		return nil
	}

	if !reflect.DeepEqual(binding.RoleRef, expectedBinding.RoleRef) {
		// roleRef can't be changed, so we need to recreate the object
		log.Info("Recreating RoleBinding due to invalid roleRef.")
		err := r.Client.Delete(ctx, &binding)
		if err != nil {
			return err
		}
		return r.Client.Update(ctx, &expectedBinding)
	}
	return nil
}

func (r *controlPlaneInstanceReconciler) setOwnerReferenceOn(binding metav1.Object) {
	owner := metav1.NewControllerRef(r.Instance, maistrav1.SchemeGroupVersion.WithKind("ServiceMeshControlPlane"))
	binding.SetOwnerReferences([]metav1.OwnerReference{*owner})
}
