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
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
)

// ConfigurationReconciler reconciles a Configuration object
type ConfigurationReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ConfigurationRoot string
	LastPath          string
}

// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	conf := workshopv1alpha1.Configuration{}
	err := r.Get(ctx, req.NamespacedName, &conf)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	fullPath := filepath.Join(r.ConfigurationRoot, conf.Spec.Path)
	if r.LastPath != "" && r.LastPath != fullPath {
		klog.InfoS("configuration path changed", "lastPath", r.LastPath, "path", fullPath)
		err = os.Remove(r.LastPath)
		if err != nil {
			return ctrl.Result{}, err
		}
		klog.InfoS("configuration path cleaned", "path", r.LastPath)
	}

	err = manageConfigurationFile(fullPath, conf.Spec)

	if err == nil {
		r.LastPath = fullPath
		klog.InfoS("configuration path registered", "path", fullPath)
	}

	return ctrl.Result{}, err
}

func manageConfigurationFile(fullPath string, confSpec workshopv1alpha1.ConfigurationSpec) (err error) {
	if confSpec.Create {
		return createConfigurationFile(fullPath, confSpec.Content, confSpec.Permission)
	}
	return updateConfigurationFile(fullPath, confSpec.Content)
}

func createConfigurationFile(confPath string, content string, confPerm *uint32) error {
	klog.InfoS("creating configuration file")
	perm := fs.FileMode(0644)
	if confPerm != nil {
		perm = fs.FileMode(*confPerm)
	}
	file, err := os.OpenFile(confPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return reconcile.TerminalError(fmt.Errorf("asked to create an existing path: %q", confPath))
		}
		return fmt.Errorf("cannot open %q: %w", confPath, err)
	}
	// File was created successfully. Write content.
	defer file.Close()
	if _, err = file.WriteString(content); err != nil {
		// Clean up the partially written file on error, but keep the original error
		_ = os.Remove(confPath)
		return fmt.Errorf("error writing the content in %q: %w", confPath, err)
	}
	return nil
}

func updateConfigurationFile(confPath string, content string) (err error) {
	klog.InfoS("updating configuration file")
	tmpFile, err := os.CreateTemp(filepath.Dir(confPath), "kubedredger-")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmpFile.Name())
		}
	}()
	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Rename(tmpFile.Name(), confPath); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workshopv1alpha1.Configuration{}).
		Named("configuration").
		Complete(r)
}
