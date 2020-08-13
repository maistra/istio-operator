package common

import (
	"bytes"
	"context"
	"fmt"

	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// IstioRootCertKey name of Secret entry Istio uses for the cert
	IstioRootCertKey = "root-cert.pem"
	// IstiodCertKey name of Secret entry Istiod uses for the cert
	IstiodCertKey = "ca-cert.pem"
	// ServiceCARootCertKey name of Secret entry service-ca operator uses for the cert
	ServiceCARootCertKey = "tls.crt"
)

// GetRootCertFromSecret retrieves the root certificate from an Istio SA secret
func GetRootCertFromSecret(ctx context.Context, client client.Client, namespace string, name string, key string) ([]byte, error) {
	secret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, secret)
	if err != nil {
		return nil, err
	}
	caBundle, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf(
			"secret %s/%s does not contain root certificate under key %s",
			namespace,
			name,
			key,
		)
	}
	return caBundle, nil
}

// InjectCABundle updates the CABundle in a WebhookClientConfig. It returns true
// if the value has changed, false otherwise
func InjectCABundle(config *v1.WebhookClientConfig, caBundle []byte) bool {
	if bytes.Compare(config.CABundle, caBundle) != 0 {
		config.CABundle = caBundle
		return true
	}
	return false
}
