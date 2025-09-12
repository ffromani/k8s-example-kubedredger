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
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workshopv1alpha1 "golab.io/kubedredger/api/v1alpha1"
	"golab.io/kubedredger/internal/configfile"
	"golab.io/kubedredger/internal/status"
)

// ConfigurationReconciler reconciles a Configuration object
type ConfigurationReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	ConfMgr *configfile.Manager
}

// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workshop.golab.io,resources=configurations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;update;patch

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
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	oldStatus := conf.Status.DeepCopy()

	res := r.ConfMgr.Handle(lh, conf.Spec)

	conf.Status, err = statusFromResponse(res)

	if status.NeedsUpdate(oldStatus, &conf.Status) {
		conf.Status.LastUpdated = metav1.Time{Time: time.Now()}
		updErr := r.Client.Status().Update(ctx, &conf)
		if updErr != nil {
			lh.Error(updErr, "Failed to update configuration status")
			return ctrl.Result{}, fmt.Errorf("could not update status for object %s: %w", client.ObjectKeyFromObject(&conf), updErr)
		}
	}
	return ctrl.Result{}, err
}

func statusFromResponse(res configfile.Result) (workshopv1alpha1.ConfigurationStatus, error) {
	condType := status.ConditionAvailable
	err := res.Error
	if res.Error != nil {
		condType = status.ConditionDegraded
		if !res.Retryable {
			err = reconcile.TerminalError(res.Error)
		}
	}

	return workshopv1alpha1.ConfigurationStatus{
		Content:    res.Content,
		FileExists: res.FileExists,
		Conditions: status.NewConditions(time.Now(), condType, status.ReasonFromError(res.Error), status.MessageFromError(res.Error)),
	}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workshopv1alpha1.Configuration{}).
		Named("configuration").
		Complete(r)
}
