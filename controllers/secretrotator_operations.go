package controllers

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	"artifactory-secrets-rotator/internal/handler"
	operations "artifactory-secrets-rotator/internal/operations"
	resource "artifactory-secrets-rotator/internal/resource"
	"errors"
	"fmt"
	"sort"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// InitializeResource initializes the secret rotator object and validates specs
func (r *SecretRotatorReconciler) InitializeResource(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, req ctrl.Request) error {

	// if this is the first secret we are updating this reconciliation, lets get a new token
	if err := r.HandleConditions(ctx, secretRotator, req); err != nil {
		return err
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	if err := r.HandleFinalizers(ctx, secretRotator); err != nil {
		return err
	}

	// if this is the first secret we are updating this reconciliation, lets get a new token
	if err := r.SecretRotatorChecker(ctx, secretRotator, req); err != nil {
		return err
	}

	p := client.MergeFrom(secretRotator.DeepCopy())
	defer r.DeferPatch(ctx, secretRotator, p)

	// ValidateObjectSpec method validates CR spec is correct
	if err := operations.ValidateObjectSpec(ctx, tokenDetails, secretRotator, r.Client); err != nil {
		return err
	}

	// HandleCerts method copies certificates into the container
	if err := resource.HandleCerts(ctx, secretRotator.Spec.Security.SecretNamespace, secretRotator.Spec.Security.CertificateSecretName, secretRotator.Name, r.Client); err != nil {
		return err
	}
	return nil
}

// ManagingSecrets is validating the desired state versus the actual state of secrets and creating or updating secrets.
func (r *SecretRotatorReconciler) ManagingSecrets(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, req ctrl.Request) error {

	// It is checking namespaces and deleting our secrets from namespaces that are no lnger selected through the namespace selector
	tokenDetails.FailedNamespaces = resource.DeleteOutdatedSecrets(ctx, tokenDetails.NamespaceList, tokenDetails.SecretName, secretRotator.Name, secretRotator.Status.ProvisionedNamespaces, r.Client)
	tokenDetails.RequeueInterval = r.RequeueInterval

	for _, namespace := range tokenDetails.NamespaceList.Items {

		// GetSecret retrieves the provided secret and returns a secret object.
		existingSecret, err := resource.GetSecret(ctx, namespace.Name, tokenDetails.SecretName, r.Client)
		if err != nil && !apierrors.IsNotFound(err) {
			r.Log.Error(err, "will jump to next namespace, could not get existing Secret", "namespace", namespace)
			tokenDetails.FailedNamespaces[namespace.Name] = err
			continue
		}

		if err == nil && !resource.IsSecretOwnedBy(existingSecret, secretRotator.Name) {
			r.Log.Info("secret is not owned by us, delete it manually if you want this operator to control it")
			tokenDetails.FailedNamespaces[namespace.Name] = fmt.Errorf("secret is not owned by us")
			continue
		}

		// if this is the first secret we are updating this reconciliation, lets get a new token
		if err := handler.HandlingToken(ctx, tokenDetails, secretRotator, r.Recorder); err != nil {
			return err
		}

		// CreateOrUpdateSecret handling secrets with new or latest token
		if err := resource.CreateOrUpdateSecret(req, ctx, tokenDetails, secretRotator, namespace, r.Client, r.Scheme); err != nil {
			r.Log.Error(err, "failed to create or update secret")
			tokenDetails.FailedNamespaces[namespace.Name] = err
			continue
		}

		tokenDetails.ProvisionedNamespaces = append(tokenDetails.ProvisionedNamespaces, namespace.Name)
	}
	return nil
}

// UpdateStatus update the custom resource status
func (r *SecretRotatorReconciler) UpdateStatus(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator) error {

	// The following implementation will update the status after reconciliation completed
	meta.SetStatusCondition(&secretRotator.Status.Conditions, metav1.Condition{Type: operations.TypeAvailableSecretRotator,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Updating of Secret %s in namespaces with label %s created successfully", tokenDetails.SecretName, tokenDetails.NamespaceSelector)})

	// ToNamespaceFailures iterates through failed namespaces and returns a list with failure reason
	secretRotator.Status.FailedNamespaces = resource.ToNamespaceFailures(tokenDetails.FailedNamespaces)

	// Sorting ProvisionedNamespaces to update in status
	sort.Strings(tokenDetails.ProvisionedNamespaces)
	secretRotator.Status.ProvisionedNamespaces = tokenDetails.ProvisionedNamespaces

	// Update status for resource
	if err := r.Status().Update(ctx, secretRotator); err != nil {
		return &operations.ReconcileError{Message: "Failed to update SecretRotator status", Cause: err, RetryIn: 1 * time.Minute}
	}
	r.Recorder.Eventf(secretRotator, "Normal", "Secret rotated successfully", "")

	return nil
}

// HandleConditions handling kubernetes conditions for secret rotator object
func (r *SecretRotatorReconciler) HandleConditions(ctx context.Context, secretRotator *jfrogv1alpha1.SecretRotator, req ctrl.Request) error {
	var err error

	// Let's set the status as Unknown when no status are available
	if secretRotator.Status.Conditions == nil || len(secretRotator.Status.Conditions) == 0 {

		meta.SetStatusCondition(&secretRotator.Status.Conditions, metav1.Condition{Type: operations.TypeAvailableSecretRotator, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, secretRotator); err != nil {
			return &operations.ReconcileError{Message: fmt.Sprintf("Failed to update secretRotator status, exiting reconciliation. secret rotator: `%s`", secretRotator.Name), Cause: err}
		}

		if err := r.Get(ctx, req.NamespacedName, secretRotator); err != nil {
			return &operations.ReconcileError{Message: "Failed to re-fetch secretRotator, the reconciliation will not run", Cause: err}
		}
	}
	return nil
}

// HandleFinalizers handling kubernetes finalizers for secret rotator object
func (r *SecretRotatorReconciler) HandleFinalizers(ctx context.Context, secretRotator *jfrogv1alpha1.SecretRotator) error {

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	if !controllerutil.ContainsFinalizer(secretRotator, operations.SecretRotatorFinalizer) {
		r.Log.Info("Adding Finalizer for secretRotator")
		if ok := controllerutil.AddFinalizer(secretRotator, operations.SecretRotatorFinalizer); !ok {
			return &operations.ReconcileError{Message: "Failed to add finalizer into the custom resource, requeuing", RetryIn: 1 * time.Minute}
		}

		if err := r.Update(ctx, secretRotator); err != nil {
			return &operations.ReconcileError{Message: "Failed to update custom resource to add finalizer, requeuing", RetryIn: 1 * time.Minute, Cause: err}
		}
	}
	return nil
}

// SecretRotatorChecker Check if the SecretRotator instance is marked to be deleted, which is
// indicated by the deletion timestamp being set.
func (r *SecretRotatorReconciler) SecretRotatorChecker(ctx context.Context, secretRotator *jfrogv1alpha1.SecretRotator, req ctrl.Request) error {

	logger := log.FromContext(ctx)
	var err error
	// Check if the SecretRotator instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isSecretRotatorMarkedToBeDeleted := secretRotator.GetDeletionTimestamp() != nil

	if isSecretRotatorMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(secretRotator, operations.SecretRotatorFinalizer) {
			logger.Info("Performing Finalizer Operations for secretRotator before delete CR")

			// Let's add here a status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&secretRotator.Status.Conditions, metav1.Condition{Type: operations.TypeDegradedSecretRotator,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", secretRotator.Name)})

			if err := r.Status().Update(ctx, secretRotator); err != nil {
				return &operations.ReconcileError{Message: "Failed to update SecretRotator status", Cause: err}
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.DoFinalizerOperationsForSecretRotator(secretRotator)

			// Re-fetch the Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and so that we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, secretRotator); err != nil {
				return &operations.ReconcileError{Message: "Failed to re-fetch secretRotator", Cause: err}
			}

			meta.SetStatusCondition(&secretRotator.Status.Conditions, metav1.Condition{Type: operations.TypeDegradedSecretRotator,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", secretRotator.Name)})

			if err := r.Status().Update(ctx, secretRotator); err != nil {
				return &operations.ReconcileError{Message: "Failed to update SecretRotator status", Cause: err}
			}

			r.Log.Info("Removing Finalizer for SecretRotator after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(secretRotator, operations.SecretRotatorFinalizer); !ok {
				return &operations.ReconcileError{Message: "Failed to remove finalizer for SecretRotator", Cause: err}
			}

			if err := r.Update(ctx, secretRotator); err != nil {
				return &operations.ReconcileError{Message: "Failed to update - remove finalizer for secretRotator", Cause: err}
			}
		}
	}
	return nil
}

// DeferPatch patches the status
func (r *SecretRotatorReconciler) DeferPatch(ctx context.Context, secretRotator *jfrogv1alpha1.SecretRotator, p client.Patch) {
	if err := r.Status().Patch(ctx, secretRotator, p); err != nil {
		r.Log.Error(err, "unable to patch status")
	}
}

// DoFinalizerOperationsForSecretRotator updating k8s event
func (r *SecretRotatorReconciler) DoFinalizerOperationsForSecretRotator(secretRotator *jfrogv1alpha1.SecretRotator) {
	r.Recorder.Event(secretRotator, "Warning", "Deleting", fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s", secretRotator.Name, secretRotator.Namespace))
}

// handleError converts an error into reconcile result
func (r *SecretRotatorReconciler) handleError(err error) (ctrl.Result, error) {
	var status *operations.ReconcileError
	if !errors.As(err, &status) {
		r.Log.Error(err, "Reconcile terminated")
		return ctrl.Result{}, err
	}
	if status.Cause == nil {
		r.Log.Error(status, status.Message)
	} else {
		r.Log.Error(status.Cause, status.Message)
	}
	if status.RetryIn == 0*time.Minute {
		r.Log.Info("Reconcile stopped")
		return ctrl.Result{}, nil
	}
	r.Log.Info("Reconcile stopped, will retry in", "next iteration", status.RetryIn)
	return ctrl.Result{RequeueAfter: status.RetryIn}, nil
}
