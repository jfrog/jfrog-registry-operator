package operations

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// AccessResponse JFrog token response
type AccessResponse struct {
	TokenId     string `json:"token_id"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Username    string `json:"username"`
}

// TokenDetails holding resource object token details
type TokenDetails struct {
	Username              string
	Token                 string
	ArtifactoryUrl        string
	SecretName            string
	NamespaceSelector     labels.Selector
	RequeueInterval       time.Duration
	NamespaceList         v1.NamespaceList
	FailedNamespaces      map[string]error
	ProvisionedNamespaces []string
	TTLInSeconds          float64
}

// ReconcileError reconcile error struct
type ReconcileError struct {
	RetryIn time.Duration
	Message string
	Cause   error
}

func (r *ReconcileError) Error() string {
	return r.Message
}

const SecretRotatorFinalizer = "apps.jfrog.com/finalizer"
const (
	// TypeAvailableSecretRotator represents the status of the Deployment reconciliation
	TypeAvailableSecretRotator = "Available"
	// TypeDegradedSecretRotator represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	TypeDegradedSecretRotator = "Degraded"
)
