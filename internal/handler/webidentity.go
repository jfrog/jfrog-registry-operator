package handler

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	"artifactory-secrets-rotator/internal/operations"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/smithy-go/ptr"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetSignedRequestForWebIdentity builds a signed STS GetCallerIdentity request using IRSA (OIDC token + STS AssumeRoleWithWebIdentity).
func GetSignedRequestForWebIdentity(ctx context.Context, tokenDetails *operations.TokenDetails, serviceAccount *corev1.ServiceAccount, recorder record.EventRecorder, clientset *kubernetes.Clientset, secretRotator *jfrogv1alpha1.SecretRotator) (*http.Request, error) {
	logger := log.FromContext(ctx)
	logger.Info("Using Web Identity (IRSA) flow - assuming IAM role via STS with service account token")
	var err error
	// Check if the role arn annotation exists or not, if not, it will be set for reconciliation
	roleARN := serviceAccount.Annotations[operations.RoleARNKey]
	if roleARN == "" {
		logger.Error(fmt.Errorf("role ARN annotation is empty"), "Error getting the role ARN from the service account's annotations")
		return nil, &operations.ReconcileError{Message: "role ARN annotation is empty", RetryIn: 1 * time.Minute}
	}

	// Create token request for the target service account
	tokenRequest, err := clientset.CoreV1().ServiceAccounts(secretRotator.Spec.ServiceAccount.Namespace).CreateToken(
		ctx,
		secretRotator.Spec.ServiceAccount.Name,
		&authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{Audiences: []string{operations.AmazonAwsSts}, ExpirationSeconds: ptr.Int64(operations.ServiceAccountExpirationSeconds)}},
		metav1.CreateOptions{},
	)
	if err != nil {
		recorder.Eventf(secretRotator, "Warning", "Misconfiguration",
			fmt.Sprintf("failed to create token for user/service account %s from %s namespace, error: %s", secretRotator.Spec.ServiceAccount.Name, secretRotator.Spec.ServiceAccount.Namespace, err.Error()))
		return nil, err
	}

	// getting signed request headers for AWS STS GetCallerIdentity call and check role max session duration
	// this is needed to get the max session duration for the role ARN
	request, err := GetSignedRequestAndHandleRoleMaxSession(ctx, roleARN, tokenRequest.Status.Token, secretRotator.Spec.ServiceAccount.Name, secretRotator.Spec.ServiceAccount.Namespace, tokenDetails)
	if err != nil {
		recorder.Eventf(secretRotator, "Warning", "TokenGenerationFailure",
			fmt.Sprintf("Error getting signed AWS credentials, error was %s", err.Error()))
		return nil, err
	}

	logger.Info("Successfully created signed request for Web Identity")
	return request, nil
}
