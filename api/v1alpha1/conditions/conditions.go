package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Conditions struct {
	Conditions     *[]metav1.Condition `json:"conditions,omitempty"`
	ConditionTypes []string            `json:"conditionTypes"`
}

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

func (c *Conditions) SetCondition(conditionType string, options ...ConditionOption) bool {
	changed := false

	condition := c.GetCondition(conditionType)
	if condition == nil {
		hasChanged := c.addUnknownCondition(conditionType)
		if hasChanged {
			changed = true
		}
	}

	for _, option := range options {
		hasChanged := option(condition)
		if hasChanged {
			changed = true
		}
	}

	return changed
}

// ================================================== Condition Option ==================================================

type ConditionOption func(*metav1.Condition) bool

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

// ================================================ Private Functions ================================================

func (c *Conditions) addUnknownCondition(conditionType string) bool {
	return meta.SetStatusCondition(c.Conditions, metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionUnknown,
		Reason:  "Unknown",
		Message: "Unknown",
	})
}
