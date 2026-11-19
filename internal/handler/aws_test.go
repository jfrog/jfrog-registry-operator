package handler

import (
	controllers2 "artifactory-secrets-rotator/internal/sign"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignedCallerIdentityRequest(t *testing.T) {
	for _, tt := range []struct {
		region string
		host   string
	}{
		{region: "eu-west-1", host: "sts.eu-west-1.amazonaws.com"},
		{region: "cn-northwest-1", host: "sts.cn-northwest-1.amazonaws.com.cn"},
	} {
		t.Run(tt.region, func(t *testing.T) {
			req, err := signedCallerIdentityRequest(context.Background(), controllers2.AwsCredentials{
				AccessKey:    "test-access-key",
				SecretKey:    "test-secret-key",
				SessionToken: "test-session-token",
				RegionName:   tt.region,
			})
			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, req.Method)
			assert.Equal(t, "https", req.URL.Scheme)
			assert.Equal(t, tt.host, req.URL.Host)
			assert.Equal(t, tt.host, req.Header.Get("Host"))
			assert.Equal(t, "GetCallerIdentity", req.URL.Query().Get("Action"))
			assert.Equal(t, "2011-06-15", req.URL.Query().Get("Version"))
			assert.Contains(t, req.Header.Get("Authorization"), "AWS4-ECDSA-P256-SHA256 ")
			assert.Contains(t, req.Header.Get("Authorization"), "SignedHeaders=host;")
			assert.Equal(t, "test-session-token", req.Header.Get("X-Amz-Security-Token"))
		})
	}
}
