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
	// ServiceCABundleKey name of config map entry service-ca operator uses for the CA bundle
	ServiceCABundleKey = "service-ca.crt"
)

// GetCABundleFromSecret retrieves the root certificate from an Istio SA secret
func GetCABundleFromSecret(ctx context.Context, client client.Client, namespacedName types.NamespacedName, key string) ([]byte, error) {
	secret := &corev1.Secret{}
	err := client.Get(ctx, namespacedName, secret)
	if err != nil {
		return nil, err
	}
	caBundle, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf(
			"secret %s does not contain root certificate under key %s",
			namespacedName,
			key,
		)
	}
	return caBundle, nil
}

// GetCABundleFromConfigMap retrieves the CABundle from a ConfigMap injected via service.beta.openshift.io/inject-cabundle annotation
func GetCABundleFromConfigMap(ctx context.Context, client client.Client, namespacedName types.NamespacedName, key string) ([]byte, error) {
	cm := &corev1.ConfigMap{}
	err := client.Get(ctx, namespacedName, cm)
	if err != nil {
		return nil, err
	}
	caBundle, ok := cm.Data[key]
	if !ok {
		return nil, fmt.Errorf(
			"config map %s does not contain CA bundle under key %s",
			namespacedName,
			key,
		)
	}
	return []byte(caBundle), nil
}

// InjectCABundle updates the CABundle in a WebhookClientConfig. It returns true
// if the value has changed, false otherwise
func InjectCABundle(config *v1.WebhookClientConfig, caBundle []byte) bool {
	if !bytes.Equal(config.CABundle, caBundle) {
		config.CABundle = caBundle
		return true
	}
	return false
}
