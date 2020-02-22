package webhooks

import (
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/mutation"
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/validation"
)

const componentName = "servicemesh-webhook-server"

var log = logf.Log.WithName(componentName)

var (
	smcpValidatorServicePath = "/validate-smcp"
	smcpMutatorServicePath = "/mutate-smcp"
	smmrValidatorServicePath = "/validate-smmr"
	smmrMutatorServicePath = "/mutate-smmr"
	smmValidatorServicePath = "/validate-smm"
)

// Add webhook handlers
func Add(mgr manager.Manager) error {
	log.Info("Configuring Maistra webhooks")

	operatorNamespace := common.GetOperatorNamespace()
	if err := createWebhookResources(mgr, log, operatorNamespace); err != nil {
		return err
	}

	watchNamespaceStr, err := k8sutil.GetWatchNamespace()
	if err != nil {
		return err
	}
	namespaceFilter := webhookcommon.NamespaceFilter(watchNamespaceStr)

	hookServer := mgr.GetWebhookServer()

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

