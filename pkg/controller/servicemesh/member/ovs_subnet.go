package member

import "context"

type subnetStrategy struct{}

var _ NamespaceReconciler = (*subnetStrategy)(nil)

func (*subnetStrategy) reconcileNamespaceInMesh(ctx context.Context, namespace string) error {
	return nil
}

func (*subnetStrategy) removeNamespaceFromMesh(ctx context.Context, namespace string) error {
	return nil
}
