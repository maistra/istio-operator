package controlplane

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/controllers/common"
)

func addNamespaceLabels(ctx context.Context, cl client.Client, namespace string) error {
	return setNamespaceLabels(ctx, cl, namespace, map[string]string{
		common.IgnoreNamespaceKey: "ignore",  // ensures injection is disabled for the control plane
		common.MemberOfKey:        namespace, // ensures networking works correctly
	})
}

func removeNamespaceLabels(ctx context.Context, cl client.Client, namespace string) error {
	return setNamespaceLabels(ctx, cl, namespace, map[string]string{
		common.IgnoreNamespaceKey: "",
		common.MemberOfKey:        "",
	})
}

func setNamespaceLabels(ctx context.Context, cl client.Client, namespace string, labels map[string]string) error {
	log := common.LogFromContext(ctx)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	err := cl.Get(ctx, client.ObjectKey{Name: namespace}, ns)
	if err != nil {
		return err
	}
	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	updateRequired := false
	for key, newValue := range labels {
		currValue, currValueExists := ns.Labels[key]
		if newValue == "" {
			if currValueExists {
				log.Info("Removing label from namespace", "label", fmt.Sprintf("%s=%s", key, currValue))
				delete(ns.Labels, key)
				updateRequired = true
			}
		} else if currValue != newValue {
			log.Info("Adding label to namespace", "label", fmt.Sprintf("%s=%s", key, newValue))
			ns.Labels[key] = newValue
			updateRequired = true
		}
	}
	if updateRequired {
		err = cl.Update(ctx, ns)
	}
	return err
}
