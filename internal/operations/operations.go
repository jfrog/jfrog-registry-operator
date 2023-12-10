package operations

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	"context"
	"math/rand"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateObjectSpec method validates CR spec is correct
func ValidateObjectSpec(ctx context.Context, tokenDetails *TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, k8sClient client.Client) error {
	var err error
	logger := log.FromContext(ctx)

	tokenDetails.SecretName = secretRotator.Spec.SecretName
	if tokenDetails.SecretName == "" {
		return &ReconcileError{Message: "Missing getting the secret name in operator object configuration, please configure the secret name on your operator object, the current reconciliation cycle will end here"}
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

	//check if artifactory host contains http or https
	// if the operator was configured with full uri, remove http or https
	if tokenDetails.ArtifactoryUrl[:8] == "https://" {
		tokenDetails.ArtifactoryUrl = tokenDetails.ArtifactoryUrl[8:]
	} else if tokenDetails.ArtifactoryUrl[:7] == "http://" {
		tokenDetails.ArtifactoryUrl = tokenDetails.ArtifactoryUrl[7:]
	}

	logger.Info("Artifactory host", "host", tokenDetails.ArtifactoryUrl)
	return nil
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
func ListSecretRotatorObjects(cli client.Client) *jfrogv1alpha1.SecretRotatorList {
	secretRotators := &jfrogv1alpha1.SecretRotatorList{}
	err := cli.List(context.Background(), secretRotators, &client.ListOptions{})
	if err != nil {
		return &jfrogv1alpha1.SecretRotatorList{}
	}
	return secretRotators
}

func HandlingNamespaceEvents(cli client.Client, log logr.Logger, object *jfrogv1alpha1.SecretRotator) bool {
	if object.Annotations == nil {
		object.Annotations = make(map[string]string)
	}
	object.Annotations["uid"] = GetRandomString()
	if err := cli.Update(context.Background(), object, &client.UpdateOptions{}); err != nil {
		return false
	}
	return true
}
