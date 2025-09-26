package controller

import (
	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"golab.io/kubedredger/internal/configfile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	ConditionAvailable   = "Available"
	ConditionProgressing = "Progressing"
	ConditionDegraded    = "Degraded"
)

const (
	ConditionReasonUpToDate        = "UpToDate"
	ConditionReasonWriteError      = "WriteError"
	ConditionReasonUpdatingContent = "UpdatingContent"
	ConditionReasonUpdatingLabels  = "UpdatingLabels"
)

func configurationRequestFromSpec(desired workshopv1alpha1.ConfigurationSpec) configfile.ConfigRequest {
	res := configfile.ConfigRequest{
		Content: desired.Content,
		Create:  desired.Create,
	}
	if desired.Permission != nil {
		res.Permission = ptr.To(*desired.Permission)
	}
	return res
}

func statusFromConfStatus(desired workshopv1alpha1.ConfigurationSpec, confStatus configfile.ConfigurationStatus, labelErr error) workshopv1alpha1.ConfigurationStatus {
	updateTime := metav1.NewTime(confStatus.FileUpdated)

	res := workshopv1alpha1.ConfigurationStatus{
		FileExists:  confStatus.FileExists,
		LastUpdated: updateTime,
		Content:     confStatus.Content,
	}

	degraded := metav1.Condition{
		Type:               ConditionDegraded,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: updateTime,
	}
	if confStatus.LastWriteError != "" {
		degraded.Status = metav1.ConditionTrue
		degraded.Reason = ConditionReasonWriteError
		degraded.Message = confStatus.LastWriteError
	}

	progressing := metav1.Condition{
		Type:               ConditionProgressing,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: updateTime,
	}
	if desired.Content != confStatus.Content && confStatus.LastWriteError != "" {
		progressing.Status = metav1.ConditionTrue
		progressing.Reason = ConditionReasonUpdatingContent
	} else if labelErr != nil {
		progressing.Status = metav1.ConditionTrue
		progressing.Reason = ConditionReasonUpdatingLabels
		progressing.Message = labelErr.Error()
	}

	available := metav1.Condition{
		Type:               ConditionAvailable,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: updateTime,
	}
	if confStatus.LastWriteError == "" && res.Content == desired.Content && labelErr == nil {
		available.Status = metav1.ConditionTrue
		available.Reason = ConditionReasonUpToDate
		available.Message = "file up to date"
	}
	res.Conditions = []metav1.Condition{degraded, progressing, available}
	return res
}

func statusesAreEqual(a, b *workshopv1alpha1.ConfigurationStatus) bool {
	if a.FileExists != b.FileExists || a.Content != b.Content {
		return false
	}

	if len(a.Conditions) != len(b.Conditions) {
		return false
	}

	for i, condA := range a.Conditions {
		condB := b.Conditions[i]
		if condA.Type != condB.Type ||
			condA.Status != condB.Status ||
			condA.Reason != condB.Reason ||
			condA.Message != condB.Message {
			return false
		}
	}

	return true
}
