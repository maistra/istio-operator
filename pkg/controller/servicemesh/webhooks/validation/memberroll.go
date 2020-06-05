package validation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	admissionv1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

const maxConcurrentSARChecks = 5

type MemberRollValidator struct {
	client          client.Client
	decoder         atypes.Decoder
	namespaceFilter webhookcommon.NamespaceFilter
}

func NewMemberRollValidator(namespaceFilter webhookcommon.NamespaceFilter) *MemberRollValidator {
	return &MemberRollValidator{
		namespaceFilter: namespaceFilter,
	}
}

var _ admission.Handler = (*MemberRollValidator)(nil)
var _ inject.Client = (*MemberRollValidator)(nil)
var _ inject.Decoder = (*MemberRollValidator)(nil)

func (v *MemberRollValidator) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	logger := logf.Log.WithName("smmr-validator").
		WithValues("ServiceMeshMemberRoll", webhookcommon.ToNamespacedName(req.AdmissionRequest))

	// use a self-imposed 3s time limit so that we can inform the user how to work
	// around the issue when the webhook takes too long to complete
	ctx, cancel := context.WithTimeout(common.NewContextWithLog(ctx, logger), 3*time.Second)
	defer cancel()

	smmr := &maistrav1.ServiceMeshMemberRoll{}
	err := v.decoder.Decode(req, smmr)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	// do we care about this object?
	if !v.namespaceFilter.Watching(smmr.Namespace) {
		logger.Info(fmt.Sprintf("operator is not watching namespace '%s'", smmr.Namespace))
		return admission.ValidationResponse(true, "")
	} else if smmr.ObjectMeta.DeletionTimestamp != nil {
		logger.Info("skipping deleted smmr resource")
		return admission.ValidationResponse(true, "")
	}

	// is this mesh configured for multitenancy?
	smcpList := &maistrav1.ServiceMeshControlPlaneList{}
	err = v.client.List(ctx, client.InNamespace(smmr.Namespace), smcpList)
	if err != nil {
		logger.Error(err, "error listing smcp resources")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	if len(smcpList.Items) == 0 {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("no service mesh is configured in namespace '%s'", smmr.Namespace))
	}

	// verify name == default
	if common.MemberRollName != smmr.Name {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("ServiceMeshMemberRoll must be named '%s'", common.MemberRollName))
	}

	if smmr.Namespace == common.GetOperatorNamespace() {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("ServiceMeshMemberRoll may not be created in the same project/namespace as the operator"))
	}

	smmrList := &maistrav1.ServiceMeshMemberRollList{}
	err = v.client.List(ctx, nil, smmrList)
	if err != nil {
		logger.Error(err, "error listing smmr resources")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}

	// verify no duplicate members across all smmr resources
	namespacesAlreadyConfigured := sets.NewString()
	for _, othermr := range smmrList.Items {
		if othermr.Name == smmr.Name && othermr.Namespace == smmr.Namespace {
			continue
		}
		for _, member := range othermr.Spec.Members {
			namespacesAlreadyConfigured.Insert(member)
		}
	}

	for _, member := range smmr.Spec.Members {
		if namespacesAlreadyConfigured.Has(member) {
			return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "one or more members are already defined in another ServiceMeshMemberRoll")
		} else if smmr.Namespace == member {
			return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "mesh project/namespace cannot be listed as a member")
		}
	}

	allowed, err := v.isUserAllowedToUpdatePods(common.NewContextWithLog(ctx, logger.WithValues("namespace", "<all>")), req, "")
	if err != nil {
		logger.Error(err, fmt.Sprintf("error performing cluster-scoped SAR check"))
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	if !allowed {
		// check each namespace separately, but only check newly added namespaces
		namespacesToCheck, err := v.findNewlyAddedNamespaces(smmr, req)
		if err != nil {
			return admission.ErrorResponse(http.StatusBadRequest, err)
		}

		allowed, rejectedNamespaces, err := v.isUserAllowedToUpdatePodsInAllNamespaces(ctx, req, namespacesToCheck)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return admission.ErrorResponse(http.StatusBadRequest, fmt.Errorf("too many namespaces in ServiceMeshMemberRoll; validating webhook couldn't perform the authorization checks for all namespaces; either try the operation again as a cluster admin, or add fewer namespaces in a single operation"))
			}
			logger.Error(err, fmt.Sprintf("error performing SAR check each namespace"))
			return admission.ErrorResponse(http.StatusInternalServerError, err)
		}
		if !allowed {
			return validationFailedResponse(http.StatusForbidden, metav1.StatusReasonBadRequest, fmt.Sprintf("user '%s' does not have permission to access namespace(s): %s", req.AdmissionRequest.UserInfo.Username, rejectedNamespaces))
		}
	}

	return admission.ValidationResponse(true, "")
}

func (v *MemberRollValidator) findNewlyAddedNamespaces(smmr *maistrav1.ServiceMeshMemberRoll, req atypes.Request) (sets.String, error) {
	namespacesToCheck := sets.NewString(smmr.Spec.Members...)

	if req.AdmissionRequest.Operation == admissionv1.Update {
		oldSmmr := &maistrav1.ServiceMeshMemberRoll{}
		err := v.decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldSmmr)
		if err != nil {
			return nil, err
		}
		namespacesToCheck.Delete(oldSmmr.Spec.Members...)
	}
	return namespacesToCheck, nil
}

func (v *MemberRollValidator) isUserAllowedToUpdatePodsInAllNamespaces(ctx context.Context, req atypes.Request, namespacesToCheck sets.String) (bool, []string, error) {
	numConcurrentSARChecks := min(len(namespacesToCheck), maxConcurrentSARChecks)

	log := common.LogFromContext(ctx)
	log.Info("Performing SAR check for each namespace", "namespaces", len(namespacesToCheck), "workers", numConcurrentSARChecks)

	t := time.Now()
	defer func() {
		log.Info("SAR check completed", "duration", time.Now().Sub(t))
	}()

	in := make(chan string)
	go func() {
		defer close(in)
		for _, ns := range namespacesToCheck.List() {
			in <- ns
		}
	}()

	out := make(chan result)
	var wg sync.WaitGroup
	wg.Add(numConcurrentSARChecks)
	for i := 0; i < numConcurrentSARChecks; i++ {
		go func() {
			workerCtx := common.NewContextWithLogValues(ctx, "worker", i)
			for ns := range in {
				allowed, err := v.isUserAllowedToUpdatePods(common.NewContextWithLogValues(workerCtx, "namespace", ns), req, ns)
				out <- result{namespace: ns, allowed: allowed, err: err}
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()

	rejectedNamespaces := []string{}
	for res := range out {
		if res.err != nil {
			return false, nil, res.err
		}
		if !res.allowed {
			rejectedNamespaces = append(rejectedNamespaces, res.namespace)
		}
	}
	return len(rejectedNamespaces) == 0, rejectedNamespaces, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type result struct {
	namespace string
	allowed   bool
	err       error
}

func (v *MemberRollValidator) isUserAllowedToUpdatePods(ctx context.Context, req atypes.Request, member string) (bool, error) {
	log := common.LogFromContext(ctx)
	log.Info("Performing SAR check")
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   req.AdmissionRequest.UserInfo.Username,
			UID:    req.AdmissionRequest.UserInfo.UID,
			Extra:  convertUserInfoExtra(req.AdmissionRequest.UserInfo.Extra),
			Groups: req.AdmissionRequest.UserInfo.Groups,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:      "update",
				Group:     "",
				Resource:  "pods",
				Namespace: member,
			},
		},
	}
	err := v.client.Create(ctx, sar)
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed && !sar.Status.Denied, nil
}

func convertUserInfoExtra(extra map[string]authenticationv1.ExtraValue) map[string]authorizationv1.ExtraValue {
	converted := map[string]authorizationv1.ExtraValue{}
	for key, value := range extra {
		converted[key] = authorizationv1.ExtraValue(value)
	}
	return converted
}

// InjectClient injects the client.
func (v *MemberRollValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *MemberRollValidator) InjectDecoder(d atypes.Decoder) error {
	v.decoder = d
	return nil
}
