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
export TF_VAR_service_accounts=<k8s-service-account>     # Default value is jfrogoperatorsa
export TF_VAR_aws_iam_role_names=<aws-iam-role-name>     # Default value is jfrogoperatorrole
export TF_VAR_aws_iam_policy_names=<aws-iam-policy-name> # Default value is jfrogoperatorpolicy
export TF_VAR_operator_version=<operator-version>       # Default value is latest
export TF_VAR_service_users=<user-name>                  # Default value is admin

Required:

export TF_VAR_eks_cluster_name=<eks-cluster-name>
export TF_VAR_eks_region=<aws-region>
export TF_VAR_jfrog_url=<jfrog-artifactory-url>
export TF_VAR_jfrog_scoped_tokens=<jfrog-api-token>
```

### Example for multiple ARNs/Users:

```
export TF_VAR_namespace=demo
export TF_VAR_aws_iam_role_names="role1,role2"
export TF_VAR_aws_iam_policy_names="policy1,policy2"
export TF_VAR_service_accounts="sa1,sa2"
export TF_VAR_service_users="user1,user2"
export TF_VAR_jfrog_scoped_tokens="token1,token2"
export TF_VAR_eks_cluster_name=aws-operator-jfrog
export TF_VAR_eks_region=ap-northeast-3
export TF_VAR_jfrog_url="artifactory.jfrog.com"
export TF_VAR_operator_version=latest
```

4. Initialize Terraform

Run the following command to initialize the Terraform configuration:

```
terraform init
terraform plan
```

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
      kubernetes.io/metadata.name: jfrog-operator
  generatedSecrets:
  - secretName: token-imagepull-secret
    secretType: docker
  # - secretName: token-generic-secret
  #   secretType: generic
  artifactoryUrl: "artifactory.example.com"
      kubernetes.io/metadata.name: jfrogoperator
  refreshTime: 30m
  #  serviceAccount: # The default name and namespace will be the operator’s service account name and namespace
  #    name: ""
  #    namespace: ""
  secretMetadata:
    annotations:
      annotationKey: annotationValue
    labels:
      labelName: labelValue
  security:
    enabled: false
    secretNamespace:
    ## NOTE: You can provide either a ca.pem or ca.crt. But make sure that key needs to same as ca.crt or ca.pem in secret
    certificateSecretName:
    insecureSkipVerify: false
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

### Variables

| Variable Name                 | Description                               | Format Example      |
| ----------------------------- | ----------------------------------------- | ------------------- |
| `TF_VAR_namespace`            | Kubernetes namespace                      | `"demo"`            |
| `TF_VAR_aws_iam_role_names`   | IAM role names mapped to service accounts | `"role1,role2"`     |
| `TF_VAR_aws_iam_policy_names` | IAM policy names for each role            | `"policy1,policy2"` |
| `TF_VAR_service_accounts`     | Kubernetes service account names          | `"sa1,sa2"`         |
| `TF_VAR_service_users`        | JFrog service user names                  | `"user1,user2"`     |
| `TF_VAR_jfrog_scoped_tokens`  | Admin scoped tokens for JFrog users       | `"token1,token2"`   |
| `TF_VAR_eks_cluster_name`     | Name of the EKS cluster                   | `"aws-operator-jfrog"` |
| `TF_VAR_eks_region`           | AWS region where the EKS cluster is hosted | `"ap-northeast-3"`  |
| `TF_VAR_jfrog_url`            | URL for the JFrog Artifactory instance    | `"artifactory.jfrog.com"` |
| `TF_VAR_operator_version`     | Version of the JFrog operator             | `"latest"`          |


### Resources
| Resource Name                                            | Description                                                                           |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| `aws_iam_openid_connect_provider.eks_oidc_provider`      | Creates an OpenID Connect provider in AWS IAM.                                        |
| `aws_iam_role.aws_reg_operator_role`                     | Creates IAM roles for service accounts in the EKS cluster.                            |
| `aws_iam_policy.aws_reg_operator_policy`                 | Creates IAM policies for each IAM role.                                               |
| `aws_iam_role_policy_attachment.aws_operator_attachment` | Attaches IAM policies to the corresponding IAM roles.                                 |
| `null_resource.list_length_check`                        | Validates that the number of service accounts, IAM roles, policies, and tokens match. |
| `null_resource.curl_operations`                          | Executes curl commands to configure JFrog Platform for passwordless access to EKS.    |
| `helm_release.jfrog_operator`                            | Deploys the JFrog registry operator Helm chart to the Kubernetes cluster.             |

Troubleshooting

 - AWS Permissions: Ensure that the AWS credentials you're using have the necessary permissions to create IAM roles and policies.
 - Helm Chart Deployment Issues: Ensure that Helm is installed, the cluster is properly configured, and Terraform envs is set up correctly
 - Error: creating IAM OIDC Provider: This might occure due to Entity Already Exists. You may ignore this.
