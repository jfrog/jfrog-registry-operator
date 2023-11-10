package operations

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	types "artifactory-secrets-rotator/internal/types"

	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ValidateObjectSpec method validates CR spec is correct
func ValidateObjectSpec(ctx context.Context, tokenDetails *types.TokenDetails, secretRotator *jfrogv1alpha1.SecretRotator, k8sClient client.Client) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	var err error

	refreshInt := tokenDetails.RequeueInterval
	if secretRotator.Spec.RefreshInterval != nil {
		refreshInt = secretRotator.Spec.RefreshInterval.Duration
	}

	logger.Info("Setting reconciliation interval", "interval", refreshInt)
	tokenDetails.SecretName = secretRotator.Spec.SecretName
	if tokenDetails.SecretName == "" {
		logger.Error(err, "Missing getting the secret name in operator object configuration, please configure the secret name on your operator object, the current reconciliation cycle will end here")
		return RetryFailedReconciliation()
	}

	tokenDetails.NamespaceSelector, err = metav1.LabelSelectorAsSelector(&secretRotator.Spec.NamespaceSelector)
	if err != nil {
		logger.Error(err, "Error reading namespace labels selector from operator object configuration, no secrets will be created or updated, the current reconciliation cycle will end here")
		return RetryFailedReconciliation()
	}

	tokenDetails.NamespaceList = v1.NamespaceList{}
	err = k8sClient.List(ctx, &tokenDetails.NamespaceList, &client.ListOptions{LabelSelector: tokenDetails.NamespaceSelector})
	if err != nil {
		logger.Error(err, "No namespaces match the configured namespace selector, the current reconciliation cycle will end here")
		return RetryFailedReconciliation()
	}

	tokenDetails.ArtifactoryUrl = secretRotator.Spec.ArtifactoryUrl
	if tokenDetails.ArtifactoryUrl == "" {
		logger.Error(err, "Missing ArtifactoryUrl in operator object configuration, no secrets will be created or updated, the current reconciliation cycle will end here")
		return RetryFailedReconciliation()
	}

	//check if artifactory host contains http or https
	// if the operator was configured with full uri, remove http or https
	if tokenDetails.ArtifactoryUrl[:8] == "https://" {
		tokenDetails.ArtifactoryUrl = tokenDetails.ArtifactoryUrl[8:]
	} else if tokenDetails.ArtifactoryUrl[:7] == "http://" {
		tokenDetails.ArtifactoryUrl = tokenDetails.ArtifactoryUrl[7:]
	}

	logger.Info("Artifactory host", "host", tokenDetails.ArtifactoryUrl)
	logger.Info("Updating of Secret", "secretName", tokenDetails.SecretName, "namespaces", len(tokenDetails.NamespaceList.Items), " reconciliation interval ", refreshInt.String())
	return ctrl.Result{}, nil
}
