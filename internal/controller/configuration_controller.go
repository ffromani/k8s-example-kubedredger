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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"golab.io/kubedredger/internal/configfile"
	"golab.io/kubedredger/internal/nodelabel"
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
	if apierrors.IsNotFound(err) {
		if err := r.ConfMgr.Delete(); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete the configuration: %w", err)
		}
		return ctrl.Result{}, nil
	}

	if err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	oldStatus := conf.Status.DeepCopy()
	configurationRequest := configurationRequestFromSpec(conf.Spec)

	err = r.ConfMgr.HandleSync(lh, configurationRequest)
	if errors.As(err, &configfile.NonRecoverableError{}) {
		lh.Error(err, "Non-recoverable error handling configuration")
		return ctrl.Result{}, nil
	}

	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(conf.Spec.Content)))[:60]
	err = r.Labeller.Set(ctx, nodelabel.ContentHash, contentHash)
	lh.Info("labelled node", "value", contentHash, "error", err)

	confStatus := r.ConfMgr.Status()
	conf.Status = statusFromConfStatus(conf.Spec, confStatus, err)

	if !statusesAreEqual(oldStatus, &conf.Status) {
		updErr := r.Client.Status().Update(ctx, &conf)
		if updErr != nil {
			lh.Error(updErr, "Failed to update configuration status")
			return ctrl.Result{}, fmt.Errorf("could not update status for object %s: %w", client.ObjectKeyFromObject(&conf), updErr)
		}
	}
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workshopv1alpha1.Configuration{}).
		Named("configuration").
		Complete(r)
}
