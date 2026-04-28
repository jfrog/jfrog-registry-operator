package handler

import (
	"artifactory-secrets-rotator/internal/operations"
	controllers2 "artifactory-secrets-rotator/internal/sign"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const ErrorExtractMaxSession = "could not extract max session from roleARN"

type tokenProvider struct {
	token string
}

// GetIdentityToken implements the stscreds.IdentityTokenRetriever interface
func (tp *tokenProvider) GetIdentityToken() ([]byte, error) {
	return []byte(tp.token), nil
}

// GetMaxSession retrieves role session duration
func GetMaxSession(ctx context.Context, roleArn string, appCreds *aws.CredentialsCache, resourceSAName, resourceSANamespace string, tokenDetails *operations.TokenDetails) (*int32, error) {
	logger := log.FromContext(ctx)
	// extracting role name from role ARN
	substrings := strings.Split(roleArn, "/")
	if len(substrings) != 2 {
		err := errors.New("role arn is not valid")
		return nil, err
	}
	roleName := substrings[1]
	var cfg aws.Config
	var err error

	if resourceSAName == tokenDetails.DefaultServiceAccountName && resourceSANamespace == tokenDetails.DefaultServiceAccountNamespace {
		logger.Info("Operator's service account is used", "role", roleName, "service account", "type - single user")
		cfg, err = awsconfig.LoadDefaultConfig(ctx)
	} else {
		logger.Info("External service account is used", "role", roleName, "service account", "type - multi user", "aws-config", "default aws region and ec2 imds region")
		cfg, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithCredentialsProvider(appCreds), awsconfig.WithRegion(tokenDetails.IAMRoleAwsRegion), awsconfig.WithEC2IMDSRegion())
	}
	if err != nil {
		return nil, err
	}

	// initiating aws iam client
	iamClient := iam.NewFromConfig(cfg)

	// getting role
	result, err := iamClient.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		return nil, err
	} else if result.Role == nil {
		return nil, errors.New("could not extract max session from roleARN")
	}
	return result.Role.MaxSessionDuration, nil
}

// GetMaxSessionWithCredentialCache reads IAM MaxSessionDuration for roleArn using a fixed credential source
// (e.g. Pod Identity keys from the agent). Used when credentials are not loaded via the default SDK chain.
func GetMaxSessionWithCredentialCache(ctx context.Context, roleArn string, credCache *aws.CredentialsCache, tokenDetails *operations.TokenDetails) (*int32, error) {
	logger := log.FromContext(ctx)
	substrings := strings.Split(roleArn, "/")
	if len(substrings) != 2 {
		return nil, errors.New("role arn is not valid")
	}
	roleName := substrings[1]
	region := tokenDetails.IAMRoleAwsRegion
	if region == "" {
		region = operations.AwsRegion
	}
	logger.Info("Reading IAM role max session duration", "role", roleName, "auth", "podIdentity")
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credCache),
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}
	iamClient := iam.NewFromConfig(cfg)
	result, err := iamClient.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		return nil, err
	}
	if result.Role == nil {
		return nil, errors.New("could not extract max session from roleARN")
	}
	return result.Role.MaxSessionDuration, nil
}

// GetSignedRequestAndHandleRoleMaxSession signs aws credentials to be used for GetCallerIdentity request
func GetSignedRequestAndHandleRoleMaxSession(ctx context.Context, roleArn string, webIdentityToken string, resourceSAName, resourceSANamespace string, tokenDetails *operations.TokenDetails) (*http.Request, error) {
	logger := log.FromContext(ctx)
	logger.Info("Signing request", "role", roleArn)
	var cfg aws.Config
	var err error

	// loading default aws config
	// if the operator's service account is used, we will use the default aws config
	// if the external service account is used, we will use the default iam config
	// and the ec2 imds region
	if resourceSAName == tokenDetails.DefaultServiceAccountName && resourceSANamespace == tokenDetails.DefaultServiceAccountNamespace {
		cfg, err = awsconfig.LoadDefaultConfig(ctx)
	} else {
		cfg, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(tokenDetails.IAMRoleAwsRegion), awsconfig.WithEC2IMDSRegion())
	}
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Got error loading default aws config", Cause: err, RetryIn: 1 * time.Minute}
	}

	// creating sts client
	client := sts.NewFromConfig(cfg)

	//prepare assumed role session name
	sessionTime := time.Now().UTC().UnixMilli()
	sessionName := fmt.Sprintf("%s%d", "artifactorySecretRotation", sessionTime)
	appCreds := aws.NewCredentialsCache(stscreds.NewWebIdentityRoleProvider(
		client,
		roleArn,
		&tokenProvider{token: webIdentityToken},
		func(o *stscreds.WebIdentityRoleOptions) {
			o.RoleSessionName = sessionName
		}))

	// creating credentials cache, this is needed to get the credentials for the role ARN
	credentials, err := appCreds.Retrieve(ctx)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Got error on appCreds.Retrieve", Cause: err, RetryIn: 1 * time.Minute}
	}

	// creating aws credentials struct, this is needed to sign the request. Needed to sign the request
	creds := &controllers2.AwsCredentials{
		AccessKey:    credentials.AccessKeyID,
		SecretKey:    credentials.SecretAccessKey,
		RegionName:   cfg.Region, // Required for STS
		SessionToken: credentials.SessionToken,
	}

	//using temporary role credentials for producing signed getCallerIdentity request headers
	req, err := controllers2.SignV4a("GET",
		"https://sts.amazonaws.com?Action=GetCallerIdentity&Version=2011-06-15", "sts", *creds)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Got error signing sts request", Cause: err, RetryIn: 1 * time.Minute}
	}

	//getting max aws role session time to be used as artiactory token expiration time
	tokenDetails.RoleMaxSessionDuration, err = GetMaxSession(ctx, roleArn, appCreds, resourceSAName, resourceSANamespace, tokenDetails)
	if err != nil {
		tokenDetails.RoleMaxSessionDuration = aws.Int32(operations.RoleMaxSessionDuration) // 3 hours
		logger.Info("Using default Artifactory token expiration (IAM GetRole / max session lookup failed)",
			"reason", err.Error(),
			"durationSeconds", *tokenDetails.RoleMaxSessionDuration)
	}

	return req, nil
}
