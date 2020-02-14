package webhooks

import (
	"fmt"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/validation"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	arbeta1 "k8s.io/api/admissionregistration/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/types"
)

const componentName = "servicemesh-webhook-server"

var log = logf.Log.WithName(componentName)

// Add webhooks
func Add(mgr manager.Manager) error {
	log.Info("Setting up webhook server")
	operatorNamespace := common.GetOperatorNamespace()
	hookServer, err := webhook.NewServer(
		componentName,
		mgr,
		webhook.ServerOptions{
			Port:    11999,
			CertDir: "/tmp/cert",
			BootstrapOptions: &webhook.BootstrapOptions{
				ValidatingWebhookConfigName: fmt.Sprintf("%s.servicemesh-resources.maistra.io", operatorNamespace),
				Service: &webhook.Service{
					Name:      "admission-controller",
					Namespace: operatorNamespace,
					Selectors: map[string]string{
						"name": "istio-operator",
					},
				},
			},
		})
	if err != nil {
		return err
	}

	log.Info("Registering webhooks to the webhook server")
	failurePolicy := arbeta1.Fail
	return hookServer.Register(
		&admission.Webhook{
			Name: "smcp.validation.maistra.io",
			Path: "/validate-smcp",
			Rules: []arbeta1.RuleWithOperations{
				{
					Rule: arbeta1.Rule{
						APIGroups:   []string{maistrav1.SchemeGroupVersion.Group},
						APIVersions: []string{maistrav1.SchemeGroupVersion.Version},
						Resources:   []string{"servicemeshcontrolplanes"},
					},
					Operations: []arbeta1.OperationType{arbeta1.Create, arbeta1.Update},
				},
			},
			FailurePolicy: &failurePolicy,
			Type:          types.WebhookTypeValidating,
			Handlers: []admission.Handler{
				&validation.ControlPlaneValidator{},
			},
		},
		&admission.Webhook{
			Name: "smmr.validation.maistra.io",
			Path: "/validate-smmr",
			Rules: []arbeta1.RuleWithOperations{
				{
					Rule: arbeta1.Rule{
						APIGroups:   []string{maistrav1.SchemeGroupVersion.Group},
						APIVersions: []string{maistrav1.SchemeGroupVersion.Version},
						Resources:   []string{"servicemeshmemberrolls"},
					},
					Operations: []arbeta1.OperationType{arbeta1.Create, arbeta1.Update},
				},
			},
			FailurePolicy: &failurePolicy,
			Type:          types.WebhookTypeValidating,
			Handlers: []admission.Handler{
				&validation.MemberRollValidator{},
			},
		},
		&admission.Webhook{
			Name: "smm.validation.maistra.io",
			Path: "/validate-smm",
			Rules: []arbeta1.RuleWithOperations{
				{
					Rule: arbeta1.Rule{
						APIGroups:   []string{maistrav1.SchemeGroupVersion.Group},
						APIVersions: []string{maistrav1.SchemeGroupVersion.Version},
						Resources:   []string{"servicemeshmembers"},
					},
					Operations: []arbeta1.OperationType{arbeta1.Create, arbeta1.Update},
				},
			},
			FailurePolicy: &failurePolicy,
			Type:          types.WebhookTypeValidating,
			Handlers: []admission.Handler{
				&validation.MemberValidator{},
			},
		},
	)
}
