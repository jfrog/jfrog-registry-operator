package operations

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"artifactory-secrets-rotator/api/v1alpha1"
)

// Secret Info
const (
	// Secret types
	SecretTypeDocker  = "docker"
	SecretTypeGeneric = "generic"

	// Generic secret keys
	GenericSecretUser  = "user"
	GenericSecretToken = "token"

	// Docker secret key
	DockerSecretJSON = ".dockerconfigjson"
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
	Username       string
	Token          string
	ArtifactoryUrl string
	// SecretName is optional in 2.x and will be depreciate in next upcoming releases
	SecretName                string
	GeneratedSecrets          []v1alpha1.GeneratedSecret
	NamespaceSelector         labels.Selector
	RequeueInterval           time.Duration
	NamespaceList             v1.NamespaceList
	FailedNamespaces          map[string]error
	ProvisionedNamespaces     []string
	TTLInSeconds              float64
	SecretManagedByNamespaces map[string][]string
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
