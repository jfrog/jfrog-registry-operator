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
	// SecretName is optional in 2.x and will be depreciate in next upcoming releases
	SecretName                     string
	GeneratedSecrets               []v1alpha1.GeneratedSecret
	NamespaceList                  v1.NamespaceList
	FailedNamespaces               map[string]error
	ProvisionedNamespaces          []string
	TTLInSeconds                   float64
	SecretManagedByNamespaces      map[string][]string
	Username                       string
	Token                          string
	ArtifactoryUrl                 string
	NamespaceSelector              labels.Selector
	RequeueInterval                time.Duration
	DefaultServiceAccountName      string
	DefaultServiceAccountNamespace string
	RoleMaxSessionDuration         *int32
	IAMRoleAwsRegion               string
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

const (
	// AwsRegion value of AwsRoleARNKey
	AwsRegion = "us-west-2"
	// Default value of AwsRoleARNKey
	AwsRoleARNKey = "eks.amazonaws.com/role-arn"
	// Default value of RoleARNKey
	RoleARNKey = "eks.amazonaws.com/role-arn"
	// Default value of AmazonAwsSts
	AmazonAwsSts = "sts.amazonaws.com"
)

const (
	// RoleMaxSessionDuration is the default max session duration for AWS roles
	RoleMaxSessionDuration = 10800

	// ServiceAccountExpirationSeconds is the default expiration time for service account tokens
	ServiceAccountExpirationSeconds = 3600
)
