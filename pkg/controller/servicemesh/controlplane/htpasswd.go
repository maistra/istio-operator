package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"math/big"
	"regexp"

	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func (r *controlPlaneInstanceReconciler) patchHtpasswdSecret(ctx context.Context, object *unstructured.Unstructured) error {
	var rawPassword, auth string
	log := common.LogFromContext(ctx)

	htSecret := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: object.GetNamespace(), Name: "htpasswd"}, htSecret)
	if err == nil {
		rawPassword = string(htSecret.Data["rawPassword"])
		auth = string(htSecret.Data["auth"])
	} else {
		log.Info("Creating HTPasswd entry", object.GetKind(), object.GetName())

		rawPassword, err = generatePassword(72) // 72 is the maximum length of a password for bcrypt
		if err != nil {
			log.Error(err, "failed to generate the HTPasswd password")
			return err
		}
		var version versions.Version
		version, err = versions.ParseVersion(r.Instance.Spec.Version)
		if err != nil {
			log.Error(err, "invalid version specified")
			return err
		}

		auth, err = hashPassword(version, rawPassword)
		if err != nil {
			log.Error(err, "hashing htpasswd failed")
			return err
		}
	}

	b64Password := base64.StdEncoding.EncodeToString([]byte(rawPassword))
	b64Auth := base64.StdEncoding.EncodeToString([]byte(auth))

	// We store the raw password in order to be able to retrieve it below, when patching Grafana ConfigMap
	err = unstructured.SetNestedField(object.UnstructuredContent(), b64Password, "data", "rawPassword")
	if err != nil {
		log.Error(err, "failed to set htpasswd raw password")
		return err
	}

	err = unstructured.SetNestedField(object.UnstructuredContent(), b64Auth, "data", "auth")
	if err != nil {
		log.Error(err, "failed to set htpasswd auth entry")
		return err
	}

	return nil
}

func hashPassword(version versions.Version, rawPass string) (string, error) {
	var auth, hashedPassword string
	username := "internal"

	// For SMCP versions 2.4 and above, use bcrypt hashing.
	if version.AtLeast(versions.V2_4) {
		hashedBytes, err1 := bcrypt.GenerateFromPassword([]byte(rawPass), bcrypt.DefaultCost)
		if err1 != nil {
			return "", err1
		}

		auth = fmt.Sprintf("%s:%s", username, string(hashedBytes))
	} else { // If the SMCP version is below v2.4, we use SHA-1 to keep behavior of old SMCP versions consistent,
		h := sha1.New()
		h.Write([]byte(rawPass))
		hashedPassword = base64.StdEncoding.EncodeToString(h.Sum(nil)) // hash password
		auth = fmt.Sprintf("%s:{SHA}%s", username, hashedPassword)     // store user, hash with prefix.
	}

	return auth, nil
}

func (r *controlPlaneInstanceReconciler) getRawHtPasswd(ctx context.Context) (string, error) {
	log := common.LogFromContext(ctx)
	htSecret := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: r.Instance.GetNamespace(), Name: "htpasswd"}, htSecret)
	if err != nil {
		log.Error(err, "error retrieving htpasswd Secret")
		return "", err
	}

	return string(htSecret.Data["rawPassword"]), nil
}

func (r *controlPlaneInstanceReconciler) patchGrafanaConfig(ctx context.Context, object *unstructured.Unstructured) error {
	log := common.LogFromContext(ctx)
	dsYaml, found, err := unstructured.NestedString(object.UnstructuredContent(), "data", "datasources.yaml")
	if err != nil || !found {
		log.Info("skipping configuration of Grafana-Prometheus link: Could not find/retrieve datasources.yaml from Grafana ConfigMap")
		return nil
	}

	log.Info("patching Grafana-Prometheus link", object.GetKind(), object.GetName())

	rawPassword, err := r.getRawHtPasswd(ctx)
	if err != nil {
		return err
	}

	re := regexp.MustCompile("(?s)(basicAuthPassword:).*?\n")
	dsYaml = re.ReplaceAllString(dsYaml, fmt.Sprintf("${1} %s\n", rawPassword))
	err = unstructured.SetNestedField(object.UnstructuredContent(), dsYaml, "data", "datasources.yaml")
	if err != nil {
		log.Error(err, "failed to set datasources.yaml")
		return err
	}

	return nil
}

func (r *controlPlaneInstanceReconciler) patchProxySecret(ctx context.Context, object *unstructured.Unstructured) error {
	var rawPassword string
	log := common.LogFromContext(ctx)

	proxySecret := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: object.GetNamespace(), Name: object.GetName()}, proxySecret)
	if err == nil {
		rawPassword = string(proxySecret.Data["session_secret"])
	} else {
		log.Info("Creating session_secret", object.GetKind(), object.GetName())

		rawPassword, err = generatePassword(255)
		if err != nil {
			log.Error(err, "failed to generate the session_secret password")
			return err
		}
	}

	b64Password := base64.StdEncoding.EncodeToString([]byte(rawPassword))
	err = unstructured.SetNestedField(object.UnstructuredContent(), b64Password, "data", "session_secret")
	if err != nil {
		log.Error(err, "failed to set session_secret")
		return err
	}

	return nil
}

func generatePassword(length int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	numLetters := big.NewInt(int64(len(letters)))
	ret := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, numLetters)
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}
