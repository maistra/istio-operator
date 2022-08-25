package webhookca

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectRef struct {
	Kind      string
	Namespace string
	Name      string
}

type CABundleSource interface {
	GetCABundle(ctx context.Context, client client.Client) ([]byte, error)
	GetNamespace() string
	SetNamespace(string)
	MatchedObjects() []ObjectRef
	Copy() CABundleSource
}

type SecretCABundleSource struct {
	SecretNames []string
	Namespace   string
	Key         string
}

var _ CABundleSource = (*SecretCABundleSource)(nil)

func (s *SecretCABundleSource) GetCABundle(ctx context.Context, client client.Client) ([]byte, error) {
	var caBundle []byte
	errList := []error{}
	for _, secretName := range s.SecretNames {
		namespacedName := types.NamespacedName{
			Namespace: s.Namespace,
			Name:      secretName,
		}
		secret := &corev1.Secret{}
		err := client.Get(ctx, namespacedName, secret)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		var ok bool
		caBundle, ok = secret.Data[s.Key]
		if !ok {
			errList = append(errList, fmt.Errorf(
				"secret %s does not contain root certificate under key %s",
				namespacedName,
				s.Key,
			))
			continue
		}
		break
	}
	if caBundle == nil {
		return nil, errors.NewAggregate(errList)
	}
	return caBundle, nil
}

func (s *SecretCABundleSource) GetNamespace() string {
	return s.Namespace
}

func (s *SecretCABundleSource) SetNamespace(namespace string) {
	s.Namespace = namespace
}

func (s *SecretCABundleSource) MatchedObjects() []ObjectRef {
	refs := []ObjectRef{}
	for _, secretName := range s.SecretNames {
		refs = append(refs, ObjectRef{
			Kind:      "Secret",
			Namespace: s.Namespace,
			Name:      secretName,
		})
	}
	return refs
}

func (s *SecretCABundleSource) Copy() CABundleSource {
	s2 := *s
	return &s2
}

type ConfigMapCABundleSource struct {
	ConfigMapName string
	Key           string
	Namespace     string
}

var _ CABundleSource = (*ConfigMapCABundleSource)(nil)

func (s *ConfigMapCABundleSource) GetCABundle(ctx context.Context, client client.Client) ([]byte, error) {
	namespacedName := types.NamespacedName{Namespace: s.Namespace, Name: s.ConfigMapName}
	cm := &corev1.ConfigMap{}
	err := client.Get(ctx, namespacedName, cm)
	if err != nil {
		return nil, err
	}
	caBundle, ok := cm.Data[s.Key]
	if !ok {
		return nil, fmt.Errorf(
			"config map %s does not contain CA bundle under key %s",
			namespacedName,
			s.Key,
		)
	}
	return []byte(caBundle), nil
}

func (s *ConfigMapCABundleSource) GetNamespace() string {
	return s.Namespace
}

func (s *ConfigMapCABundleSource) SetNamespace(namespace string) {
	s.Namespace = namespace
}

func (s *ConfigMapCABundleSource) MatchedObjects() []ObjectRef {
	return []ObjectRef{
		{
			Kind:      "ConfigMap",
			Namespace: s.Namespace,
			Name:      s.ConfigMapName,
		},
	}
}

func (s *ConfigMapCABundleSource) Copy() CABundleSource {
	s2 := *s
	return &s2
}
