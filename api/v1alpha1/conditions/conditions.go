package conditions

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Conditions is a struct that holds a list of conditions and a list of condition types that should be present.
// As it uses pointers, you can freely modify the conditions and it will be reflected in the original object.
type Conditions struct {
	Conditions     *[]metav1.Condition `json:"conditions,omitempty"`
	ConditionTypes []string            `json:"conditionTypes"`
}

// FillConditions is used to fill the conditions with the condition types that are specified in ConditionTypes.
// This is so that the conditions are always up to date with the condition types.
func (c *Conditions) FillConditions() bool {
	changed := false
	for _, conditionType := range c.ConditionTypes {
		if c.GetCondition(conditionType) != nil {
			continue
		}

		hasChanged := c.addUnknownCondition(conditionType)
		if hasChanged {
			changed = true
		}
	}

	return changed
}

// GetCondition returns the condition with the specified type.
// If the condition does not exist, it returns nil.
func (c *Conditions) GetCondition(conditionType string) *metav1.Condition {
	if c.Conditions == nil {
		return nil
	}

	for i := range *c.Conditions {
		if (*c.Conditions)[i].Type == conditionType {
			return &(*c.Conditions)[i]
		}
	}

	return nil
}

// SetCondition sets the condition with the specified type to the specified options.
// If the condition does not exist, it will be created with unknown status.
func (c *Conditions) SetCondition(conditionType string, options ...ConditionOption) bool {
	changed := false

	condition := c.GetCondition(conditionType)
	if condition == nil {
		hasChanged := c.addUnknownCondition(conditionType)
		if hasChanged {
			changed = true
			condition = c.GetCondition(conditionType)
		}
	}

	for _, option := range options {
		hasChanged := option(condition)
		if hasChanged {
			changed = true
		}
	}

	if changed {
		condition.LastTransitionTime = metav1.NewTime(time.Now())
	}

	return changed
}

// ================================================ Private Functions ================================================

func (c *Conditions) addUnknownCondition(conditionType string) bool {
	return meta.SetStatusCondition(c.Conditions, metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionUnknown,
		Reason:  "Unknown",
		Message: "Unknown",
	})
}

// ================================================== Condition Option ==================================================

// ConditionOption is a function that modifies a condition.
// Different options can be combined to modify a condition in multiple ways.
// Check out the Functional Options Pattern for more information.
type ConditionOption func(*metav1.Condition) bool

// WithReasonAndMessage is a shorthand for WithReason and WithMessage
// as those two are often used together.
func WithReasonAndMessage(reason, message string) ConditionOption {
	return func(condition *metav1.Condition) bool {
		changed := false
		changed = WithReason(reason)(condition) || changed
		changed = WithMessage(message)(condition) || changed
		return changed
	}
}

func WithMessage(message string) ConditionOption {
	return func(condition *metav1.Condition) bool {
		if condition.Message == message {
			return false
		}

		condition.Message = message
		return true
	}
}

func WithReason(reason string) ConditionOption {
	return func(condition *metav1.Condition) bool {
		if condition.Reason == reason {
			return false
		}

		condition.Reason = reason
		return true
	}
}

func True() ConditionOption {
	return func(condition *metav1.Condition) bool {
		if condition.Status == metav1.ConditionTrue {
			return false
		}

		condition.Status = metav1.ConditionTrue
		return true
	}
}

func False() ConditionOption {
	return func(condition *metav1.Condition) bool {
		if condition.Status == metav1.ConditionFalse {
			return false
		}

		condition.Status = metav1.ConditionFalse
		return true
	}
}

func WithObservedGeneration(generation int64) ConditionOption {
	return func(condition *metav1.Condition) bool {
		if condition.ObservedGeneration == generation {
			return false
		}

		condition.ObservedGeneration = generation
		return true
	}
}
