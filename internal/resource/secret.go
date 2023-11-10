package resource

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	tokenType "artifactory-secrets-rotator/internal/types"
	"context"
	"encoding/base64"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func IsSecretOwnedBy(secret *v1.Secret, secretOperatorName string) bool {
	owner := metav1.GetControllerOf(secret)

	return owner != nil && owner.APIVersion == jfrogv1alpha1.GroupVersion.String() && owner.Kind == jfrogv1alpha1.SecretKind && owner.Name == secretOperatorName
}
func GetSecret(ctx context.Context, namespace, secretName string, k8sClient client.Client) (*v1.Secret, error) {
	// Should not use esv1beta1.ExternalSecret since we specify builder.OnlyMetadata and cache only metadata
	logger := log.FromContext(ctx)

	secret := &v1.Secret{}
	logger.Info("getting secret with ", "namespace", namespace, "name", secretName)
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, secret)
	return secret, err
}

// DeleteOutdatedSecrets removes=ing secrets created by the controller which are no longer selected by the namesapce selector
func DeleteOutdatedSecrets(ctx context.Context, namespaceList v1.NamespaceList, secretName, secretRotatorName string, provisionedNamespaces []string, k8sClient client.Client) map[string]error {
	logger := log.FromContext(ctx)
	failedNamespaces := map[string]error{}

	// Loop through existing namespaces first to make sure they still have our labels
	// if not remove their provisioned secrets
	for _, namespace := range getRemovedNamespaces(namespaceList, provisionedNamespaces) {
		err := DeleteSecret(ctx, secretName, secretRotatorName, namespace, k8sClient)
		if err != nil {
			logger.Error(err, "unable to delete external secret")
			failedNamespaces[namespace] = err
		}
	}
	return failedNamespaces
}

// find namespaces that are no longer in the spec namespaces selector result
func getRemovedNamespaces(currentNSs v1.NamespaceList, provisionedNSs []string) []string {
	currentNSSet := map[string]struct{}{}
	for i := range currentNSs.Items {
		currentNSSet[currentNSs.Items[i].Name] = struct{}{}
	}
	var removedNSs []string
	for _, ns := range provisionedNSs {
		if _, ok := currentNSSet[ns]; !ok {
			removedNSs = append(removedNSs, ns)
		}
	}
	return removedNSs
}

// DeleteSecret deleting secrets which are not required
// Validating using our labels, if not then remove provisioned secrets
func DeleteSecret(ctx context.Context, esName, cesName, namespace string, k8sClient client.Client) error {
	existingES, err := GetSecret(ctx, namespace, esName, k8sClient)
	if err != nil {
		// If we can't find it then just leave
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !IsSecretOwnedBy(existingES, cesName) {
		return nil
	}

	err = k8sClient.Delete(ctx, existingES, &client.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("external secret in non matching namespace could not be deleted: %w", err)
	}
	return nil
}

// CreateOrUpdatelSecret handling secrets with new or latest token
func CreateOrUpdatelSecret(req ctrl.Request, ctx context.Context, tokenDetails *tokenType.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, namespace v1.Namespace, k8sClient client.Client, scheme *runtime.Scheme) error {
	logger := log.FromContext(ctx)

	secretObj := &v1.Secret{}
	secretObj.Name = tokenDetails.SecretName
	req.NamespacedName.Name = tokenDetails.SecretName
	req.NamespacedName.Namespace = namespace.Name
	secretObj.Namespace = namespace.Name

	err := k8sClient.Get(ctx, req.NamespacedName, secretObj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			controllerutil.SetControllerReference(secretRotator, secretObj, scheme)
		} else {
			logger.Error(err, "Failed to get SecretRotator")
			return err
		}
	}

	secretObj.Labels = secretRotator.Spec.SecretMetadata.Labels
	secretObj.Annotations = secretRotator.Spec.SecretMetadata.Annotations
	//turn token into base64
	auth := fmt.Sprintf("%s%s%s", tokenDetails.Username, ":", tokenDetails.Token)
	tokenb64 := base64.StdEncoding.EncodeToString([]byte(auth))

	secretObj.Data = map[string][]byte{
		".dockerconfigjson": []byte(fmt.Sprintf(
			`{
			"auths": {
				"%s": {
					"auth": "%s"
				}
			}
		}`, tokenDetails.ArtifactoryUrl, tokenb64)),
	}

	err = k8sClient.Update(ctx, secretObj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = k8sClient.Create(ctx, secretObj)
			if err != nil {
				return err
			}
			return nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to update SecretRotator")
		return err
	}
	return nil
}
