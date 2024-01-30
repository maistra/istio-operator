package versions

import (
	"context"
	"fmt"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"testing"
)

type validationTestCase struct {
	name         string
	smcp         *maistrav2.ControlPlaneSpec
	existingObjs []*maistrav2.ServiceMeshControlPlane
	expectedErr  error
}

func NewV2SMCPResource(name, namespace string, spec *maistrav2.ControlPlaneSpec) *maistrav2.ServiceMeshControlPlane {
	smcp := &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	spec.DeepCopyInto(&smcp.Spec)
	smcp.Spec.Profiles = []string{"maistra"}
	smcp.Spec.Version = "v2.5"
	smcp.UID = uuids[spec]
	return smcp
}

var (
	simpleMultiTenant  = newSmcpSpec("mode: MultiTenant")
	simpleMultiTenant2 = simpleMultiTenant.DeepCopy()
	simpleClusterWide  = newSmcpSpec("mode: ClusterWide")
	simpleClusterWide2 = simpleClusterWide.DeepCopy()

	multiTenantGatewayController = newSmcpSpec(`
techPreview:
  gatewayAPI:
    controllerMode: true`)

	clusterWideGatewayController = newSmcpSpec(`
mode: ClusterWide
techPreview:
  gatewayAPI:
    controllerMode: true
  global:
    caCertConfigMapName: ossm-ca-root-cert
`)

	clusterWideGatewayController2 = clusterWideGatewayController.DeepCopy()

	clusterWideGatewayControllerDefaultCA = newSmcpSpec(`
mode: ClusterWide
techPreview:
  gatewayAPI:
    controllerMode: true
`)

	uuids = map[*maistrav2.ControlPlaneSpec]types.UID{
		simpleMultiTenant:                     uuid.NewUUID(),
		simpleMultiTenant2:                    uuid.NewUUID(),
		simpleClusterWide:                     uuid.NewUUID(),
		simpleClusterWide2:                    uuid.NewUUID(),
		multiTenantGatewayController:          uuid.NewUUID(),
		clusterWideGatewayController:          uuid.NewUUID(),
		clusterWideGatewayController2:         uuid.NewUUID(),
		clusterWideGatewayControllerDefaultCA: uuid.NewUUID(),
	}
)

var testCases = []validationTestCase{
	// TODO: add test cases where 2 multi-tenant control planes already exist
	{
		name: "creating multi-tenant SMCP when no other SMCPs exists - no errors",
		smcp: newSmcpSpec(`mode: ClusterWide`),
	},
	{
		name: "creating multi-tenant SMCP when another multi-tenant already exists - no errors",
		smcp: simpleMultiTenant,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant2),
		},
	},
	{
		name: "creating cluster-wide gateway controller when multi-tenant SMCP exists - no errors",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant),
		},
	},
	{
		name: "creating cluster-wide gateway controller when multi-tenant SMCP exists - no errors (2nd execution)",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant),
			NewV2SMCPResource("basic", "istio-system-2", clusterWideGatewayController),
		},
	},
	{
		name: "creating cluster-wide gateway controller when multi-tenant gateway controller exists - no errors",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", multiTenantGatewayController),
		},
	},
	{
		name: "creating cluster-wide gateway controller when multi-tenant gateway controller exists - no errors (2nd execution)",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", multiTenantGatewayController),
			NewV2SMCPResource("basic", "istio-system-2", clusterWideGatewayController),
		},
	},
	{
		name: "creating cluster-wide gateway-controller when multi-tenant SMCP exists - no errors",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant),
		},
	},
	{
		name: "creating cluster-wide gateway-controller when multi-tenant SMCP exists - no errors (2nd execution)",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant),
			NewV2SMCPResource("basic", "istio-system-2", clusterWideGatewayController),
		},
	},
	{
		name: "creating cluster-wide gateway controller when simple cluster-wide SMCP exists - no errors",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-2", simpleClusterWide),
		},
	},
	{
		name: "creating cluster-wide gateway controller when simple cluster-wide SMCP exists - no errors (2nd execution)",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayController),
			NewV2SMCPResource("basic", "istio-system-2", simpleClusterWide),
		},
	},
	{
		name: "creating simple cluster-wide SMCP when cluster-wide gateway controller exists - no errors",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayController),
		},
	},
	{
		name: "creating simple cluster-wide SMCP when cluster-wide gateway controller exists - no errors (2nd execution)",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayController),
			NewV2SMCPResource("basic", "istio-system-2", simpleClusterWide),
		},
	},
	{
		name: "creating multi-tenant SMCP when cluster-wide gateway controller exists - no errors",
		smcp: simpleMultiTenant,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayController),
		},
	},
	{
		name: "creating multi-tenant SMCP when cluster-wide gateway controller exists - no errors (2nd execution)",
		smcp: simpleMultiTenant,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayController),
			NewV2SMCPResource("basic", "istio-system-2", simpleMultiTenant),
		},
	},
	{
		name: "creating cluster-wide SMCP when multi-tenant mesh already exists - error expected",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant),
		},
		expectedErr: fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist"),
	},
	{
		name: "creating cluster-wide SMCP when multi-tenant mesh already exists - error expected (2nd execution)",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant),
			NewV2SMCPResource("basic", "istio-system-2", simpleClusterWide),
		},
		expectedErr: fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist"),
	},
	{
		name: "creating cluster-wide SMCP when multi-tenant gateway controller exists - error expected",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", multiTenantGatewayController),
		},
		expectedErr: fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist"),
	},
	{
		name: "creating cluster-wide SMCP when multi-tenant gateway controller exists - error expected (2nd execution)",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", multiTenantGatewayController),
			NewV2SMCPResource("basic", "istio-system-2", simpleClusterWide),
		},
		expectedErr: fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist"),
	},
	{
		name: "creating cluster-wide SMCP when cluster-wide SMCP exists - expected error",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleClusterWide2),
		},
		expectedErr: fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist"),
	},
	{
		name: "creating cluster-wide SMCP when cluster-wide SMCP exists - expected error (2nd execution)",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleClusterWide),
			NewV2SMCPResource("basic", "istio-system-2", simpleClusterWide2),
		},
		expectedErr: fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist"),
	},
	{
		name: "creating cluster-wide gateway controller SMCP when another already exists - expected error",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayController2),
		},
		expectedErr: fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist"),
	},
	{
		name: "creating cluster-wide gateway controller SMCP when another already exists - expected error (2nd execution)",
		smcp: clusterWideGatewayController,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayController),
			NewV2SMCPResource("basic", "istio-system-2", clusterWideGatewayController2),
		},
		expectedErr: fmt.Errorf("a cluster-scoped SMCP may only be created when no other SMCPs exist"),
	},
	{
		name: "creating multi-tenant SMCP when cluster-wide SMCP exists - expected error",
		smcp: simpleMultiTenant,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleClusterWide),
		},
		expectedErr: fmt.Errorf("no other SMCPs may be created when a cluster-scoped SMCP exists"),
	},
	{
		name: "creating multi-tenant SMCP when cluster-wide SMCP exists - expected error (2nd execution)",
		smcp: simpleMultiTenant,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleClusterWide),
			NewV2SMCPResource("basic", "istio-system-2", simpleMultiTenant),
		},
		expectedErr: fmt.Errorf("no other SMCPs may be created when a cluster-scoped SMCP exists"),
	},
	{
		name: "creating cluster-wide gateway controller with default CA config map name when multi-tenant SMCP exists - error expected",
		smcp: clusterWideGatewayControllerDefaultCA,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating cluster-wide gateway controller with default CA config map name when multi-tenant SMCP exists - error expected (2nd execution)",
		smcp: clusterWideGatewayControllerDefaultCA,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleMultiTenant),
			NewV2SMCPResource("basic", "istio-system-2", clusterWideGatewayControllerDefaultCA),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating cluster-wide gateway controller with default CA config map name when multi-tenant gateway controller exists - error expected",
		smcp: clusterWideGatewayControllerDefaultCA,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", multiTenantGatewayController),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating cluster-wide gateway controller with default CA config map name when multi-tenant gateway controller exists - error expected (2nd execution)",
		smcp: clusterWideGatewayControllerDefaultCA,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", multiTenantGatewayController),
			NewV2SMCPResource("basic", "istio-system-2", clusterWideGatewayControllerDefaultCA),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating cluster-wide gateway controller with default CA config map name when simple cluster-wide SMCP exists - error expected",
		smcp: clusterWideGatewayControllerDefaultCA,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-2", simpleClusterWide),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating cluster-wide gateway controller with default CA config map name when simple cluster-wide SMCP exists - error expected (2nd execution)",
		smcp: clusterWideGatewayControllerDefaultCA,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", simpleClusterWide),
			NewV2SMCPResource("basic", "istio-system-2", clusterWideGatewayControllerDefaultCA),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating simple cluster-wide SMCP with default CA config map name when cluster-wide gateway controller with default CA config map name exists - error expected",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayControllerDefaultCA),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating simple cluster-wide SMCP with default CA config map name when cluster-wide gateway controller with default CA config map name exists - error expected (2nd execution)",
		smcp: simpleClusterWide,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayControllerDefaultCA),
			NewV2SMCPResource("basic", "istio-system-2", simpleClusterWide),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating multi-tenant SMCP when cluster-wide gateway controller with default CA config map name exists - error expected",
		smcp: simpleMultiTenant,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayControllerDefaultCA),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
	{
		name: "creating multi-tenant SMCP when cluster-wide gateway controller with default CA config map name exists - error expected (2nd execution)",
		smcp: simpleMultiTenant,
		existingObjs: []*maistrav2.ServiceMeshControlPlane{
			NewV2SMCPResource("basic", "istio-system-1", clusterWideGatewayControllerDefaultCA),
			NewV2SMCPResource("basic", "istio-system-2", simpleMultiTenant),
		},
		expectedErr: fmt.Errorf("cannot create cluster-wide SMCP with overlapping caCertConfigMapName"),
	},
}

func TestValidateV2(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := newFakeClient(tc.existingObjs)
			v := versionStrategyV2_5{Ver: V2_5}
			err := v.ValidateV2(context.TODO(), c, &metav1.ObjectMeta{Name: "basic", Namespace: "istio-sytem", UID: uuids[tc.smcp]}, tc.smcp)

			if tc.expectedErr == nil {
				assert.Nil(err, "unexpected error occurred", t)
			} else {
				if err == nil {
					t.Errorf("expected error '%s', but no error was returned", tc.expectedErr)
				} else {
					assert.Equals(err.Error(), tc.expectedErr.Error(), "unexpected error occurred", t)
				}
			}
		})
	}
}

func newSmcpSpec(specYaml string) *maistrav2.ControlPlaneSpec {
	smcp := &maistrav2.ControlPlaneSpec{}
	err := yaml.Unmarshal([]byte(specYaml), smcp)
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %v", err))
	}
	return smcp
}

type fakeClient struct {
	objects map[*metav1.ObjectMeta]runtime.Object
}

func newFakeClient(smcps []*maistrav2.ServiceMeshControlPlane) *fakeClient {
	objects := map[*metav1.ObjectMeta]runtime.Object{}
	for _, smcp := range smcps {
		objects[&metav1.ObjectMeta{Name: smcp.Name, Namespace: smcp.Namespace}] = smcp
	}
	return &fakeClient{objects}
}

func (f fakeClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	if val, exists := f.objects[&metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}]; exists {
		obj = val
	}
	return nil
}

func (f fakeClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	switch l := list.(type) {
	case *maistrav2.ServiceMeshControlPlaneList:
		for _, obj := range f.objects {
			switch o := obj.(type) {
			case *maistrav2.ServiceMeshControlPlane:
				l.Items = append(l.Items, *o)
			}
		}
		list = l
	default:
		panic("unsupported resource")
	}
	return nil
}

func (f fakeClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	panic("implement me")
}

func (f fakeClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	panic("implement me")
}

func (f fakeClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	panic("implement me")
}

func (f fakeClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	panic("implement me")
}

func (f fakeClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

func (f fakeClient) Status() client.StatusWriter {
	panic("implement me")
}
