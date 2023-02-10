package validation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	admissionv1 "k8s.io/api/admission/v1beta1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"
)

const maxConcurrentSARChecks = 5

var memberRegex *regexp.Regexp

type MemberRollValidator struct {
	client          client.Client
	decoder         *admission.Decoder
	namespaceFilter webhookcommon.NamespaceFilter
}

func init() {
	validNamespaceName := "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$" // matches: foo, foo-bar, foo-123, ...
	wildcardWithOptionalPrefix := "^[-a-z0-9]*\\*$"         // matches: *, foo*, foo-*, foo-123*, ...
	memberRegex = regexp.MustCompile(validNamespaceName + "|" + wildcardWithOptionalPrefix)
}

func NewMemberRollValidator(namespaceFilter webhookcommon.NamespaceFilter) *MemberRollValidator {
	return &MemberRollValidator{
		namespaceFilter: namespaceFilter,
	}
}

var (
	_ admission.Handler         = (*MemberRollValidator)(nil)
	_ inject.Client             = (*MemberRollValidator)(nil)
	_ admission.DecoderInjector = (*MemberRollValidator)(nil)
)

func (v *MemberRollValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := logf.Log.WithName("smmr-validator").
		WithValues("ServiceMeshMemberRoll", webhookcommon.ToNamespacedName(&req.AdmissionRequest))

	// use a self-imposed 3s time limit so that we can inform the user how to work
	// around the issue when the webhook takes too long to complete
	ctx, cancel := context.WithTimeout(common.NewContextWithLog(ctx, logger), 3*time.Second)
	defer cancel()

	smmr := &maistrav1.ServiceMeshMemberRoll{}
	err := v.decoder.Decode(req, smmr)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// do we care about this object?
	if !v.namespaceFilter.Watching(smmr.Namespace) {
		logger.Info(fmt.Sprintf("operator is not watching namespace '%s'", smmr.Namespace))
		return admission.Allowed("")
	} else if smmr.ObjectMeta.DeletionTimestamp != nil {
		logger.Info("skipping deleted smmr resource")
		return admission.Allowed("")
	}

	// verify name == default
	if common.MemberRollName != smmr.Name {
		return badRequest(fmt.Sprintf("ServiceMeshMemberRoll must be named '%s'", common.MemberRollName))
	}

	if smmr.Namespace == common.GetOperatorNamespace() {
		return badRequest("ServiceMeshMemberRoll may not be created in the same project/namespace as the operator")
	}

	// check if namespace names conform to DNS-1123 (we must check this in code, because +kubebuilder:validation:Pattern can't be applied to array elements yet)
	for _, member := range smmr.Spec.Members {
		if !isValidNamespaceOrWildcard(member) {
			return badRequest(fmt.Sprintf(".spec.members contains invalid value '%s'. Must be either a valid namespace name or a pattern. Valid patterns are: '*', 'my-namespace-123-*', etc.", member))
		}
	}
	for _, member := range smmr.Spec.ExcludeNamespaces {
		if !isValidNamespaceOrWildcard(member) {
			return badRequest(fmt.Sprintf(".spec.excludeNamespaces contains invalid value '%s'. Must be either a valid namespace name or a pattern. Valid patterns are: '*', 'my-namespace-123-*', etc.", member))
		}
	}

	// check for duplicate namespaces (we must check this in code, because +kubebuilder:validation:UniqueItem doesn't work)
	if len(sets.NewString(smmr.Spec.Members...)) != len(smmr.Spec.Members) {
		return badRequest("ServiceMeshMemberRoll may not contain duplicate namespaces in .spec.members")
	}

	smmrList := &maistrav1.ServiceMeshMemberRollList{}
	err = v.client.List(ctx, smmrList)
	if err != nil {
		logger.Error(err, "error listing smmr resources")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// verify no duplicate members across all smmr resources
	namespacesInOtherMemberRolls := sets.NewString()
	for _, othermr := range smmrList.Items {
		if othermr.Name == smmr.Name && othermr.Namespace == smmr.Namespace {
			continue
		}
		for _, member := range othermr.Spec.Members {
			namespacesInOtherMemberRolls.Insert(member)
		}
	}

	for _, member := range smmr.Spec.Members {
		if (member == "*" && len(namespacesInOtherMemberRolls) > 0) || namespacesInOtherMemberRolls.Has(member) || namespacesInOtherMemberRolls.Has("*") {
			return badRequest("one or more members are already defined in another ServiceMeshMemberRoll")
		} else if smmr.Namespace == member {
			return badRequest("mesh project/namespace cannot be listed as a member")
		}
	}

	allowed, err := v.isUserAllowedToUpdatePodsInAllNamespaces(ctx, req)
	if err != nil {
		logger.Error(err, "error performing cluster-scoped SAR check")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if !allowed {
		if smmr.MatchesNamespacesDynamically() {
			return forbidden(fmt.Sprintf("only users that are allowed to update pods at the cluster scope are allowed to use wildcards or member selectors; user %s does not have that permission", req.AdmissionRequest.UserInfo.Username))
		}

		// check each namespace separately, but only check newly added namespaces
		namespacesToCheck, err := v.findNewlyAddedNamespaces(smmr, req)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		allowed, rejectedNamespaces, err := v.isUserAllowedToUpdatePodsInNamespaces(ctx, req, namespacesToCheck)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return admission.Errored(http.StatusBadRequest, fmt.Errorf("too many namespaces in ServiceMeshMemberRoll; "+
					"validating webhook couldn't perform the authorization checks for all namespaces; "+
					"either try the operation again as a cluster admin, or add fewer namespaces in a single operation"))
			}
			logger.Error(err, "error performing SAR check each namespace")
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if !allowed {
			return forbidden(fmt.Sprintf("user '%s' does not have permission to access namespace(s): %s", req.AdmissionRequest.UserInfo.Username, rejectedNamespaces))
		}
	}

	return admission.Allowed("")
}

func isValidNamespaceOrWildcard(member string) bool {
	return memberRegex.MatchString(member)
}

func (v *MemberRollValidator) findNewlyAddedNamespaces(smmr *maistrav1.ServiceMeshMemberRoll, req admission.Request) (sets.String, error) {
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

func (v *MemberRollValidator) isUserAllowedToUpdatePodsInAllNamespaces(ctx context.Context, req admission.Request) (bool, error) {
	log := common.LogFromContext(ctx)
	ctx = common.NewContextWithLog(ctx, log.WithValues("namespace", "<all>"))
	return v.isUserAllowedToUpdatePodsInNamespace(ctx, req, "")
}

func (v *MemberRollValidator) isUserAllowedToUpdatePodsInNamespaces(ctx context.Context, req admission.Request,
	namespacesToCheck sets.String,
) (bool, []string, error) {
	numConcurrentSARChecks := min(len(namespacesToCheck), maxConcurrentSARChecks)

	log := common.LogFromContext(ctx)
	log.Info("Performing SAR check for each namespace", "namespaces", len(namespacesToCheck), "workers", numConcurrentSARChecks)

	t := time.Now()
	defer func() {
		log.Info("SAR check completed", "duration", time.Since(t))
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
		go func(workerNumber int) {
			workerCtx := common.NewContextWithLogValues(ctx, "worker", workerNumber)
			for ns := range in {
				allowed, err := v.isUserAllowedToUpdatePodsInNamespace(common.NewContextWithLogValues(workerCtx, "namespace", ns), req, ns)
				out <- result{namespace: ns, allowed: allowed, err: err}
			}
			wg.Done()
		}(i)
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

func (v *MemberRollValidator) isUserAllowedToUpdatePodsInNamespace(ctx context.Context, req admission.Request, ns string) (bool, error) {
	log := common.LogFromContext(ctx)
	log.Info("Performing SAR check")
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   req.AdmissionRequest.UserInfo.Username,
			UID:    req.AdmissionRequest.UserInfo.UID,
			Extra:  common.ConvertUserInfoExtra(req.AdmissionRequest.UserInfo.Extra),
			Groups: req.AdmissionRequest.UserInfo.Groups,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:      "update",
				Group:     "",
				Resource:  "pods",
				Namespace: ns,
			},
		},
	}
	err := v.client.Create(ctx, sar)
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed && !sar.Status.Denied, nil
}

// InjectClient injects the client.
func (v *MemberRollValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *MemberRollValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
