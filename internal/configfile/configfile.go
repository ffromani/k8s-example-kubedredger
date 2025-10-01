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

// Package configfile provide facilities to create, update and delete
// configuration file. It is meant to be used by the controller.
package configfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
)

// DefaultPermission is the default UNIX permission expressed in octal form
// the file(s) will be using. Example: 644 (rw-r--r--)
const DefaultPermission = 644

// ConfigurationStatus represents informations about how the last configuration
// sync went
type ConfigurationStatus struct {
	// LastWriteError is the last occurred error in human-friendly way
	LastWriteError string
	// FileExists is true if the file was created. Note this is true even if the content is out of sync
	FileExists bool
	// Content is a mirror of the last content written on storage
	Content string
	// FileUpdate is a timestamp of the last time the file was successfully updated
	FileUpdated time.Time
}

// Manager represent an object capable of storing the configuration on a given path
type Manager struct {
	path string
	errs map[string]error
}

// NewManager creates a Manager owning a given <configurationPath>
// When a Manager is created, the assumption is it becomes the sole and only
// owner of the path. The user must not assume the Manager will ignore content
// he didn't create: being the sole owner of a path, it can cancel data
// at any time and change according to its policies.
// The manager will guarantee data is stored in the configuration files.
func NewManager(configurationPath string) *Manager {
	return &Manager{
		path: configurationPath,
		errs: make(map[string]error),
	}
}

// NonRecoverableError is an error which can't be retried. Parameters must change.
type NonRecoverableError struct {
	err error
}

func (e NonRecoverableError) Error() string {
	return e.err.Error()
}

func (mgr *Manager) CleanAll(lh logr.Logger) error {
	entries, err := os.ReadDir(mgr.path)
	if err != nil {
		lh.Info("configuration root missing, recreating", "configRoot", mgr.path)
		if os.IsNotExist(err) {
			return os.MkdirAll(mgr.path, 0755)
		}
		return err
	}

	entryNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryNames = append(entryNames, entry.Name())
	}

	err = mgr.CleanEntries(entryNames...)
	if err != nil {
		return NonRecoverableError{
			err: err,
		}
	}
	return nil
}

func (mgr *Manager) CleanEntries(entries ...string) error {
	var errs []error
	for _, entry := range entries {
		entryPath := filepath.Join(mgr.path, entry)
		err := os.RemoveAll(entryPath)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ConfigRequest represents a request to write configuration on storage.
type ConfigRequest struct {
	Filename   string
	Content    string
	Create     bool
	Permission *uint32
}

// HandleSync reconciles the on-disk configuration with the given request.
// Once it returns, the operation is completed.
// On failure, returns non-nil error; on success, returns nil
func (mgr *Manager) HandleSync(lh logr.Logger, request ConfigRequest) error {
	err := mgr.handle(lh, request)
	if err != nil {
		mgr.errs[request.Filename] = err
		return err
	}
	return nil
}

func (mgr *Manager) handle(lh logr.Logger, request ConfigRequest) error {
	content := request.Content
	fullPath := filepath.Join(mgr.path, request.Filename)
	exists, err := FileExists(fullPath)
	if err != nil {
		return fmt.Errorf("failed to check if file %q exists: %w", fullPath, err)
	}

	if !exists && !request.Create {
		return NonRecoverableError{
			err: fmt.Errorf("file %q does not exist and creation is not allowed", mgr.path),
		}
	}

	lh.Info("creating temporary configuration file", "path", fullPath)

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
	lh.Info("setting permissions", "perms", perm)
	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set permissions on temporary file: %w", err)
	}

	lh.Info("finalizing file content")
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Rename(tmpFile.Name(), fullPath); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	lh.Info("configuration updated")
	return nil
}

// Delete removes the configuration file at the manager's path.
func (mgr *Manager) Delete(fileName string) error {
	fullPath := filepath.Join(mgr.path, fileName)
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		delete(mgr.errs, fileName)
		return nil
	}
	if err != nil {
		mgr.errs[fileName] = err
		return fmt.Errorf("failed to delete file %q: %w", fullPath, err)
	}
	delete(mgr.errs, fileName)
	return nil
}

// Status reports how the last sync attempt went.
func (mgr *Manager) Status(fileName string) ConfigurationStatus {
	fullPath := filepath.Join(mgr.path, fileName)
	res := ConfigurationStatus{}
	if err := mgr.errs[fileName]; err != nil {
		res.LastWriteError = err.Error()
	}
	finfo, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		res.FileExists = false
		return res
	}
	res.FileUpdated = finfo.ModTime()
	content, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		res.FileExists = false
		return res
	}
	res.FileExists = true
	res.Content = string(content)
	return res
}

// FileExists return true if the given path exists;
// On failure, returns non-nil error and the truth value should be ignored.
func FileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("error checking existence of %s: %w", filePath, err)
}
