package handler

import (
	"artifactory-secrets-rotator/internal/operations"
	controllers2 "artifactory-secrets-rotator/internal/sign"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ResolveIAMRoleARNFromPodIdentityCredentials returns the IAM role ARN for the current Pod Identity session.
// EKS Pod Identity does not inject AWS_ROLE_ARN; the role is discovered via STS GetCallerIdentity using the
// temporary keys from the credentials endpoint (Arn is an assumed-role ARN, converted to arn:aws:iam::...:role/...).
func ResolveIAMRoleARNFromPodIdentityCredentials(ctx context.Context, region string, credResp *operations.CredentialsResponse, tokenDetails *operations.TokenDetails) (*int32, error) {
	logger := log.FromContext(ctx)

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(credResp.AccessKeyId, credResp.SecretAccessKey, credResp.Token)),
	)
	if err != nil {
		return nil, err
	}
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}
	stsArn := aws.ToString(out.Arn)
	accountID := aws.ToString(out.Account)
	if stsArn == "" || accountID == "" {
		return nil, fmt.Errorf("STS GetCallerIdentity returned empty Arn or Account")
	}
	roleARN, err := assumedRoleSTSArnToIAMRoleARN(stsArn, accountID)
	if err != nil {
		return nil, err
	}

	// IAM role ARN is not in Pod Identity env vars; resolve via STS using these temporary credentials.
	podIdentityCreds := aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
		credResp.AccessKeyId, credResp.SecretAccessKey, credResp.Token))

	maxDur, err := GetMaxSessionWithCredentialCache(ctx, roleARN, podIdentityCreds, tokenDetails)
	if err != nil {
		return nil, err
	}
	logger.Info("Successfully retrieved IAM role ARN from Pod Identity credentials", "roleARN", roleARN, "maxSessionDuration in seconds", *maxDur)
	return maxDur, nil
}

func assumedRoleSTSArnToIAMRoleARN(stsArn, accountID string) (string, error) {
	const marker = ":assumed-role/"
	i := strings.Index(stsArn, marker)
	if i < 0 {
		return "", fmt.Errorf("caller identity ARN is not an assumed role: %s", stsArn)
	}
	tail := stsArn[i+len(marker):]
	lastSlash := strings.LastIndex(tail, "/")
	if lastSlash < 0 {
		return "", fmt.Errorf("invalid assumed-role ARN: %s", stsArn)
	}
	rolePathAndName := tail[:lastSlash]
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, rolePathAndName), nil
}

// GetSignedRequestForPodIdentity signs the GetCallerIdentity request that JFrog expects
func GetSignedRequestForPodIdentity(ctx context.Context, tokenDetails *operations.TokenDetails) (*http.Request, error) {
	logger := log.FromContext(ctx)
	logger.Info("Using the Pod Identity flow: fetching credentials from the credentials endpoint")

	// Get Pod Identity credentials from credential endpoint
	credUri := os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI")
	if credUri == "" {
		return nil, &operations.ReconcileError{Message: "No AWS credentials available for Pod Identity (AWS_CONTAINER_CREDENTIALS_FULL_URI not set)", RetryIn: 1 * time.Minute}
	}

	logger.Info("Sending a request to the Pod Identity credentials endpoint", "uri", credUri)

	// Make direct HTTP call to Pod Identity credential endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", credUri, nil)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Failed to create Pod Identity request", Cause: err, RetryIn: 1 * time.Minute}
	}

	// Add authorization token from file (Pod Identity requires this)
	authTokenFile := os.Getenv("AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE")
	if authTokenFile == "" {
		return nil, &operations.ReconcileError{Message: "Pod Identity token file not found (AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE not set)", RetryIn: 1 * time.Minute}
	}

	tokenBytes, err := os.ReadFile(authTokenFile)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Failed to read Pod Identity token file", Cause: err, RetryIn: 1 * time.Minute}
	}
	req.Header.Set("Authorization", string(tokenBytes))

	resp, err := client.Do(req)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Failed to get Pod Identity credentials", Cause: err, RetryIn: 1 * time.Minute}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &operations.ReconcileError{
			Message: fmt.Sprintf("%s%s%s%d%s", "Error getting in Pod Identity credentials endpoint ", credUri, " returned ", resp.StatusCode, " response"),
			RetryIn: 1 * time.Minute,
		}
	}

	var credResp operations.CredentialsResponse
	if err := json.NewDecoder(resp.Body).Decode(&credResp); err != nil {
		return nil, &operations.ReconcileError{Message: "Failed to parse Pod Identity credentials", Cause: err, RetryIn: 1 * time.Minute}
	}

	logger.Info("Successfully retrieved Pod Identity credentials", "accessKeyId", credResp.AccessKeyId[:8]+"...")

	// Get the region from the SecretRotator spec.awsRegion, else use operations default
	region := tokenDetails.IAMRoleAwsRegion
	if region == "" {
		region = operations.AwsRegion
	}

	// Create AWS credentials struct for signing (region from SecretRotator spec.awsRegion, else operations default)
	creds := &controllers2.AwsCredentials{
		AccessKey:    credResp.AccessKeyId,
		SecretKey:    credResp.SecretAccessKey,
		RegionName:   region,
		SessionToken: credResp.Token,
	}

	// Sign the GetCallerIdentity request that JFrog expects
	req, err = controllers2.SignV4a("GET",
		"https://sts.amazonaws.com?Action=GetCallerIdentity&Version=2011-06-15", "sts", *creds)
	if err != nil {
		return nil, &operations.ReconcileError{Message: "Failed to sign the STS GetCallerIdentity request using Pod Identity credentials", Cause: err, RetryIn: 1 * time.Minute}
	}

	logger.Info("Successfully created a signed GetCallerIdentity request for Pod Identity")

	logger.Info("Resolving IAM role ARN from Pod Identity credentials and getting max session duration")
	tokenDetails.RoleMaxSessionDuration, err = ResolveIAMRoleARNFromPodIdentityCredentials(ctx, region, &credResp, tokenDetails)
	if err != nil {
		tokenDetails.RoleMaxSessionDuration = aws.Int32(operations.RoleMaxSessionDuration)
		logger.Info("Using default Artifactory token expiration for Pod Identity (role ARN / max session lookup failed)",
			"reason", err.Error(),
			"durationSeconds", *tokenDetails.RoleMaxSessionDuration)
	}

	return req, nil
}
