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

// GetMaxSession retrieves role session duration
func GetMaxSession(ctx context.Context, roleArn string) (*int32, error) {
	// extracting role name from role ARN
	substrings := strings.Split(roleArn, "/")
	if len(substrings) != 2 {
		err := errors.New("role arn is not valid")
		return nil, err
	}
	roleName := substrings[1]
	// initiating aws iam client
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	iamClient := iam.NewFromConfig(cfg)
	// getting role
	result, err := iamClient.GetRole(ctx,
		&iam.GetRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		return nil, err
	} else if result.Role == nil {
		return nil, errors.New("could not extract max session from roleARN")
	}
	roleMaxSession := result.Role.MaxSessionDuration
	return roleMaxSession, nil
}

// GetSignedRequest signs aws credentials to be used for GetCallerIdentity request
func GetSignedRequest(ctx context.Context, roleArn string, webIdentityTokenFile string) (*http.Request, error) {
	logger := log.FromContext(ctx)
	logger.Info("Signing request", "role", roleArn)
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Got error loading default aws config", Cause: err, RetryIn: 1 * time.Minute}
	}
	client := sts.NewFromConfig(cfg)
	//prepare assumed role session name
	sessionTime := time.Now().UTC().UnixMilli()
	sessionName := fmt.Sprintf("%s%d", "artifactorySecretRotation", sessionTime)

	appCreds := aws.NewCredentialsCache(stscreds.NewWebIdentityRoleProvider(
		client,
		roleArn,
		stscreds.IdentityTokenFile(webIdentityTokenFile),
		func(o *stscreds.WebIdentityRoleOptions) {
			o.RoleSessionName = sessionName
		}))
	credentials, err := appCreds.Retrieve(ctx)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Got error on appCreds.Retrieve", Cause: err, RetryIn: 1 * time.Minute}
	}
	creds := &controllers2.AwsCredentials{
		AccessKey:    credentials.AccessKeyID,
		SecretKey:    credentials.SecretAccessKey,
		RegionName:   cfg.Region, //"*",
		SessionToken: credentials.SessionToken,
	}
	//using temporary role credentials for producing signed getCallerIdentity request headers
	req, err := controllers2.SignV4a("GET",
		"https://sts.amazonaws.com?Action=GetCallerIdentity&Version=2011-06-15", "sts", *creds)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Got error signing sts request", Cause: err, RetryIn: 1 * time.Minute}
	}
	return req, nil
}
