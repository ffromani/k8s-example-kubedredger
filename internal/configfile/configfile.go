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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
)

const DefaultPermission = 644

type ConfigurationStatus struct {
	LastWriteFailed bool
	LastWriteError  string
	FileExists      bool
	Content         string
	FileUpdated     time.Time
}

type Manager struct {
	path            string
	lastWriteFailed bool
	lastWriteError  string
}

func NewManager(configurationPath string) *Manager {
	return &Manager{
		path: configurationPath,
	}
}

type NonRecoverableError struct {
	msg string
}

func (e NonRecoverableError) Error() string {
	return e.msg
}

type ConfigRequest struct {
	Content    string
	Create     bool
	Permission *uint32
}

func (mgr *Manager) HandleSync(lh logr.Logger, request ConfigRequest) error {
	err := mgr.handle(lh, request)
	if err != nil {
		mgr.lastWriteError = err.Error()
		mgr.lastWriteFailed = true
		return err
	}
	mgr.lastWriteError = ""
	mgr.lastWriteFailed = false
	return nil
}

// Handle reconciles the on-disk configuration with the given spec.
func (mgr *Manager) handle(lh logr.Logger, request ConfigRequest) error {
	content := request.Content
	exists, err := fileExists(mgr.path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}

	if !exists && !request.Create {
		return NonRecoverableError{msg: fmt.Sprintf("file %q does not exist and creation is not allowed", mgr.path)}
	}

	lh.Info("creating temporary configuration file")

	tmpFile, err := os.CreateTemp(filepath.Dir(mgr.path), "kubedredger-")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	lh.Info("updating temporary configuration file")
	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	perm := fs.FileMode(0644)
	if request.Permission != nil {
		perm = fs.FileMode(*request.Permission)
	}
	fmt.Println("FEDE setting permissions", perm)
	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set permissions on temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Rename(tmpFile.Name(), mgr.path); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// Delete removes the configuration file at the manager's path.
func (mgr *Manager) Delete() error {
	err := os.Remove(mgr.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete file %q: %w", mgr.path, err)
	}
	return nil
}

func (mgr *Manager) Status() ConfigurationStatus {
	res := ConfigurationStatus{
		LastWriteFailed: mgr.lastWriteFailed,
		LastWriteError:  mgr.lastWriteError,
	}
	content, err := os.ReadFile(mgr.path)
	if os.IsNotExist(err) {
		res.FileExists = false
		return res
	}
	res.FileExists = true
	res.Content = string(content)
	return res
}

func fileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("error checking existence of %s: %w", filePath, err)
}
