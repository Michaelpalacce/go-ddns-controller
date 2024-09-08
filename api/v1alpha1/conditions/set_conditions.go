package conditions

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type conditionResource[T any] interface {
	client.Object
	DeepCopy() T
	Conditions() *Conditions
}

// PatchConditions is a helper function to patch the conditions of a resource.
// It works by merging the conditions of the resource with the new condition options.
func PatchConditions[T client.Object](
	ctx context.Context,
	r client.Client,
	res conditionResource[T],
	conditionType string,
	options ...ConditionOption,
) error {
	patch := client.MergeFrom(res.DeepCopy())
	options = append(options, WithObservedGeneration(res.GetGeneration()))
	if res.Conditions().SetCondition(conditionType, options...) {
		err := r.Status().Patch(ctx, res, patch)
		if err != nil {
			return err
		}
	}

	return nil
}
