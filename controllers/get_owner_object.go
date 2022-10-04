package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetParentObject returns the first find in the owner references of a given object.
// T denotes the type that the user wants returned.
func GetParentObject(ctx context.Context, c client.Client, kind, group string, source, parent client.Object) error {
	for _, ref := range source.GetOwnerReferences() {
		if ref.Kind != kind {
			continue
		}

		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return fmt.Errorf("failed to parse group version: %w", err)
		}

		if gv.Group != group {
			continue
		}

		key := client.ObjectKey{
			Namespace: source.GetNamespace(),
			Name:      ref.Name,
		}

		if err := c.Get(ctx, key, parent); err != nil {
			return fmt.Errorf("failed to get parent Source: %w", err)
		}

		return nil
	}

	return fmt.Errorf("parent not found")
}
