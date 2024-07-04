package webhooks

import (
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"

	// This is required to ensure v1.ConverterV1V2 and v1.ConverterV2V1 are properly initialized
	_ "github.com/maistra/istio-operator/pkg/apis/maistra/conversion"
	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/mutation"
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/validation"
)

const componentName = "servicemesh-webhook-server"

var log = logf.Log.WithName(componentName)

var (
	smcpValidatorServicePath = "/validate-smcp"
	smcpMutatorServicePath   = "/mutate-smcp"
	smcpConverterServicePath = "/convert-smcp"
	SmeConverterServicePath  = "/convert-sme"
	smmrValidatorServicePath = "/validate-smmr"
	smmrMutatorServicePath   = "/mutate-smmr"
	smmValidatorServicePath  = "/validate-smm"
)

// Add webhook handlers
func Add(mgr manager.Manager) error {
	ctx := common.NewContextWithLog(common.NewContext(), log)
	log.Info("Configuring Maistra webhooks")

	if !common.Config.Controller.WebhookManagementEnabled {
		log.Info("Webhook Config Management is disabled via olm configuration")
	} else {
		operatorNamespace := common.GetOperatorNamespace()
		if err := createWebhookResources(ctx, mgr, log, operatorNamespace); err != nil {
			return err
		}
	}

	watchNamespaceStr, err := k8sutil.GetWatchNamespace()
	if err != nil {
		return err
	}
	namespaceFilter := webhookcommon.NamespaceFilter(watchNamespaceStr)

	hookServer := mgr.GetWebhookServer()
	hookServer.Register(smcpConverterServicePath, &conversion.Webhook{})
	hookServer.Register(SmeConverterServicePath, &conversion.Webhook{})
	hookServer.Register(smcpValidatorServicePath, &webhook.Admission{Handler: validation.NewControlPlaneValidator(namespaceFilter)})
	hookServer.Register(smcpMutatorServicePath, &webhook.Admission{Handler: mutation.NewControlPlaneMutator(namespaceFilter)})
	hookServer.Register(smmrValidatorServicePath, &webhook.Admission{Handler: validation.NewMemberRollValidator(namespaceFilter)})
	hookServer.Register(smmrMutatorServicePath, &webhook.Admission{Handler: mutation.NewMemberRollMutator(namespaceFilter)})
	hookServer.Register(smmValidatorServicePath, &webhook.Admission{Handler: validation.NewMemberValidator()})
	return nil
}
