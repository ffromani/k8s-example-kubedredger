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

package validate

import (
	"testing"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestRequest(t *testing.T) {
	type testCase struct {
		name        string
		spec        workshopv1alpha1.ConfigurationSpec
		expectedErr error
	}

	testCases := []testCase{
		{
			name:        "empty",
			spec:        workshopv1alpha1.ConfigurationSpec{},
			expectedErr: ErrMissingFilename,
		},
		{
			name: "good",
			spec: workshopv1alpha1.ConfigurationSpec{
				Filename:   "fooconf.json",
				Content:    "{}",
				Create:     true,
				Permission: ptr.To[uint32](0644),
			},
			expectedErr: nil,
		},
		{
			name: "empty content is fine",
			spec: workshopv1alpha1.ConfigurationSpec{
				Filename:   "fooconf.json",
				Create:     true,
				Permission: ptr.To[uint32](0644),
			},
			expectedErr: nil,
		},
		{
			name: "bad permissions",
			spec: workshopv1alpha1.ConfigurationSpec{
				Filename:   "fooconf.json",
				Create:     true,
				Permission: ptr.To[uint32](0xCAFECAFE),
			},
			expectedErr: ErrInvalidPermission,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			gotErr := Request(tcase.spec)
			if gotErr == nil && tcase.expectedErr == nil {
				return
			}
			if gotErr == tcase.expectedErr {
				return
			}
			t.Errorf("unexpected error got=%v expected=%v", gotErr, tcase.expectedErr)
		})
	}
}
