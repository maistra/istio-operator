package validation

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/maistra/istio-operator/pkg/controller/common"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	arbeta1 "k8s.io/api/admissionregistration/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	admissiontypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
	"sigs.k8s.io/controller-runtime/pkg/webhook/types"
)

var log = logf.Log.WithName("controller_servicemeshvalidation")

type namespaceFilter string

var watchNamespace namespaceFilter

func init() {
	watchNamespaceStr, _ := k8sutil.GetWatchNamespace()
	watchNamespace = namespaceFilter(watchNamespaceStr)
}

// Add webhooks
func Add(mgr manager.Manager) error {
	log.Info("setting up webhook server")
	operatorNamespace := common.GetOperatorNamespace()
	hookServer, err := webhook.NewServer(
		"servicemesh-webhook-server",
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

	log.Info("registering webhooks to the webhook server")
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
				&controlPlaneValidator{},
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
				&memberRollValidator{},
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
				&memberValidator{},
			},
		},
	)
}

func (f namespaceFilter) watching(namespace string) bool {
	return len(f) == 0 || namespace == string(f)
}

func validationFailedResponse(httpStatusCode int32, reason metav1.StatusReason, message string) admissiontypes.Response {
	response := admission.ValidationResponse(false, string(reason))
	if len(reason) == 0 {
		response.Response.Result = &metav1.Status{}
	}
	response.Response.Result.Code = httpStatusCode
	response.Response.Result.Message = message
	return response
}
