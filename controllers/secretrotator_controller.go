/*
Copyright 2023.

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

package controllers

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	"artifactory-secrets-rotator/internal/operations"
	"math"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretRotatorReconciler reconciles a SecretRotator object
type SecretRotatorReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	RequeueInterval time.Duration
}

//+kubebuilder:rbac:groups=apps.jfrog.com,resources=secretrotators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.jfrog.com,resources=secretrotators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.jfrog.com,resources=secretrotators/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps;core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps;core,resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps;core,resources=pods,verbs=get
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get,resourceNames=jfrog-operator-sa
//+kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=get;create,resourceNames=jfrog-operator-sa

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecretRotator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *SecretRotatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Starting Artifactory Secret Rotation Reconcile")
	ctx = log.IntoContext(ctx, r.Log)

	var tokenDetails operations.TokenDetails
	secretRotator := &jfrogv1alpha1.SecretRotator{}

	// Fetch the SecretRotator instance
	err := r.Get(ctx, req.NamespacedName, secretRotator)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			r.Log.Info("Secret rotator object not found")
			r.Recorder.Event(secretRotator, "Warning", "MissingResource", fmt.Sprintf("Operator object not found, the reconciliation will not run"))
			return r.handleError(&operations.ReconcileError{Message: "Secret rotator object not found", Cause: err})
		}
		// Error reading the object - requeue the request.
		return r.handleError(&operations.ReconcileError{Message: "Failed to get SecretRotator", Cause: err, RetryIn: 10 * time.Minute})
	}

	// InitializeResource initializes the secret rotator object and validates specs
	if err := r.InitializeResource(ctx, &tokenDetails, secretRotator, req); err != nil {
		r.Recorder.Eventf(secretRotator, "Warning", "Failed in initializing resource", "%s", err)
		return reconcile.Result{RequeueAfter: 1 * time.Second, Requeue: true}, nil
	}

	// ManagingSecrets is validating the desired state versus the actual state of secrets and creating or updating secrets.
	if err := r.ManagingSecrets(ctx, &tokenDetails, secretRotator, req); err != nil {
		r.Recorder.Eventf(secretRotator, "Warning", "Failed in managing secret", "%s", err)
		return reconcile.Result{RequeueAfter: 1 * time.Second, Requeue: true}, nil
	}

	// UpdateStatus, update the custom resource status
	if err := r.UpdateStatus(ctx, &tokenDetails, secretRotator); err != nil {
		r.Recorder.Eventf(secretRotator, "Warning", "Failed in updating status", "%s", err)
		return reconcile.Result{RequeueAfter: 1 * time.Second, Requeue: true}, nil
	}

	// Use fixed interval if configured, otherwise utilize aws role max session time
	if secretRotator.Spec.RefreshInterval == nil {
		r.RequeueInterval = time.Duration(tokenDetails.TTLInSeconds * 0.75 * float64(time.Second))
	} else {
		r.RequeueInterval = secretRotator.Spec.RefreshInterval.Duration
	}
	r.Log.Info("Reconcile completed, see you in", "next iteration", r.RequeueInterval)
	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretRotatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&jfrogv1alpha1.SecretRotator{}).
		WithEventFilter(WatchNsChanges(r)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Owns(&corev1.Namespace{}).
		Complete(r)
}

// WatchNsChanges uses predicates for Event Filtering (namespace creation changes)
func WatchNsChanges(r *SecretRotatorReconciler) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if _, ok := e.Object.(*corev1.Namespace); ok {
				secretRotators := operations.ListSecretRotatorObjects(r.Client)
				difference := e.Object.GetCreationTimestamp().Time.Sub(time.Now())
				for i := range secretRotators.Items {
					if operations.IsExist(e.Object.GetLabels(), secretRotators.Items[i].Spec.NamespaceSelector.MatchLabels) && int(math.Abs(difference.Seconds())) < 20 {
						r.Log.Info("Created new namespace with matching labels to secret rotator object, ", "Namespace name :", e.Object.GetName(), "Secret rotator name :", secretRotators.Items[i].Name)
						if flag := operations.HandlingNamespaceEvents(r.Client, r.Log, &secretRotators.Items[i]); !flag {
							return false
						}
					}
				}
			}
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if _, ok := e.ObjectOld.(*corev1.Namespace); ok {
				if !reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels()) {
					secretRotators := operations.ListSecretRotatorObjects(r.Client)
					for i := range secretRotators.Items {
						if operations.IsExist(e.ObjectNew.GetLabels(), secretRotators.Items[i].Spec.NamespaceSelector.MatchLabels) || operations.IsExist(e.ObjectOld.GetLabels(), secretRotators.Items[i].Spec.NamespaceSelector.MatchLabels) {
							r.Log.Info("Namespace lebels has been changes, ", "Namespace name :", e.ObjectNew.GetName(), "Secret rotator name :", secretRotators.Items[i].Name)
							if flag := operations.HandlingNamespaceEvents(r.Client, r.Log, &secretRotators.Items[i]); !flag {
								return false
							}
						}
					}
				}
			}
			return true
		},
	}
}
