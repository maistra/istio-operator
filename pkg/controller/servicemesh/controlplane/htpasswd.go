package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (r *ControlPlaneReconciler) patchHtpasswdSecret(object *unstructured.Unstructured) error {
	var rawPassword, auth string

	htSecret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: object.GetNamespace(), Name: "htpasswd"}, htSecret)
	if err == nil {
		rawPassword = string(htSecret.Data["rawPassword"])
		auth = string(htSecret.Data["auth"])
	} else {
		r.Log.Info("Creating HTPasswd entry", object.GetKind(), object.GetName())

		rawPassword, err = generatePassword(255)
		if err != nil {
			r.Log.Error(err, "failed to generate the HTPasswd password")
			return err
		}
		h := sha1.New()
		h.Write([]byte(rawPassword))
		auth = "internal:{SHA}" + base64.StdEncoding.EncodeToString(h.Sum(nil))
	}

	b64Password := base64.StdEncoding.EncodeToString([]byte(rawPassword))
	b64Auth := base64.StdEncoding.EncodeToString([]byte(auth))

	// We store the raw password in order to be able to retrieve it below, when patching Grafana ConfigMap
	err = unstructured.SetNestedField(object.UnstructuredContent(), b64Password, "data", "rawPassword")
	if err != nil {
		r.Log.Error(err, "failed to set htpasswd raw password")
		return err
	}

	err = unstructured.SetNestedField(object.UnstructuredContent(), b64Auth, "data", "auth")
	if err != nil {
		r.Log.Error(err, "failed to set htpasswd auth entry")
		return err
	}

	return nil
}

func (r *ControlPlaneReconciler) getRawHtPasswd(object *unstructured.Unstructured) (string, error) {
	htSecret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: object.GetNamespace(), Name: "htpasswd"}, htSecret)
	if err != nil {
		r.Log.Error(err, "error retrieving htpasswd Secret")
		return "", err
	}

	return string(htSecret.Data["rawPassword"]), nil
}

func (r *ControlPlaneReconciler) patchGrafanaConfig(object *unstructured.Unstructured) error {
	dsYaml, found, err := unstructured.NestedString(object.UnstructuredContent(), "data", "datasources.yaml")
	if err != nil || !found {
		r.Log.Info("skipping configuration of Grafana-Prometheus link: Could not find/retrieve datasources.yaml from Grafana ConfigMap")
		return nil
	}

	r.Log.Info("patching Grafana-Prometheus link", object.GetKind(), object.GetName())

	rawPassword, err := r.getRawHtPasswd(object)
	if err != nil {
		return err
	}

	var re = regexp.MustCompile("(?s)(basicAuthPassword:).*?\n")
	dsYaml = re.ReplaceAllString(dsYaml, fmt.Sprintf("${1} %s\n", rawPassword))
	err = unstructured.SetNestedField(object.UnstructuredContent(), dsYaml, "data", "datasources.yaml")
	if err != nil {
		r.Log.Error(err, "failed to set datasources.yaml")
		return err
	}

	return nil
}

func generatePassword(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}
