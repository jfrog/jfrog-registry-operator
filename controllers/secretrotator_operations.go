package controllers

import (
	"artifactory-secrets-rotator/api/v1alpha1"
	"artifactory-secrets-rotator/internal/handler"
	"artifactory-secrets-rotator/internal/operations"
	"artifactory-secrets-rotator/internal/resource"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// InitializeResource initializes the secret rotator object and validates specs
func (r *SecretRotatorReconciler) InitializeResource(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *v1alpha1.SecretRotator, req ctrl.Request) error {
	// Handle conditions for the secret rotator
	if err := r.HandleConditions(ctx, secretRotator, req); err != nil {
		return err
	}

	// Add a finalizer to handle cleanup before deletion
	if err := r.HandleFinalizers(ctx, secretRotator); err != nil {
		return err
	}

	// Check if the secret rotator is marked for deletion
	if err := r.SecretRotatorChecker(ctx, secretRotator, req); err != nil {
		return err
	}

	p := client.MergeFrom(secretRotator.DeepCopy())
	defer r.DeferPatch(ctx, secretRotator, p)

	// Validate the SecretRotator spec
	if err := operations.ValidateObjectSpec(ctx, tokenDetails, secretRotator, r.Client); err != nil {
		return err
	}

	// Handle certificates if security is enabled and verification is not skipped
	if secretRotator.Spec.Security.Enabled && !secretRotator.Spec.Security.InsecureSkipVerify {
		if err := resource.HandleCerts(ctx, secretRotator.Spec.Security.SecretNamespace, secretRotator.Spec.Security.CertificateSecretName, secretRotator.Name, r.Client); err != nil {
			return err
		}
	}
	return nil
}

// ManagingSecrets validates the desired state versus the actual state of secrets and creates or updates secrets.
func (r *SecretRotatorReconciler) ManagingSecrets(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *v1alpha1.SecretRotator, req ctrl.Request) error {
	logger := log.FromContext(ctx)

	// Delete outdated secrets from namespaces no longer selected
	tokenDetails.FailedNamespaces = resource.DeleteOutdatedSecrets(ctx, tokenDetails, secretRotator.Name, secretRotator.Status.ProvisionedNamespaces, r.Client)
	tokenDetails.RequeueInterval = r.RequeueInterval
	skippedSecrets := make(map[string]string, 0)
	failedSecrets := []string{}
	tokenDetails.SecretManagedByNamespaces = make(map[string][]string)

	for _, namespace := range tokenDetails.NamespaceList.Items {
		// Iterate over generated secrets, which includes secrets from SecretRotatorSpec.SecretName (appended in ValidateObjectSpec)
		for _, gSecret := range tokenDetails.GeneratedSecrets {

			existingSecret, err := resource.GetSecret(ctx, namespace.Name, gSecret.SecretName, r.Client)
			if err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "Could not get existing secret, skipping namespace", "secretType", gSecret.SecretType, "secret", gSecret.SecretName, "namespace", namespace.Name)
				failedSecrets = append(failedSecrets, fmt.Sprintf("%s (%s) Reason: not found, ", gSecret.SecretName, gSecret.SecretType))
				skippedSecrets[gSecret.SecretName] = namespace.Name
				continue
			}

			if err == nil && !resource.IsSecretOwnedBy(existingSecret, secretRotator.Name) {
				logger.Info("Secret is not owned by this SecretRotator, delete it manually if you want this operator to control it", "secretType", gSecret.SecretType, "secret", gSecret.SecretName, "namespace", namespace.Name)
				failedSecrets = append(failedSecrets, fmt.Sprintf("%s (%s) Reason: not owned by secretrotator, ", gSecret.SecretName, gSecret.SecretType))
				skippedSecrets[gSecret.SecretName] = namespace.Name
				continue
			}
		}

		// Fetch a new token for this reconciliation
		if err := handler.HandlingToken(ctx, tokenDetails, secretRotator, r.Recorder); err != nil {
			logger.Error(err, "Failed to handle token", "namespace", namespace.Name)
			return err
		}

		// Create or update secrets
		for _, gSecret := range tokenDetails.GeneratedSecrets {
			value, isExist := skippedSecrets[gSecret.SecretName]
			if isExist || value == namespace.Name {
				continue
			}
			if err := resource.CreateOrUpdateSecrets(req, ctx, tokenDetails, secretRotator, namespace, r.Client, r.Scheme, gSecret.SecretName, gSecret.SecretType); err != nil {
				logger.Error(err, "Failed to create or update secret", "secretType", gSecret.SecretType, "secret", gSecret.SecretName, "namespace", namespace.Name)
				failedSecrets = append(failedSecrets, fmt.Sprintf("%s (%s) Reason: failed in create/update", gSecret.SecretName, gSecret.SecretType))
				continue
			}
			tokenDetails.SecretManagedByNamespaces[namespace.Name] = append(tokenDetails.SecretManagedByNamespaces[namespace.Name], gSecret.SecretName)
		}
		if len(failedSecrets) > 0 {
			message := fmt.Errorf("Unable to manage secrets %s in namespace %s", strings.Join(failedSecrets, " and "), namespace.Name)
			tokenDetails.FailedNamespaces[namespace.Name] = message
			failedSecrets = []string{}
		}

		// Delete outdated generated secrets from the cluster if they are not present in the current configuration
		if len(secretRotator.Status.SecretManagedByNamespaces) > 0 {
			err := operations.DeleteOutdatedGeneratedSecrets(ctx, tokenDetails, secretRotator, r.Client)
			if err != nil {
				return err
			}
		}
		skippedSecrets = map[string]string{}
		// Mark namespace as provisioned if secrets are successfully created/updated
		tokenDetails.ProvisionedNamespaces = append(tokenDetails.ProvisionedNamespaces, namespace.Name)
		logger.Info("Successfully managed secrets for namespace", "namespace", namespace.Name)
	}

	return nil
}

// UpdateStatus updates the custom resource status
func (r *SecretRotatorReconciler) UpdateStatus(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *v1alpha1.SecretRotator) error {
	// Collect docker and generic secret names
	var dockerSecretNames []string
	var genericSecretNames []string
	secretNames := []string{}

	for _, gSecret := range tokenDetails.GeneratedSecrets {
		if gSecret.SecretType == operations.SecretTypeDocker {
			dockerSecretNames = append(dockerSecretNames, gSecret.SecretName)
		} else if gSecret.SecretType == operations.SecretTypeGeneric {
			genericSecretNames = append(genericSecretNames, gSecret.SecretName)
		}
	}

	if len(dockerSecretNames) != 0 {
		secretNames = append(secretNames, fmt.Sprintf("%s (docker)", dockerSecretNames))

	}
	if len(genericSecretNames) != 0 {
		secretNames = append(secretNames, fmt.Sprintf("%s (generic)", genericSecretNames))

	}

	// Update the status after reconciliation completed
	message := fmt.Sprintf("Secrets %s in namespaces with label %s managed successfully, Please check if any failed namespaces are visible, in order to evaluate individual secret failures, if any.", strings.Join(secretNames, " and "), tokenDetails.NamespaceSelector)
	meta.SetStatusCondition(&secretRotator.Status.Conditions, metav1.Condition{
		Type:    operations.TypeAvailableSecretRotator,
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciling",
		Message: message,
	})

	// ToNamespaceFailures iterates through failed namespaces and returns a list with failure reason
	secretRotator.Status.FailedNamespaces = resource.ToNamespaceFailures(tokenDetails.FailedNamespaces)

	// Sorting ProvisionedNamespaces to update in status
	sort.Strings(tokenDetails.ProvisionedNamespaces)
	secretRotator.Status.ProvisionedNamespaces = tokenDetails.ProvisionedNamespaces
	secretRotator.Status.SecretManagedByNamespaces = tokenDetails.SecretManagedByNamespaces

	// Update status for resource
	if err := r.Status().Update(ctx, secretRotator); err != nil {
		return &operations.ReconcileError{Message: "Failed to update SecretRotator status", Cause: err, RetryIn: 1 * time.Minute}
	}
	r.Recorder.Eventf(secretRotator, "Normal", "Secret rotated successfully", "")

	return nil
}

// HandleConditions handles kubernetes conditions for secret rotator object
func (r *SecretRotatorReconciler) HandleConditions(ctx context.Context, secretRotator *v1alpha1.SecretRotator, req ctrl.Request) error {
	var err error

	// Set the status as Unknown when no status is available
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

// HandleFinalizers handles kubernetes finalizers for secret rotator object
func (r *SecretRotatorReconciler) HandleFinalizers(ctx context.Context, secretRotator *v1alpha1.SecretRotator) error {
	// Add a finalizer to handle cleanup before deletion
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

// SecretRotatorChecker checks if the SecretRotator instance is marked to be deleted
func (r *SecretRotatorReconciler) SecretRotatorChecker(ctx context.Context, secretRotator *v1alpha1.SecretRotator, req ctrl.Request) error {
	logger := log.FromContext(ctx)
	var err error
	// Check if the SecretRotator instance is marked to be deleted
	isSecretRotatorMarkedToBeDeleted := secretRotator.GetDeletionTimestamp() != nil

	if isSecretRotatorMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(secretRotator, operations.SecretRotatorFinalizer) {
			logger.Info("Performing Finalizer Operations for secretRotator before delete CR")

			// Set a "Downgrade" status to indicate the resource is being terminated
			meta.SetStatusCondition(&secretRotator.Status.Conditions, metav1.Condition{Type: operations.TypeDegradedSecretRotator,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", secretRotator.Name)})

			if err := r.Status().Update(ctx, secretRotator); err != nil {
				return &operations.ReconcileError{Message: "Failed to update SecretRotator status", Cause: err}
			}

			// Perform finalizer operations
			r.DoFinalizerOperationsForSecretRotator(secretRotator)

			// Re-fetch the Custom Resource to avoid conflicts
			if err := r.Get(ctx, req.NamespacedName, secretRotator); err != nil {
				return &operations.ReconcileError{Message: "Failed to re-fetch secretRotator", Cause: err}
			}

			meta.SetStatusCondition(&secretRotator.Status.Conditions, metav1.Condition{Type: operations.TypeDegradedSecretRotator,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", secretRotator.Name)})

			if err := r.Status().Update(ctx, secretRotator); err != nil {
				return &operations.ReconcileError{Message: "Failed to update SecretRotator status", Cause: err}
			}

			r.Log.Info("Removing Finalizer for SecretRotator after successfully performing the operations")
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
func (r *SecretRotatorReconciler) DeferPatch(ctx context.Context, secretRotator *v1alpha1.SecretRotator, p client.Patch) {
	if err := r.Status().Patch(ctx, secretRotator, p); err != nil {
		r.Log.Error(err, "unable to patch status")
	}
}

// DoFinalizerOperationsForSecretRotator updates k8s event
func (r *SecretRotatorReconciler) DoFinalizerOperationsForSecretRotator(secretRotator *v1alpha1.SecretRotator) {
	r.Recorder.Event(secretRotator, "Warning", "Deleting", fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s", secretRotator.Name, secretRotator.Namespace))
}

// handleError converts an error into reconcile result
func (r *SecretRotatorReconciler) handleError(err error) (ctrl.Result, error) {
	var status *operations.ReconcileError
	if !errors.As(err, &status) {
		// Convert non-ReconcileError to ReconcileError with default retry
		r.Log.Error(err, "Reconcile terminated")
		return ctrl.Result{}, &operations.ReconcileError{
			Message: "Unexpected error during reconciliation, wait for 10 seconds",
			Cause:   err,
			RetryIn: 10 * time.Second,
		}
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
