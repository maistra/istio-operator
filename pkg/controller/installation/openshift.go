package installation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"context"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	anyuid       = "anyuid"
	clusterAdmin = "cluster-admin"
)

func (h *ReconcileInstallation) ensureProjectAndServiceAccount() error {
	project := &corev1.Namespace {
		TypeMeta: metav1.TypeMeta {
			Kind: "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: "",
		},
	}

	if err := h.client.Create(context.TODO(), project) ; err != nil && !errors.IsAlreadyExists(err) {
		logrus.Infof("Failed to create namespace %v, error is: %v", namespace, err)
		return err
	}

	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	if err := h.client.Create(context.TODO(), serviceAccount) ; err != nil && !errors.IsAlreadyExists(err) {
		logrus.Infof("Failed to create service account %v, error is: %v", serviceAccountName, err)
		return err
	}

	if err := h.addServiceAccountToSCC(namespace, serviceAccountName, anyuid) ; err != nil {
		logrus.Infof("Failed to add service account %v to scc %v, error is: %v", serviceAccountName, anyuid, err)
		return err
	}

	if err := h.addClusterRoleToServiceAccount(clusterAdmin, namespace, serviceAccountName) ; err != nil {
		logrus.Infof("Failed to add cluster role %v to service account %v, error is: %v", clusterAdmin, serviceAccountName, err)
		return err
	}
	return nil
}

func (h *ReconcileInstallation) addServiceAccountToSCC(namespace, serviceAccountName, scc string) error {
	constraint := &securityv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityContextConstraints",
			APIVersion: "security.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: scc,
		},
	}

	objectKey, err := client.ObjectKeyFromObject(constraint)
	if err != nil {
		logrus.Infof("Failed to create object key for scc: %v", err)
		return err
	}
	if err := h.client.Get(context.TODO(), objectKey, constraint); err != nil {
		logrus.Infof("Failed to retrieve scc: %v", err)
		return err
	}

	serviceAccount := "system:serviceaccount:" + namespace + ":" + serviceAccountName
	for _, user := range constraint.Users {
		if serviceAccount == user {
			return nil
		}
	}

	constraint.Users = append(constraint.Users, serviceAccount)

	if err := h.client.Update(context.TODO(), constraint); err != nil {
		logrus.Infof("Failed to update scc: %v", err)
		return err
	}

	return nil
}

func (h *ReconcileInstallation) addClusterRoleToServiceAccount(clusterRole, namespace, serviceAccountName string) error {
	binding := &v1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openshift-ansible-installer-cluster-role-binding",
			Namespace: "",
		},
		RoleRef: v1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterRole,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []v1.Subject{{
			Kind:      "ServiceAccount",
			Namespace: namespace,
			Name:      serviceAccountName,
		}},
	}

	if err := h.client.Create(context.TODO(), binding); err != nil && ! errors.IsAlreadyExists(err) {
		logrus.Infof("Failed to create cluster role binding: %v", err)
		return err
	}
	return nil
}
