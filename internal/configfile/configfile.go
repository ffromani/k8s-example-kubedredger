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

package configfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
)

type Manager struct {
	configurationRoot string
	lastPath          string
}

func NewManager(configurationRoot string) *Manager {
	return &Manager{
		configurationRoot: configurationRoot,
	}
}

func (mgr *Manager) GetConfigurationRoot() string {
	return mgr.configurationRoot
}

// Handle reconciles the on-disk configuration with the given spec.
// Returns error if failed, and if so a boolean to tell if the error is retryable.
func (mgr *Manager) Handle(lh logr.Logger, confSpec workshopv1alpha1.ConfigurationSpec) (error, bool) {
	var err error
	retryable := true // optimist by default
	fullPath := filepath.Join(mgr.configurationRoot, confSpec.Path)
	if mgr.lastPath != "" && mgr.lastPath != fullPath {
		lh.Info("configuration path changed", "lastPath", mgr.lastPath, "path", fullPath)
		err = os.Remove(mgr.lastPath)
		if err != nil {
			return err, retryable
		}
		lh.Info("configuration path cleaned", "path", mgr.lastPath)
	}

	if confSpec.Create {
		err, retryable = Create(lh, fullPath, confSpec.Content, confSpec.Permission)
	} else {
		err, retryable = Update(lh, fullPath, confSpec.Content)
	}

	if err == nil {
		mgr.lastPath = fullPath
		lh.Info("configuration path registered", "path", fullPath)
	}
	return err, retryable
}

// Create creates a new configfile on given `confPath` with the given `content`.
// The file must not be existing already.
// If non-nil, `confPerm` are the unix permissions to apply, before umask.
// Returns error if failed, and if so a boolean to tell if the error is retryable.
func Create(lh logr.Logger, confPath string, content string, confPerm *uint32) (error, bool) {
	lh.Info("creating configuration file")
	perm := fs.FileMode(0644)
	if confPerm != nil {
		perm = fs.FileMode(*confPerm)
	}
	file, err := os.OpenFile(confPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("asked to create an existing path: %q", confPath), false
		}
		return fmt.Errorf("cannot open %q: %w", confPath, err), true
	}
	// File was created successfully. Write content.
	defer file.Close()
	if _, err = file.WriteString(content); err != nil {
		// Clean up the partially written file on error, but keep the original error
		_ = os.Remove(confPath)
		return fmt.Errorf("error writing the content in %q: %w", confPath, err), true
	}
	return nil, false
}

// Update updates an existing configfile on given `confPath` with the given `content`.
// The function does not check if the file exists already.
// If non-nil, `confPerm` are the unix permissions to apply, before umask.
// Returns error if failed, and if so a boolean to tell if the error is retryable.
func Update(lh logr.Logger, confPath string, content string) (uerr error, retryable bool) {
	retryable = true // always retryable
	lh.Info("updating configuration file")
	tmpFile, err := os.CreateTemp(filepath.Dir(confPath), "kubedredger-")
	if err != nil {
		uerr = fmt.Errorf("failed to create temporary file: %w", err)
		return
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmpFile.Name())
		}
	}()
	if _, err := tmpFile.WriteString(content); err != nil {
		uerr = fmt.Errorf("failed to write to temporary file: %w", err)
		return
	}
	if err := tmpFile.Close(); err != nil {
		uerr = fmt.Errorf("failed to close temporary file: %w", err)
		return
	}
	if err := os.Rename(tmpFile.Name(), confPath); err != nil {
		uerr = fmt.Errorf("failed to rename temporary file: %w", err)
		return
	}
	return
}
