package controlplane

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis/external"
	kialiv1alpha1 "github.com/maistra/istio-operator/pkg/apis/external/kiali/v1alpha1"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

func (r *controlPlaneInstanceReconciler) PatchAddons(ctx context.Context) error {
	// so far, only need to patch kiali
	return r.patchKiali(ctx)
}

func (r *controlPlaneInstanceReconciler) patchKiali(ctx context.Context) error {
	if r.Instance == nil || r.Instance.Status.AppliedSpec.Addons == nil ||
		r.Instance.Status.AppliedSpec.Addons.Visualization.Kiali == nil ||
		r.Instance.Status.AppliedSpec.Addons.Visualization.Kiali.Enabled == nil ||
		!*r.Instance.Status.AppliedSpec.Addons.Visualization.Kiali.Enabled {
		return nil
	}

	log := common.LogFromContext(ctx)

	kialiConfig := r.Instance.Status.AppliedSpec.Addons.Visualization.Kiali

	// get the kiali resource
	kiali := &kialiv1alpha1.Kiali{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: kialiConfig.Name, Namespace: r.Instance.Namespace}, kiali); err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			log.Error(nil, fmt.Sprintf("could not patch kiali CR, %s/%s does not exist", r.Instance.Namespace, kialiConfig.Name))
			return nil
		}
		return err
	}
	log.Info("patching kiali CR", kiali.Kind, kiali.GetName())

	if kiali.Spec == nil {
		kiali.Spec = v1.NewHelmValues(make(map[string]interface{}))
	}

	updatedKiali := &kialiv1alpha1.Kiali{
		Base: external.Base{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kiali.Name,
				Namespace: kiali.Namespace,
			},
			Spec: v1.NewHelmValues(make(map[string]interface{})),
		},
	}

	// grafana
	grafanaURL, err := r.grafanaURL(ctx, log)
	if err != nil {
		return err
	} else if grafanaURL == "" {
		// disable grafana
		if err := updatedKiali.Spec.SetField("external_services.grafana.enabled", false); err != nil {
			return fmt.Errorf("could not set external_services.grafana.enabled in kiali CR: %s", err)
		}
	} else {
		// enable grafana and set URL
		// XXX: should we also configure the in_cluster_url
		if err := updatedKiali.Spec.SetField("external_services.grafana.url", grafanaURL); err != nil {
			return fmt.Errorf("could not set external_services.grafana.url in kiali CR: %s", err)
		}
		if err := updatedKiali.Spec.SetField("external_services.grafana.enabled", true); err != nil {
			return fmt.Errorf("could not set external_services.grafana.enabled in kiali CR: %s", err)
		}
	}

	// jaeger
	jaegerURL, err := r.jaegerURL(ctx, log)
	if err != nil {
		return nil
	} else if jaegerURL == "" {
		// disable jaeger
		if err := updatedKiali.Spec.SetField("external_services.tracing.enabled", false); err != nil {
			return fmt.Errorf("could not set external_services.tracing.enabled in kiali CR: %s", err)
		}
	} else {
		// enable jaeger and set URL
		// XXX: should we also configure the in_cluster_url
		if err := updatedKiali.Spec.SetField("external_services.tracing.url", jaegerURL); err != nil {
			return fmt.Errorf("could not set external_services.tracing.url in kiali CR: %s", err)
		}
		if err := updatedKiali.Spec.SetField("external_services.tracing.enabled", true); err != nil {
			return fmt.Errorf("could not set external_services.tracing.enabled in kiali CR: %s", err)
		}
	}

	// XXX: should we also configure prometheus?

	// credentials
	rawPassword, err := r.getRawHtPasswd(ctx)
	if err != nil {
		return fmt.Errorf("could not get htpasswd required for kiali external_serivces: %s", err)
	}
	if err := updatedKiali.Spec.SetField("external_services.grafana.auth.password", rawPassword); err != nil {
		return fmt.Errorf("could not set external_services.grafana.auth.password in kiali CR: %s", err)
	}
	if err := updatedKiali.Spec.SetField("external_services.prometheus.auth.password", rawPassword); err != nil {
		return fmt.Errorf("could not set external_services.prometheus.auth.password in kiali CR: %s", err)
	}
	if err := updatedKiali.Spec.SetField("external_services.tracing.auth.password", rawPassword); err != nil {
		return fmt.Errorf("could not set external_services.tracing.auth.password in kiali CR: %s", err)
	}

	// FUTURE: add support for synchronizing kiali version with control plane version

	if err := r.Client.Patch(ctx, updatedKiali, client.Merge); err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) || errors.IsGone(err) {
			log.Info(fmt.Sprintf("skipping kiali update, %s/%s is no longer available", kiali.GetNamespace(), kiali.GetName()))
			return nil
		}
		return err
	}

	return nil
}

func (r *controlPlaneInstanceReconciler) grafanaURL(ctx context.Context, log logr.Logger) (string, error) {
	log.Info("attempting to auto-detect Grafana for Kiali")
	grafanaRoute := &routev1.Route{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: "grafana", Namespace: r.Instance.GetNamespace()}, grafanaRoute)
	if err != nil {
		if !(errors.IsNotFound(err) || errors.IsGone(err)) {
			log.Error(err, "error retrieving Grafana route - will disable Grafana in Kiali")
			// we aren't going to return here - Grafana is optional for Kiali; Kiali can still run without it
		}
		return "", nil
	}
	return getURLForRoute(grafanaRoute), nil
}

func (r *controlPlaneInstanceReconciler) jaegerURL(ctx context.Context, log logr.Logger) (string, error) {
	log.Info("attempting to auto-detect Jaeger for Kiali")
	if r.Instance.Status.AppliedSpec.Addons == nil ||
		r.Instance.Status.AppliedSpec.Addons.Tracing.Type != v2.TracerTypeJaeger ||
		r.Instance.Status.AppliedSpec.Addons.Tracing.Jaeger == nil {
		log.Info("Jaeger is not installed, disabling tracing in Kiali")
		return "", nil
	}
	jaegerName := r.Instance.Status.AppliedSpec.Addons.Tracing.Jaeger.Name
	jaegerRoutes := &routev1.RouteList{}
	err := r.Client.List(ctx, jaegerRoutes,
		client.InNamespace(r.Instance.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance":  jaegerName,
			"app.kubernetes.io/component": "query-route",
		})
	if err != nil {
		if !(errors.IsNotFound(err) || errors.IsGone(err)) {
			log.Error(err, "error retrieving Jaeger route - will disable it in Kiali")
			// we aren't going to return here - Grafana is optional for Kiali; Kiali can still run without it
		}
		return "", nil
	} else if len(jaegerRoutes.Items) == 0 {
		// no routes
		log.Info("could not locate Jaeger query route resource, disabling tracing in Kiali")
		return "", nil
	}
	// we'll just use the first.  there should only ever be one route
	return getURLForRoute(&jaegerRoutes.Items[0]), nil
}

func getURLForRoute(route *routev1.Route) string {
	routeURL := route.Spec.Host
	if routeURL != "" {
		routeScheme := "http"
		if route.Spec.TLS != nil {
			routeScheme = "https"
		}
		routeURL = fmt.Sprintf("%s://%s", routeScheme, routeURL)
	}
	return routeURL
}
