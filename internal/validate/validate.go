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
	"errors"
	"os"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
)

var (
	ErrMissingFilename   = errors.New("filename can't be empty")
	ErrInvalidPermission = errors.New("requested permissions are not a valid UNIX permission set")
)

func Request(spec workshopv1alpha1.ConfigurationSpec) error {
	if spec.Filename == "" {
		return ErrMissingFilename
	}
	if spec.Permission != nil {
		return validPermission(*spec.Permission)
	}
	return nil
}

func validPermission(perm uint32) error {
	// no spurious bits
	if (os.FileMode(perm) & os.ModeType) != 0 {
		return ErrInvalidPermission
	}
	return nil
}
