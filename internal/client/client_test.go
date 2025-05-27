package client

import (
	"testing"
)

func TestGetK8sClient_Success(t *testing.T) {

	clientSet, err := GetK8sClient()
	if err != nil {
		t.Fatalf("GetK8sClient returned an error: %v", err)
	}

	if clientSet == nil {
		t.Fatalf("GetK8sClient returned a nil clientSet")
	}

}
