package handler

import (
	controllers2 "artifactory-secrets-rotator/internal/sign"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const ERROR_EXTRACT_MAX_SESSION = "could not extract max session from roleARN"

func GetMaxSession(roleArn string) (*int32, error) {

	logger := log.FromContext(context.TODO())

	// extracting role name from role ARN
	substrings := strings.Split(roleArn, "/")
	if len(substrings) != 2 {
		err := errors.New("role arn is not valid")
		logger.Error(err, ERROR_EXTRACT_MAX_SESSION, "roleARN", roleArn)
		return nil, err
	}
	roleName := substrings[1]
	//logger.Info("roleName", "roleName", roleName)
	// initiating aws iam client
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		logger.Error(err, ERROR_EXTRACT_MAX_SESSION+"got error loading default aws config")
		return nil, err
	}
	iamClient := iam.NewFromConfig(cfg)
	// getting role
	result, err := iamClient.GetRole(context.TODO(),
		&iam.GetRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		logger.Error(err, ERROR_EXTRACT_MAX_SESSION, "Couldn't get role", roleName)
		return nil, err
	} else if result.Role == nil {
		logger.Error(err, ERROR_EXTRACT_MAX_SESSION, "result role is null", roleName)
		return nil, err
	}
	roleMaxSession := result.Role.MaxSessionDuration

	return roleMaxSession, nil
}

func GetSignedRequest(roleArn string, webIdentityTokenFile string) (*http.Request, error) {
	logger := log.FromContext(context.TODO())
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		logger.Error(err, "Got error loading default aws config")
		return nil, err
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
	credenetials, err := appCreds.Retrieve(context.TODO())
	if err != nil {
		logger.Error(err, "Got error on appCreds.Retrieve")
		return nil, err
	}
	logger.Info("getting temporary role credentials", "Region", cfg.Region)

	creds := &controllers2.AwsCredentials{
		AccessKey:    credenetials.AccessKeyID,
		SecretKey:    credenetials.SecretAccessKey,
		RegionName:   cfg.Region, //"*",
		SessionToken: credenetials.SessionToken,
	}
	//using temporary role credentiuals for producing signed getCallerIdentity request headers
	awsRequest, err := controllers2.SignV4a("GET",
		"https://sts.amazonaws.com?Action=GetCallerIdentity&Version=2011-06-15", "sts", *creds)
	// @toDo remove once debug is done
	for k, v := range awsRequest.Header {
		logger.Info("getSignedRequest Headers", "header", k, "value", v)
	}

	return awsRequest, err
}

func GetCredentials(roleArn string, webIdentityTokenFile string) (aws.Credentials, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	client := sts.NewFromConfig(cfg)

	appCreds := aws.NewCredentialsCache(stscreds.NewWebIdentityRoleProvider(
		client,
		roleArn,
		stscreds.IdentityTokenFile(webIdentityTokenFile),
		func(o *stscreds.WebIdentityRoleOptions) {
			o.RoleSessionName = "sessionName"
		}))
	credenetials, err := appCreds.Retrieve(context.TODO())
	return credenetials, err
}

func GetAwsCrtV4aSignerHeaders(ctx context.Context, url string, method string, creds aws.Credentials) (http.Header, error) {
	logger := log.FromContext(ctx)

	//create http.request var for URL
	req, err := http.NewRequest(method, url, nil)
	//get awsV4aSigner headers
	awsV4aSigner := v4.NewSigner()
	//global_region := "aws-global"
	global_region := ""
	//err := signer.SignHTTP(ctx, req, reqBodySHA256, apiSigningName, signingRegion, time.Now())
	err = awsV4aSigner.SignHTTP(ctx, creds, req, "", "sts", global_region, time.Now())

	//print request headers
	for k, v := range req.Header {
		logger.Info("req.Header", "header", k, "value", v)
	}
	return req.Header, err
}
