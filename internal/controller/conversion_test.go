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

package controller

import (
	"testing"
	"time"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"golab.io/kubedredger/internal/configfile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConversionDegraded(t *testing.T) {
	fakeTs := time.Now()
	fakeErrText := "fake error for testing"
	var labelErr error // no error
	st := statusFromConfStatus(
		workshopv1alpha1.ConfigurationSpec{},
		configfile.ConfigurationStatus{
			LastWriteError: fakeErrText,
			FileUpdated:    fakeTs,
		},
		labelErr)

	if st.FileExists {
		t.Fatalf("file exists on error")
	}
	cond := findCondition(st.Conditions, ConditionDegraded)
	if cond == nil {
		t.Fatalf("missing degraded condition")
	}
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("condition not set")
	}
	if cond.Reason != ConditionReasonWriteError {
		t.Fatalf("wrong reason: %q", cond.Reason)
	}
}

func TestConversionProgressing(t *testing.T) {
	fakeTs := time.Now()
	var labelErr error // no error
	st := statusFromConfStatus(
		workshopv1alpha1.ConfigurationSpec{
			Content: "foo=1\n",
			Create:  true,
		},
		configfile.ConfigurationStatus{
			LastWriteError: "no space left",
			Content:        "foo=0\n",
			FileExists:     true,
			FileUpdated:    fakeTs,
		},
		labelErr)

	if !st.FileExists {
		t.Fatalf("file does not exist")
	}
	cond := findCondition(st.Conditions, ConditionProgressing)
	if cond == nil {
		t.Fatalf("missing degraded condition")
	}
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("condition not set")
	}
	if cond.Reason != ConditionReasonUpdatingContent {
		t.Fatalf("wrong reason: %q", cond.Reason)
	}
}

func findCondition(conditions []metav1.Condition, condition string) *metav1.Condition {
	for idx := 0; idx < len(conditions); idx++ {
		cond := &conditions[idx]
		if cond.Type == condition {
			return cond
		}
	}
	return nil
}
