package operations

import (
	jfrogv1alpha1 "artifactory-secrets-rotator/api/v1alpha1"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Clientset struct {
	KubeClient kubernetes.Interface
}

func TestGetServiceAccount_Success(t *testing.T) {
	ctx := context.Background()
	const podNamespace = "test-pod-ns"
	const podName = "test-pod-name"
	const serviceAccountName = "test-sa"

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: podNamespace,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: podNamespace,
					Annotations: map[string]string{
						AwsRoleARNKey: "arn:aws:iam::123456789012:role/test-role",
					},
				},
			},
		).
		Build()

	tokenDetails := &TokenDetails{}

	secretRotatorPod := &corev1.Pod{}
	err := k8sClient.Get(
		ctx,
		types.NamespacedName{Name: podName, Namespace: podNamespace},
		secretRotatorPod,
	)
	require.NoError(t, err)

	serviceAccount := &corev1.ServiceAccount{}
	err = k8sClient.Get(
		ctx,
		types.NamespacedName{Name: serviceAccountName, Namespace: podNamespace},
		serviceAccount,
	)
	require.NoError(t, err)

	tokenDetails.DefaultServiceAccountNamespace = podNamespace
	tokenDetails.DefaultServiceAccountName = serviceAccountName

	assert.NotNil(t, serviceAccount)
	assert.Equal(t, serviceAccountName, serviceAccount.Name)
	assert.Equal(t, podNamespace, serviceAccount.Namespace)
	assert.Equal(t, serviceAccountName, tokenDetails.DefaultServiceAccountName)
	assert.Equal(t, podNamespace, tokenDetails.DefaultServiceAccountNamespace)
}

func TestIsExist_Success(t *testing.T) {
	namespaceLabels := map[string]string{"environment": "dev", "region": "us-east-1"}
	objectLabels := map[string]string{"environment": "dev"}
	assert.True(t, IsExist(namespaceLabels, objectLabels))

	objectLabels = map[string]string{"environment": "dev", "region": "us-east-1"}
	assert.True(t, IsExist(namespaceLabels, objectLabels))

	objectLabels = map[string]string{}
	assert.True(t, IsExist(namespaceLabels, objectLabels))
}

func TestGetRandomString_Success(t *testing.T) {
	randomString := GetRandomString()
	assert.Len(t, randomString, 10)
	// Basic check that it contains only lowercase letters (not strictly enforced by the function)
	for _, r := range randomString {
		assert.GreaterOrEqual(t, r, 'a')
		assert.LessOrEqual(t, r, 'z')
	}
}

func TestListSecretRotatorObjects_Success(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&jfrogv1alpha1.SecretRotator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rotator-1",
				Namespace: "default",
			},
			Spec: jfrogv1alpha1.SecretRotatorSpec{},
		},
		&jfrogv1alpha1.SecretRotator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rotator-2",
				Namespace: "kube-system",
			},
			Spec: jfrogv1alpha1.SecretRotatorSpec{},
		},
	).Build()

	rotators := ListSecretRotatorObjects(fakeClient)
	assert.Len(t, rotators.Items, 2)
	assert.Equal(t, "test-rotator-1", rotators.Items[0].Name)
	assert.Equal(t, "default", rotators.Items[0].Namespace)
	assert.Equal(t, "test-rotator-2", rotators.Items[1].Name)
	assert.Equal(t, "kube-system", rotators.Items[1].Namespace)
}

func TestHandlingNamespaceEvents_Success(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&jfrogv1alpha1.SecretRotator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rotator",
				Namespace: "default",
			},
			Spec: jfrogv1alpha1.SecretRotatorSpec{},
		},
	).Build()
	logger := log.FromContext(context.Background())

	obj := &jfrogv1alpha1.SecretRotator{}
	err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-rotator", Namespace: "default"}, obj)
	require.NoError(t, err)

	updated := HandlingNamespaceEvents(fakeClient, logger, obj)
	assert.True(t, updated)

	updatedObj := &jfrogv1alpha1.SecretRotator{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-rotator", Namespace: "default"}, updatedObj)
	require.NoError(t, err)
	assert.NotEmpty(t, updatedObj.Annotations["uid"])
	assert.Len(t, updatedObj.Annotations["uid"], 10)
}

func TestFileExists_Success(t *testing.T) {
	const testFile = "test_file.txt"
	err := CreateFile(testFile, "test content")
	require.NoError(t, err)
	defer func() {
		err := os.Remove(testFile)
		assert.NoError(t, err)
	}()

	assert.True(t, FileExists(testFile))
}

func TestCreateFile_Success(t *testing.T) {
	const testFile = "new_test_file.txt"
	const fileContent = "content to write"
	err := CreateFile(testFile, fileContent)
	require.NoError(t, err)
	defer func() {
		contentBytes, err := os.ReadFile(testFile)
		assert.NoError(t, err)
		assert.Equal(t, fileContent, string(contentBytes))

		err = os.Remove(testFile)
		assert.NoError(t, err)
	}()

	assert.FileExists(t, testFile)
}

func TestCreateDir_Success(t *testing.T) {
	const testDir = "test_directory"
	err := CreateDir(testDir)
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(testDir)
		assert.NoError(t, err)
	}()

	fileInfo, err := os.Stat(testDir)
	require.NoError(t, err)
	assert.True(t, fileInfo.IsDir())

	// Test creating an existing directory, should not error
	err = CreateDir(testDir)
	assert.NoError(t, err)
}

var scheme = runtime.NewScheme()

func init() {
	_ = jfrogv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
}
