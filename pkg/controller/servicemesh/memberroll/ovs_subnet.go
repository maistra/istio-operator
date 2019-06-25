package memberroll

type subnetStrategy struct {
}

var _ networkingStrategy = (*subnetStrategy)(nil)

func (*subnetStrategy) reconcileNamespaceInMesh(namespace string) error {
	return nil
}

func (*subnetStrategy) removeNamespaceFromMesh(namespace string) error {
	return nil
}
