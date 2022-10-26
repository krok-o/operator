package controllers

import (
	"context"
	"fmt"

	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func patchObject(ctx context.Context, client client.Client, oldObject, newObject client.Object) error {
	patchHelper, err := patch.NewHelper(oldObject, client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper: %w", err)
	}
	if err := patchHelper.Patch(ctx, newObject); err != nil {
		return fmt.Errorf("failed to patch object: %w", err)
	}
	return nil
}
