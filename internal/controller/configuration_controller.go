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
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"golab.io/kubedredger/internal/configfile"
	"golab.io/kubedredger/internal/nodelabel"
	"golab.io/kubedredger/internal/validate"
)

const (
	Finalizer = "workshop.golab.io/finalizer2025"
)

// ConfigurationReconciler reconciles a Configuration object
type ConfigurationReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	ConfMgr  *configfile.Manager
	Labeller *nodelabel.Manager
}

// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;update;patch;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lh := logf.FromContext(ctx)

	conf := workshopv1alpha1.Configuration{}
	err := r.Get(ctx, req.NamespacedName, &conf)
	if err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = validate.Request(conf.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !conf.DeletionTimestamp.IsZero() {
		// Deletion
		if controllerutil.ContainsFinalizer(&conf, Finalizer) {
			err = r.syncForDelete(ctx, lh, &conf)
			if err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&conf, Finalizer)
			err = r.Update(ctx, &conf)
			return ctrl.Result{}, err
		}
	}

	// Add or Update
	if !controllerutil.ContainsFinalizer(&conf, Finalizer) {
		controllerutil.AddFinalizer(&conf, Finalizer)
		err = r.Update(ctx, &conf)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	err = r.syncForAddOrUpdate(ctx, lh, &conf)
	return ctrl.Result{}, err
}

func (r *ConfigurationReconciler) syncForDelete(ctx context.Context, lh logr.Logger, conf *workshopv1alpha1.Configuration) error {
	lh.Info("handling deletion requesi", "config", conf.Spec.Filename)
	err := r.ConfMgr.Delete(conf.Spec.Filename)
	if err != nil {
		return fmt.Errorf("failed to delete the configuration %q: %w", conf.Spec.Filename, err)
	}

	lh.Info("removing node label", "config", conf.Spec.Filename)
	err = r.Labeller.Clear(ctx, nodelabel.MakeContentHashLabel(conf.Spec.Filename))
	if err != nil {
		return fmt.Errorf("failed to delete the node label %q: %w", conf.Spec.Filename, err)
	}
	return nil
}

func (r *ConfigurationReconciler) syncForAddOrUpdate(ctx context.Context, lh logr.Logger, conf *workshopv1alpha1.Configuration) error {
	oldStatus := conf.Status.DeepCopy()
	configurationRequest := configurationRequestFromSpec(conf.Spec)

	err := r.ConfMgr.HandleSync(lh, configurationRequest)
	if errors.As(err, &configfile.NonRecoverableError{}) {
		lh.Error(err, "Non-recoverable error handling configuration")
		return nil
	}

	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(conf.Spec.Content)))[:60]
	err = r.Labeller.Set(ctx, nodelabel.MakeContentHashLabel(conf.Spec.Filename), contentHash)
	lh.Info("labelled node", "value", contentHash, "error", err)

	confStatus := r.ConfMgr.Status(configurationRequest.Filename)
	lh.Info("file status", "fileName", configurationRequest.Filename, "status", confStatus)
	conf.Status = statusFromConfStatus(conf.Spec, confStatus, err)

	if !statusesAreEqual(oldStatus, &conf.Status) {
		updErr := r.Client.Status().Update(ctx, conf)
		if updErr != nil && !apierrors.IsNotFound(updErr) {
			lh.Error(updErr, "Failed to update configuration status")
			return fmt.Errorf("could not update status for object %s: %w", client.ObjectKeyFromObject(conf), updErr)
		}
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workshopv1alpha1.Configuration{}).
		Named("configuration").
		Complete(r)
}
