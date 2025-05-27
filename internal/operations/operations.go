package operations

import (
	k8sClientSet "artifactory-secrets-rotator/internal/client"
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"artifactory-secrets-rotator/api/v1alpha1"
)

// ValidateObjectSpec method validates CR spec is correct
func ValidateObjectSpec(ctx context.Context, tokenDetails *TokenDetails, secretRotator *v1alpha1.SecretRotator, k8sClient client.Client) error {
	var err error
	logger := log.FromContext(ctx)

	// Initialize GeneratedSecrets
	tokenDetails.GeneratedSecrets = make([]v1alpha1.GeneratedSecret, 0)
	tokenDetails.GeneratedSecrets = append(tokenDetails.GeneratedSecrets, secretRotator.Spec.GeneratedSecrets...)

	// Set SecretName and append to GeneratedSecrets if provided
	if secretRotator.Spec.SecretName != "" {
		tokenDetails.GeneratedSecrets = append(tokenDetails.GeneratedSecrets, v1alpha1.GeneratedSecret{
			SecretName: secretRotator.Spec.SecretName,
			SecretType: SecretTypeDocker,
		})
		logger.Info("Using existing secret name spec.secretName, This will be deprecated soon. If new secret name added in new config this will be appended", "secretName", secretRotator.Spec.SecretName)
	}

	// Validate that at least one secret is defined
	if len(tokenDetails.GeneratedSecrets) == 0 {
		return &ReconcileError{
			Message: "No secrets defined in spec.generatedSecrets and spec.secretName. Please configure secret details. The current reconciliation cycle will end here.",
		}
	}

	// Validate secret types, ensure SecretName is provided, and check for duplicates
	seenNames := map[string]string{}
	for _, gSecret := range tokenDetails.GeneratedSecrets {
		// Check for duplicate SecretName
		if _, exists := seenNames[gSecret.SecretName]; exists {
			return &ReconcileError{
				Message: fmt.Sprintf("Duplicate SecretName '%s' in generatedSecrets. Each secret must have a unique name. The current reconciliation cycle will end here.", gSecret.SecretName),
			}
		}
		seenNames[gSecret.SecretName] = gSecret.SecretType

		// Validate the SecretName and SecretType
		if gSecret.SecretName == "" {
			return &ReconcileError{
				Message: fmt.Sprintf("Empty SecretName in generatedSecrets for %s secret. Each secret must have a valid name. The current reconciliation cycle will end here.", gSecret.SecretType),
			}
		}

		if gSecret.SecretType != SecretTypeDocker && gSecret.SecretType != SecretTypeGeneric {
			return &ReconcileError{
				Message: fmt.Sprintf("Invalid SecretType '%s' in generatedSecrets. Must be 'docker' or 'generic'. The current reconciliation cycle will end here.", gSecret.SecretType),
			}
		}

	}

	// Log GeneratedSecrets for debugging
	for i, gSecret := range tokenDetails.GeneratedSecrets {
		logger.Info("Generated Secret entry", "index", i, "secretName", gSecret.SecretName, "secretType", gSecret.SecretType)
	}

	tokenDetails.NamespaceSelector, err = metav1.LabelSelectorAsSelector(&secretRotator.Spec.NamespaceSelector)
	if err != nil {
		return &ReconcileError{Message: "Error reading namespace labels selector from operator object configuration, no secrets will be created or updated, the current reconciliation cycle will end here", Cause: err}
	}

	tokenDetails.NamespaceList = v1.NamespaceList{}
	err = k8sClient.List(ctx, &tokenDetails.NamespaceList, &client.ListOptions{LabelSelector: tokenDetails.NamespaceSelector})
	if err != nil {
		return &ReconcileError{Message: "No namespaces match the configured namespace selector, the current reconciliation cycle will end here", Cause: err}
	}

	tokenDetails.ArtifactoryUrl = secretRotator.Spec.ArtifactoryUrl
	if tokenDetails.ArtifactoryUrl == "" {
		return &ReconcileError{Message: "Missing ArtifactoryUrl in operator object configuration, no secrets will be created or updated, the current reconciliation cycle will end here"}
	}

	tokenDetails.IAMRoleAwsRegion = secretRotator.Spec.AwsRegion
	if tokenDetails.IAMRoleAwsRegion == "" {
		tokenDetails.IAMRoleAwsRegion = AwsRegion
	}

	// Check if artifactory host contains http or https
	// If the operator was configured with full URI, remove http or https
	if len(tokenDetails.ArtifactoryUrl) > 8 && tokenDetails.ArtifactoryUrl[:8] == "https://" {
		tokenDetails.ArtifactoryUrl = tokenDetails.ArtifactoryUrl[8:]
	} else if len(tokenDetails.ArtifactoryUrl) > 7 && tokenDetails.ArtifactoryUrl[:7] == "http://" {
		tokenDetails.ArtifactoryUrl = tokenDetails.ArtifactoryUrl[7:]
	}

	// Get the service account details. If not provided, the operator's service account will be used by default.
	serviceAccount, err := GetServiceAccount(ctx, k8sClient, tokenDetails)
	if err != nil {
		return &ReconcileError{Message: "Error reading operator's resource, the current reconciliation cycle will end here", Cause: err}
	}

	// Check if the service account name and namespace are provided in the custom resource, if not, updating the custom resource with the operator's service account name and namespace
	if secretRotator.Spec.ServiceAccount.Name == "" || secretRotator.Spec.ServiceAccount.Namespace == "" {
		logger.Info("Service account name and namespace not provided in the custom resource, using the operator's service account")
		roleARN := serviceAccount.Annotations[AwsRoleARNKey]
		if roleARN == "" {
			return &ReconcileError{Message: "No service account details were provided in resource, and the operator's service account does not have the required ARN annotation. Please either update the operator's service account with the appropriate annotation or specify your service account by providing serviceAccount.name and serviceAccount.namespace in the custom resource."}
		}
		logger.Info("Using the operator's default service account", "roleARN", roleARN)
	}

	logger.Info("Artifactory host", "host", tokenDetails.ArtifactoryUrl)
	return nil
}

// GetServiceAccount is used to get the service account and pod details, it will return the service account object
// and the pod object, and a boolean indicating if the service account is annotated with role ARN
// If the service account is not annotated with role ARN, it will return an error
func GetServiceAccount(ctx context.Context, k8sClient client.Client, tokenDetails *TokenDetails) (*v1.ServiceAccount, error) {

	// Get current pod name and namespace
	podName, namespace := os.Getenv("POD_NAME"), os.Getenv("POD_NAMESPACE")
	pod, serviceAccount := &v1.Pod{}, &v1.ServiceAccount{}

	// Get the clientset
	clientset, err := k8sClientSet.GetK8sClient()
	if err != nil {
		return nil, err
	}

	// Get pod details to get the service account name
	pod, err = clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Get Service Account details
	serviceAccount, err = clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, pod.Spec.ServiceAccountName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Update the token details with the service account name and namespace
	tokenDetails.DefaultServiceAccountName = pod.Spec.ServiceAccountName
	tokenDetails.DefaultServiceAccountNamespace = namespace

	return serviceAccount, nil
}

// IsExist checks the labels from namespaces and secret rotator objects
func IsExist(namespaceLabels, objectLabels map[string]string) bool {
	for val, _ := range objectLabels {
		if namespaceLabels[val] != objectLabels[val] {
			return false
		}
	}
	return true
}

// GetRandomString generates random string with size 10
func GetRandomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 10)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// ListSecretRotatorObjects return list of secret rotator objects
func ListSecretRotatorObjects(cli client.Client) *v1alpha1.SecretRotatorList {
	secretRotators := &v1alpha1.SecretRotatorList{}
	err := cli.List(context.Background(), secretRotators, &client.ListOptions{})
	if err != nil {
		return &v1alpha1.SecretRotatorList{}
	}
	return secretRotators
}

// HandlingNamespaceEvents updates annotations for namespace events
func HandlingNamespaceEvents(cli client.Client, log logr.Logger, object *v1alpha1.SecretRotator) bool {
	if object.Annotations == nil {
		object.Annotations = make(map[string]string)
	}
	object.Annotations["uid"] = GetRandomString()
	if err := cli.Update(context.Background(), object, &client.UpdateOptions{}); err != nil {
		return false
	}
	return true
}

// FileExists checks if a file exists and is not a directory
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// CreateFile creates a file with the given content
func CreateFile(filePath, content string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}

// CreateDir function to execute the program and create directory
func CreateDir(directoryname string) error {
	// Check if the directory exists
	if _, err := os.Stat(directoryname); os.IsExist(err) {
		return nil
	}
	err := os.MkdirAll(directoryname, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

// DeleteOutdatedGeneratedSecrets deletes outdated generated secrets
// from the cluster if they are not present in the current configuration
func DeleteOutdatedGeneratedSecrets(ctx context.Context, tokenDetails *TokenDetails, secretRotator *v1alpha1.SecretRotator, k8sClient client.Client) error {
	logger := log.FromContext(ctx)

	changesSecrets, changed := findSecretDifferences(tokenDetails.SecretManagedByNamespaces, secretRotator.Status.SecretManagedByNamespaces)
	if changed {
		for namespace, secretNames := range changesSecrets {
			for _, secretName := range secretNames {
				logger.Info("[Outdated secrets found] Deleting secret in namespace", "Name", secretName, "Namespace", namespace)
				err := k8sClient.Delete(ctx, &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}}, &client.DeleteOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						logger.Info("Secret not found in namespace, skipping deletion.", "Name", secretName, "Namespace", namespace)
						continue // Skip if the secret is already deleted
					}
					return fmt.Errorf("error deleting secret %s in namespace %s: %w", secretName, namespace, err)
				}
				logger.Info("Successfully deleted outdated secret in namespace", "Name", secretName, "Namespace", namespace)
			}
		}
	}
	return nil
}

// findSecretDifferences compares the new state with the old state and returns the differences
// and a boolean indicating if there are any differences
func findSecretDifferences(newState map[string][]string, oldState map[string][]string) (map[string][]string, bool) {
	differences := make(map[string][]string)

	for key, oldValues := range oldState {
		newValues, ok := newState[key]
		if ok {
			diff := findStringSliceDifference(oldValues, newValues)
			if len(diff) > 0 {
				differences[key] = diff
			}
		}
	}
	if len(differences) == 0 {
		return nil, false
	}
	return differences, true
}

// Helper function to find elements in 'a' that are not in 'b'
func findStringSliceDifference(a []string, b []string) []string {
	diff := []string{}
	bMap := make(map[string]bool)
	for _, item := range b {
		bMap[item] = true
	}
	for _, item := range a {
		if _, found := bMap[item]; !found {
			diff = append(diff, item)
		}
	}
	return diff
}
