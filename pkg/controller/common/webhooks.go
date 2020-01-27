package common

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	RootCertKey = "root-cert.pem"
)

// GetRootCertFromSecret retrieves the root certificate from an Istio SA secret
func GetRootCertFromSecret(client client.Client, namespace string, name string) ([]byte, error) {
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, secret)
	if err != nil {
		return nil, err
	}
	caBundle, ok := secret.Data[RootCertKey]
	if !ok {
		return nil, fmt.Errorf(
			"secret %s/%s does not contain root certificate under key %s",
			namespace,
			name,
			RootCertKey,
		)
	}
	return caBundle, nil
}

// InjectCABundle updates the CABundle in a WebhookClientConfig. It returns true
// if the value has changed, false otherwise
func InjectCABundle(config *v1beta1.WebhookClientConfig, caBundle []byte) bool {
	if bytes.Compare(config.CABundle, caBundle) != 0 {
		config.CABundle = caBundle
		return true
	}
	return false
}
