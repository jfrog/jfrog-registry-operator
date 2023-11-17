package operations

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	"context"
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
