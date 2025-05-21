package resource

import (
	"artifactory-secrets-rotator/api/v1alpha1"
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	"artifactory-secrets-rotator/internal/operations"
	"context"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controller "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IsSecretOwnedBy checks if the secret is owned by the specified SecretRotator.
func IsSecretOwnedBy(secret *v1.Secret, secretOperatorName string) bool {
	owner := metav1.GetControllerOf(secret)
	return owner != nil && owner.APIVersion == jfrogv1alpha1.GroupVersion.String() && owner.Kind == jfrogv1alpha1.SecretKind && owner.Name == secretOperatorName
}

// GetSecret retrieves the specified secret from the given namespace.
func GetSecret(ctx context.Context, namespace, secretName string, k8sClient client.Client) (*v1.Secret, error) {
	secret := &v1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, secret)
	return secret, err
}

// DeleteOutdatedSecrets removes secrets created by the controller which are no longer selected by the namespace selector.
func DeleteOutdatedSecrets(ctx context.Context, tokenDetails *operations.TokenDetails, secretRotatorName string, provisionedNamespaces []string, k8sClient client.Client) map[string]error {
	logger := log.FromContext(ctx)
	failedNamespaces := map[string]error{}

	// Identify namespaces no longer matched by the namespace selector and delete relevant secrets
	for _, namespace := range getRemovedNamespaces(tokenDetails.NamespaceList, provisionedNamespaces) {
		// Delete specified secrets from generatedSecrets
		for _, genSecret := range tokenDetails.GeneratedSecrets {
			// Skip entries with empty SecretName (not expected, as ValidateObjectSpec ensures non-empty SecretName)
			if genSecret.SecretName == "" {
				continue
			}
			err := DeleteSecret(ctx, genSecret.SecretName, secretRotatorName, namespace, genSecret.SecretType, k8sClient)
			if err != nil {
				logger.Error(err, "Unable to delete secret", "secretType", genSecret.SecretType, "secret", genSecret.SecretName, "namespace", namespace)
				failedNamespaces[namespace] = fmt.Errorf("failed to delete %s secret %s: %w", genSecret.SecretType, genSecret.SecretName, err)
			}
		}
	}

	return failedNamespaces
}

// getRemovedNamespaces finds namespaces that are no longer in the spec namespace selector result.
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

// DeleteSecret deletes a specific secret if it is owned by the SecretRotator.
func DeleteSecret(ctx context.Context, secretName, secretRotatorName, namespace, secretType string, k8sClient client.Client) error {
	existingSecret, err := GetSecret(ctx, namespace, secretName, k8sClient)
	if err != nil {
		// If the secret is not found, no action is needed
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get %s secret %s: %w", secretType, secretName, err)
	}

	// Skip deletion if the secret is not owned by the SecretRotator
	if !IsSecretOwnedBy(existingSecret, secretRotatorName) {
		return nil
	}

	err = k8sClient.Delete(ctx, existingSecret, &client.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("%s secret %s in namespace %s could not be deleted: %w", secretType, secretName, namespace, err)
	}
	return nil
}

// CreateOrUpdateSecrets creates or updates secrets in Kubernetes based on the specified secret type.
func CreateOrUpdateSecrets(req controller.Request, ctx context.Context, tokenDetails *operations.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, namespace corev1.Namespace, k8sClient client.Client, scheme *runtime.Scheme, secretName, secretType string) error {
	logger := log.FromContext(ctx)

	// Common function to set up secret metadata
	setSecretMetadata := func(secret *corev1.Secret) error {
		secret.Namespace = namespace.Name
		secret.Labels = secretRotator.Spec.SecretMetadata.Labels
		secret.Annotations = secretRotator.Spec.SecretMetadata.Annotations
		return controllerutil.SetControllerReference(secretRotator, secret, scheme)
	}

	secretObj := &v1.Secret{}
	secretObj.Name = secretName
	req.NamespacedName.Name = secretName
	req.NamespacedName.Namespace = namespace.Name
	secretObj.Namespace = namespace.Name

	// Get existing secret
	err := k8sClient.Get(ctx, req.NamespacedName, secretObj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := setSecretMetadata(secretObj); err != nil {
				return fmt.Errorf("failed to set controller reference for %s secret %s: %w", secretType, secretName, err)
			}
		} else {
			return fmt.Errorf("failed to get %s secret %s: %w", secretType, secretName, err)
		}
	}

	// Configure secret based on type
	if secretType == operations.SecretTypeDocker {
		// Create base64-encoded auth string for Docker config
		auth := fmt.Sprintf("%s:%s", tokenDetails.Username, tokenDetails.Token)
		tokenb64 := base64.StdEncoding.EncodeToString([]byte(auth))
		secretObj.Data = map[string][]byte{
			operations.DockerSecretJSON: []byte(fmt.Sprintf(
				`{
				"auths": {
					"%s": {
						"auth": "%s"
					}
				}
			}`, tokenDetails.ArtifactoryUrl, tokenb64)),
		}
		secretObj.Type = corev1.SecretTypeDockerConfigJson
	} else if secretType == operations.SecretTypeGeneric {
		secretObj.Data = map[string][]byte{
			operations.GenericSecretUser:  []byte(tokenDetails.Username),
			operations.GenericSecretToken: []byte(tokenDetails.Token),
		}
		secretObj.Type = corev1.SecretTypeOpaque
	}

	// Update or create secret
	err = k8sClient.Update(ctx, secretObj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = k8sClient.Create(ctx, secretObj)
			if err != nil {
				return fmt.Errorf("failed to create %s secret %s: %w", secretType, secretName, err)
			}
		} else {
			return fmt.Errorf("failed to update %s secret %s: %w", secretType, secretName, err)
		}
	}

	logger.Info("Successfully created/updated secret", "namespace", namespace.Name, "secret", secretName, "secretType", secretType)

	return nil
}

// HandleCerts copies certificates into the container.
func HandleCerts(ctx context.Context, namespace, secretName string, secretRotatorName string, k8sClient client.Client) error {
	logger := log.FromContext(ctx)

	// Reading secret for certificates
	secret, err := GetSecret(ctx, namespace, secretName, k8sClient)
	if err != nil {
		return err
	}

	// Create directory for certificates
	err = operations.CreateDir(v1alpha1.CustomCertificatePath + secretRotatorName)
	if err != nil {
		return err
	}

	// Based on passed certificates in secret, files will be created in container
	for key, encodedValue := range secret.Data {
		if "/"+key == v1alpha1.CaPem || "/"+key == v1alpha1.CertPem || "/"+key == v1alpha1.KeyPem || "/"+key == v1alpha1.TlsKey || "/"+key == v1alpha1.TlsCrt || "/"+key == v1alpha1.TlsCa {
			if err := operations.CreateFile(v1alpha1.CustomCertificatePath+secretRotatorName+"/"+key, string(encodedValue)); err != nil {
				return err
			}
		} else {
			logger.Error(err, "Key not supported. Supported certificate keys in secret are cert.pem, key.pem, ca.pem, tls.crt, tls.key and ca.crt")
		}
	}
	return nil
}
