/*
Copyright 2022.

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
	"fmt"
	"reflect"
	"testing"
	"time"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewConditions(t *testing.T) {
	type testCase struct {
		name     string
		condType string
		err      error
		expected metav1.Condition
	}

	testCases := []testCase{
		{
			name:     "available",
			condType: ConditionAvailable,
			expected: metav1.Condition{
				Type:   ConditionAvailable,
				Status: metav1.ConditionTrue,
				Reason: ConditionAvailable,
			},
		},
		{
			name:     "progressing",
			condType: ConditionProgressing,
			expected: metav1.Condition{
				Type:   ConditionProgressing,
				Status: metav1.ConditionTrue,
				Reason: ReasonAsExpected,
			},
		},
		{
			name:     "degraded",
			condType: ConditionDegraded,
			expected: metav1.Condition{
				Type:   ConditionDegraded,
				Status: metav1.ConditionTrue,
				Reason: ReasonAsExpected,
			},
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			conds := NewConditions(time.Now(), tcase.condType, ReasonFromError(tcase.err), MessageFromError(tcase.err))
			got := FindCondition(conds, tcase.condType)
			got.LastTransitionTime = metav1.Time{
				Time: time.Time{},
			}
			if !reflect.DeepEqual(*got, tcase.expected) {
				t.Errorf("failure looking for condition %q: got=%#v expected=%#v", tcase.condType, *got, tcase.expected)
			}
		})
	}
}

func TestEqualConditions(t *testing.T) {
	type testCase struct {
		name     string
		condsA   []metav1.Condition
		condsB   []metav1.Condition
		expected bool
	}

	testCases := []testCase{
		{
			// we don't care if the ts has a difference
			name:     "equal available",
			condsA:   NewConditions(time.Now(), ConditionAvailable, ReasonAsExpected, ""),
			condsB:   NewConditions(time.Now(), ConditionAvailable, ReasonAsExpected, ""),
			expected: true,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			got := EqualConditions(tcase.condsA, tcase.condsB)
			if got != tcase.expected {
				t.Fatalf("equality check failed: got=%v expected=%v", got, tcase.expected)
			}
		})
	}
}

func TestNeedsUpdate(t *testing.T) {
	type testCase struct {
		name     string
		statusA  workshopv1alpha1.ConfigurationStatus
		statusB  workshopv1alpha1.ConfigurationStatus
		expected bool
	}

	ts := time.Now()
	pastTS, err := time.Parse("2006-01-02 15:04:05 -0700", "2021-10-07 12:44:22 +0530")
	if err != nil {
		t.Fatalf("error parsif past generic ts: %v", err)
	}

	testCases := []testCase{
		{
			name: "no changes",
			statusA: workshopv1alpha1.ConfigurationStatus{
				LastUpdated: metav1.Time{
					Time: ts,
				},
				Content:    "[main]\nverbose=4\nfoo=bar,baz\n",
				FileExists: true,
				Conditions: NewConditions(ts, ConditionAvailable, ConditionAvailable, ""),
			},
			statusB: workshopv1alpha1.ConfigurationStatus{
				LastUpdated: metav1.Time{
					Time: ts,
				},
				Content:    "[main]\nverbose=4\nfoo=bar,baz\n",
				FileExists: true,
				Conditions: NewConditions(ts, ConditionAvailable, ConditionAvailable, ""),
			},
			expected: false,
		},
		{
			name: "timestamp in conditions doesn't trigger an update",
			statusA: workshopv1alpha1.ConfigurationStatus{
				LastUpdated: metav1.Time{
					Time: ts,
				},
				Content:    "[main]\nverbose=4\nfoo=bar,baz\n",
				FileExists: true,
				Conditions: NewConditions(pastTS, ConditionAvailable, ConditionAvailable, ""),
			},
			statusB: workshopv1alpha1.ConfigurationStatus{
				LastUpdated: metav1.Time{
					Time: ts,
				},
				Content:    "[main]\nverbose=4\nfoo=bar,baz\n",
				FileExists: true,
				Conditions: NewConditions(ts, ConditionAvailable, ConditionAvailable, ""),
			},
			expected: false,
		},
		{
			name: "timestamp in status doesn trigger an update",
			statusA: workshopv1alpha1.ConfigurationStatus{
				LastUpdated: metav1.Time{
					Time: pastTS,
				},
				Content:    "[main]\nverbose=4\nfoo=bar,baz\n",
				FileExists: true,
				Conditions: NewConditions(pastTS, ConditionAvailable, ConditionAvailable, ""),
			},
			statusB: workshopv1alpha1.ConfigurationStatus{
				LastUpdated: metav1.Time{
					Time: ts,
				},
				Content:    "[main]\nverbose=4\nfoo=bar,baz\n",
				FileExists: true,
				Conditions: NewConditions(ts, ConditionAvailable, ConditionAvailable, ""),
			},
			expected: true,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			got := NeedsUpdate(&tcase.statusA, &tcase.statusB)
			if got != tcase.expected {
				t.Fatalf("NeedsUpdate check failed: got=%v expected=%v", got, tcase.expected)
			}
		})
	}
}

func TestFindCondition(t *testing.T) {
	type testCase struct {
		name          string
		desired       string
		conds         []metav1.Condition
		expectedFound bool
	}

	testCases := []testCase{
		{
			name:          "nil conditions",
			desired:       ConditionAvailable,
			expectedFound: false,
		},
		{
			name:          "missing condition",
			desired:       "foobar",
			conds:         NewBaseConditions(time.Now()),
			expectedFound: false,
		},
		{
			name:          "found condition",
			desired:       ConditionProgressing,
			conds:         NewBaseConditions(time.Now()),
			expectedFound: true,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			cond := FindCondition(tcase.conds, tcase.desired)
			found := (cond != nil)
			if found != tcase.expectedFound {
				t.Errorf("failure looking for condition %q: got=%v expected=%v", tcase.desired, found, tcase.expectedFound)
			}
		})
	}
}

func TestReasonFromError(t *testing.T) {
	type testCase struct {
		name     string
		err      error
		expected string
	}

	testCases := []testCase{
		{
			name:     "nil error",
			expected: ReasonAsExpected,
		},
		{
			name:     "simple error",
			err:      errors.New("testing error with simple message"),
			expected: ReasonInternalError,
		},
		{
			name:     "simple formatted error",
			err:      fmt.Errorf("testing error message=%s val=%d", "complex", 42),
			expected: ReasonInternalError,
		},

		{
			name:     "wrapped error",
			err:      fmt.Errorf("outer error: %w", errors.New("inner error")),
			expected: ReasonInternalError,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			got := ReasonFromError(tcase.err)
			if got != tcase.expected {
				t.Errorf("failure getting reason from error: got=%q expected=%q", got, tcase.expected)
			}
		})
	}
}

func TestMessageFromError(t *testing.T) {
	type testCase struct {
		name     string
		err      error
		expected string
	}

	testCases := []testCase{
		{
			name:     "nil error",
			expected: "",
		},
		{
			name:     "simple error",
			err:      errors.New("testing error with simple message"),
			expected: "testing error with simple message",
		},
		{
			name:     "simple formatted error",
			err:      fmt.Errorf("testing error message=%s val=%d", "complex", 42),
			expected: "testing error message=complex val=42",
		},

		{
			name:     "wrapped error",
			err:      fmt.Errorf("outer error: %w", errors.New("inner error")),
			expected: "inner error",
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			got := MessageFromError(tcase.err)
			if got != tcase.expected {
				t.Errorf("failure getting message from error: got=%q expected=%q", got, tcase.expected)
			}
		})
	}
}
