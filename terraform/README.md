## Install JFrog Registry Operator Using Terraform

This Terraform configuration sets up AWS IAM roles and policies for EKS to integrate with JFrog Artifactory using OpenID Connect (OIDC). It provisions an IAM role for a Kubernetes service account and configures JFrog with this IAM role using the provided scoped token. Helm is used for deploying the operator to the EKS cluster.

### Prerequisites

- Terraform 1.0+: Ensure Terraform is installed on your machine. You can download it from the official website: https://www.terraform.io/downloads.html.
- AWS CLI: The AWS CLI is required for AWS-related commands. You can install it from the AWS CLI documentation: https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html.
- kubectl: You need kubectl for interacting with the EKS cluster. Install it from Kubernetes official docs: https://kubernetes.io/docs/tasks/tools/install-kubectl/.
- Helm: Helm is used to install and manage Kubernetes packages. Install it from the Helm website: https://helm.sh/docs/intro/install/.
- JFrog Platform: You'll need a JFrog URL and a scoped token with the required permissions to integrate with AWS IAM.

### Steps For Installation

1. Clone the Repository

First, clone the repository containing the Terraform configuration to your local machine.

```
git clone https://github.com/jfrog/jfrog-registry-operator.git
cd jfrog-registry-operator/terraform
```

2. Set Up AWS Credentials

Ensure your AWS credentials are set up using the AWS CLI. You can configure AWS credentials by running:

`
aws configure
`

You'll need your AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, and AWS_DEFAULT_REGION.

3. Configure Environment Variables

You need to export the following environment variables to run the Terraform configuration:

```
Optional:

export TF_VAR_namespace=<k8s-namespace>                 # Default value is jfrogoperator
export TF_VAR_service_account=<k8s-service-account>     # Default value is jfrogoperatorsa
export TF_VAR_aws_iam_role_name=<aws-iam-role-name>     # Default value is jfrogoperatorrole
export TF_VAR_aws_iam_policy_name=<aws-iam-policy-name> # Default value is jfrogoperatorpolicy
export TF_VAR_operator_version=<operator-version>       # Default value is latest

Required:

export TF_VAR_eks_cluster_name=<eks-cluster-name>
export TF_VAR_eks_region=<aws-region>
export TF_VAR_jfrog_url=<jfrog-artifactory-url>
export TF_VAR_jfrog_scoped_token=<jfrog-api-token>
```

4. Initialize Terraform

Run the following command to initialize the Terraform configuration:

`
terraform init
`

This will download the necessary provider plugins and set up the backend for Terraform.

5. Apply the Terraform Configuration

Run the following command to apply the Terraform configuration:

`
terraform apply
`

Terraform will prompt you to confirm the changes before provisioning resources. Type yes to proceed.

6. (Optional) Destroy the Resources

If you want to remove the created resources, run the following command:

`
terraform destroy
`

7. Verification

After applying the configuration, verify the deployed Helm release using kubectl:

`
kubectl get all -n <k8s-namespace>
`

8. Install SecretRotator Manifest

```
apiVersion: apps.jfrog.com/v1alpha1
kind: SecretRotator
metadata:
  labels:
    app.kubernetes.io/name: secretrotators.apps.jfrog.com
    app.kubernetes.io/instance: secretrotator
    app.kubernetes.io/created-by: artifactory-secrets-rotator
  name: secretrotator
spec:
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: jfrogoperator
  secretName: token-secret
  artifactoryUrl: "<artifactory-url>"
  refreshTime: 30m
  secretMetadata:
    annotations:
      annotationKey: annotationValue
    labels:
      labelName: labelValue
```

9. Secret Verification

Open kubernetes namespaces to check secret has been created

`
kubectl get secrets -n <k8s-namespace>
`

This will show the deployed pods, services, and other resources in the specified namespace.

Code Explanation

AWS Provider Configuration

The AWS provider is used to interact with AWS resources such as IAM roles, policies, and external services. The region is set dynamically via the eks_region variable.

Variables

```
namespace: The Kubernetes namespace where the operator will be deployed.
service_account: The Kubernetes service account that will be used with the IAM role.
eks_cluster_name: The name of the AWS EKS cluster.
eks_region: The AWS region where the EKS cluster is located.
jfrog_url: The URL of your JFrog Artifactory instance.
jfrog_scoped_token: The scoped API token for JFrog Artifactory.
aws_iam_role_name: Aws iam role name
aws_iam_policy_name: Aws iam policy name
operator_version: Version of jfrog registry operator helm chart
```

Resources
 - aws_iam_openid_connect_provider: This resource sets up an OIDC provider in AWS for the EKS cluster.
 - aws_iam_role: Creates an IAM role that can be assumed by the Kubernetes service account using OIDC.
 - aws_iam_policy: Defines a policy attached to the IAM role with necessary permissions.
 - aws_iam_role_policy_attachment: Attaches the policy to the IAM role.
 - (curl_operations): Executes curl commands to configure JFrog user with the IAM ARN.
 - (curl_operations_helm): Runs Helm commands to install the JFrog Registry Operator

Troubleshooting

 - AWS Permissions: Ensure that the AWS credentials you're using have the necessary permissions to create IAM roles and policies.
 - Helm Chart Deployment Issues: Ensure that Helm is installed, the cluster is properly configured, and Terraform envs is set up correctly
 - Error: creating IAM OIDC Provider: This might occure due to Entity Already Exists. You may ignore this.