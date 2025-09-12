/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package status

import (
	"errors"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
)

const (
	ConditionAvailable   = "Available"
	ConditionProgressing = "Progressing"
	ConditionDegraded    = "Degraded"
	ConditionUpgradeable = "Upgradeable"
)

const (
	ReasonAsExpected    = "AsExpected"
	ReasonInternalError = "InternalError"
)

func NeedsUpdate(oldStatus, newStatus *workshopv1alpha1.ConfigurationStatus) bool {
	os := oldStatus.DeepCopy()
	ns := newStatus.DeepCopy()

	resetIncomparableConditionFields(os.Conditions)
	resetIncomparableConditionFields(ns.Conditions)

	return !reflect.DeepEqual(os, ns)
}

func EqualConditions(current, updated []metav1.Condition) bool {
	c := CloneConditions(current)
	u := CloneConditions(updated)

	resetIncomparableConditionFields(c)
	resetIncomparableConditionFields(u)

	return reflect.DeepEqual(c, u)
}

func FindCondition(conditions []metav1.Condition, condition string) *metav1.Condition {
	for idx := 0; idx < len(conditions); idx++ {
		cond := &conditions[idx]
		if cond.Type == condition {
			return cond
		}
	}
	return nil
}

func NewConditions(ts time.Time, condType string, reason string, message string) []metav1.Condition {
	conditions := NewBaseConditions(ts)
	switch condType {
	case ConditionAvailable:
		if cond := FindCondition(conditions, condType); cond != nil {
			cond.Status = metav1.ConditionTrue
		}
		// Available implies upgradeable
		if cond := FindCondition(conditions, ConditionUpgradeable); cond != nil {
			cond.Status = metav1.ConditionTrue
		}
	case ConditionProgressing:
		fallthrough
	case ConditionDegraded:
		if cond := FindCondition(conditions, condType); cond != nil {
			cond.Status = metav1.ConditionTrue
			cond.Reason = reason
			cond.Message = message
		}
	}
	return conditions
}

func NewBaseConditions(now time.Time) []metav1.Condition {
	return []metav1.Condition{
		{
			Type:               ConditionAvailable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             ConditionAvailable,
		},
		{
			Type:               ConditionUpgradeable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             ConditionUpgradeable,
		},
		{
			Type:               ConditionProgressing,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             ConditionProgressing,
		},
		{
			Type:               ConditionDegraded,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             ConditionDegraded,
		},
	}
}

func ReasonFromError(err error) string {
	if err == nil {
		return ReasonAsExpected
	}
	return ReasonInternalError
}

func MessageFromError(err error) string {
	if err == nil {
		return ""
	}
	unwErr := errors.Unwrap(err)
	if unwErr == nil {
		return err.Error()
	}
	return unwErr.Error()
}

func resetIncomparableConditionFields(conditions []metav1.Condition) {
	for idx := range conditions {
		conditions[idx].LastTransitionTime = metav1.Time{}
		conditions[idx].ObservedGeneration = 0
	}
}

func CloneConditions(conditions []metav1.Condition) []metav1.Condition {
	var c = make([]metav1.Condition, len(conditions))
	copy(c, conditions)
	return c
}
