package webhooks

import (
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
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

	if common.Config.OLM.WebhookManagementDisabled == true {
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

	log.Info("Adding Maistra ServiceMeshControlPlane conversion handler")
	hookServer.Register(smcpConverterServicePath, &conversion.Webhook{})

	log.Info("Adding Maistra ServiceMeshExtension conversion handler")
	hookServer.Register(SmeConverterServicePath, &conversion.Webhook{})

	log.Info("Adding Maistra ServiceMeshControlPlane validation handler")
	hookServer.Register(smcpValidatorServicePath, &webhook.Admission{
		Handler: validation.NewControlPlaneValidator(namespaceFilter),
	})

	log.Info("Adding Maistra ServiceMeshControlPlane mutation handler")
	hookServer.Register(smcpMutatorServicePath, &webhook.Admission{
		Handler: mutation.NewControlPlaneMutator(namespaceFilter),
	})

	log.Info("Adding Maistra ServiceMeshMemberRoll validation handler")
	hookServer.Register(smmrValidatorServicePath, &webhook.Admission{
		Handler: validation.NewMemberRollValidator(namespaceFilter),
	})

	log.Info("Adding Maistra ServiceMeshMemberRoll mutation handler")
	hookServer.Register(smmrMutatorServicePath, &webhook.Admission{
		Handler: mutation.NewMemberRollMutator(namespaceFilter),
	})

	log.Info("Adding Maistra ServiceMeshMember validation handler")
	hookServer.Register(smmValidatorServicePath, &webhook.Admission{
		Handler: validation.NewMemberValidator(),
	})

	return nil
}
