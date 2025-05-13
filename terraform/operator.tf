provider "aws" {
  region = var.eks_region  # Use the eks_region variable for AWS region
}

variable "namespace" {
  description = "The Kubernetes namespace"
  type        = string
  default     = "jfrogoperator"
}

variable "service_account" {
  description = "The Kubernetes service account"
  type        = string
  default     = "jfrogoperatorsa"
}

variable "eks_cluster_name" {
  description = "The name of the EKS cluster"
  type        = string
}

variable "eks_region" {
  description = "The AWS region where the EKS cluster is located"
  type        = string
}

variable "jfrog_url" {
  description = "JFrog Artifactory url"
  type        = string
}

variable "jfrog_scoped_token" {
  description = "Scoped token for the user with the admin permissions"
  type        = string
}

variable "aws_iam_role_name" {
  description = "Aws iam role name"
  type        = string
  default     = "jfrogoperatorrole"
}

variable "aws_iam_policy_name" {
  description = "Aws iam policy name"
  type        = string
  default     = "jfrogoperatorpolicy"
}

variable "operator_version" {
  description = "Version of jfrog registry operator helm chart"
  type        = string
  default     = ""
}

# Fetch OIDC issuer URL from EKS
data "aws_eks_cluster" "eks" {
  name = var.eks_cluster_name  # Use the eks_cluster_name variable for the EKS cluster name
}

data "external" "eks_oidc_issuer" {
  program = [
    "bash",
    "-c",
    "echo '{\"issuer\": \"'$(aws eks describe-cluster --name ${var.eks_cluster_name} --region ${var.eks_region} --query \"cluster.identity.oidc.issuer\" --output text | sed -e \"s/^https:\\/\\///\")'\"}'"
  ]
}

data "external" "account_id" {
  program = [
    "bash",
    "-c",
    "echo '{\"account_id\": \"'$(aws sts get-caller-identity --query 'Account' --output text)'\"}'"
  ]
}

# Create OpenID Connect (OIDC) identity provider in AWS IAM
resource "aws_iam_openid_connect_provider" "eks_oidc_provider" {
  url             = "https://${data.external.eks_oidc_issuer.result["issuer"]}"  # OIDC URL
  client_id_list  = ["sts.amazonaws.com"]


  tags = {
    Name = "EKS OIDC Provider"
  }
}

output "eks_oidc_issuer" {
  value = data.external.eks_oidc_issuer.result
}

output "account_id" {
  value = data.external.account_id.result
}

output "oidc_provider_arn" {
  value = aws_iam_openid_connect_provider.eks_oidc_provider.arn
}

# Create a Role
resource "aws_iam_role" "aws_reg_operator_role" {
  name               = "${var.aws_iam_role_name}"
  assume_role_policy = jsonencode({
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Principal": {
          "Federated": "arn:aws:iam::${data.external.account_id.result["account_id"]}:oidc-provider/${data.external.eks_oidc_issuer.result["issuer"]}"
        },
        "Action": "sts:AssumeRoleWithWebIdentity",
        "Condition": {
          "StringEquals": {
            "${data.external.eks_oidc_issuer.result["issuer"]}:aud": "sts.amazonaws.com",
            "${data.external.eks_oidc_issuer.result["issuer"]}:sub": "system:serviceaccount:${var.namespace}:${var.service_account}"
          }
        }
      }
    ]
  })
}

# Create a Policy
resource "aws_iam_policy" "aws_reg_operator_policy" {
  name        = "${var.aws_iam_policy_name}"
  description = "An example policy to be attached to the IAM role"
  policy      = jsonencode({
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": "sts:GetCallerIdentity",
        "Resource": "${aws_iam_role.aws_reg_operator_role.arn}"  # Dynamically reference the ARN of the IAM role
      },
      {
        "Sid": "Statement1",
        "Effect": "Allow",
        "Action": [
          "iam:GetRole"
        ],
        "Resource": [
          "${aws_iam_role.aws_reg_operator_role.arn}"
        ]
      }
    ]
  })
}

# Attach IAM Policy to the Role
resource "aws_iam_role_policy_attachment" "aws_operator_attachment" {
  role       = aws_iam_role.aws_reg_operator_role.name
  policy_arn = aws_iam_policy.aws_reg_operator_policy.arn
}

# Configure JFrog Platform for Passwordless Access to EKS using the curl operations
resource "null_resource" "curl_operations" {
  depends_on = [aws_iam_role.aws_reg_operator_role, aws_iam_role_policy_attachment.aws_operator_attachment]

  provisioner "local-exec" {
    command = <<EOT
      curl -XPUT -H "Content-type: application/json" -H "Authorization: Bearer ${var.jfrog_scoped_token}" \
      ${var.jfrog_url}/access/api/v1/aws/iam_role \
      -d '{"username":"admin", "iam_role": "${aws_iam_role.aws_reg_operator_role.arn}"}' -vvv

      curl -H "Content-type: application/json" -H "Authorization: Bearer  ${var.jfrog_scoped_token}" \
      ${var.jfrog_url}/access/api/v1/aws/iam_role/admin -vvv
    EOT
  }
}

# Install the JFrog Registry Operator in EKS
resource "null_resource" "curl_operations_helm" {
  depends_on = [null_resource.curl_operations]

  triggers = {
    service_account = var.service_account
    namespace       = var.namespace
    role_arn        = aws_iam_role.aws_reg_operator_role.arn
    operator_version         = var.operator_version
  }
  provisioner "local-exec" {
    command = <<EOT
      # Check if operator_version is defined
      if [ -n "${var.operator_version}" ]; then
        # Run Helm upgrade/install with the operator version if defined
        helm upgrade --install secretrotator jfrog/jfrog-registry-operator --version ${var.operator_version} \
          --set "serviceAccount.name=${var.service_account}" \
          --set "serviceAccount.annotations=eks.amazonaws.com/role-arn: ${aws_iam_role.aws_reg_operator_role.arn}" \
          --create-namespace -n ${var.namespace}
      else
        # Run Helm upgrade/install without the operator version if not defined
        helm upgrade --install secretrotator jfrog/jfrog-registry-operator \
          --set "serviceAccount.name=${var.service_account}" \
          --set "serviceAccount.annotations=eks.amazonaws.com/role-arn: ${aws_iam_role.aws_reg_operator_role.arn}" \
          --create-namespace -n ${var.namespace}
      fi
    EOT
  }


  provisioner "local-exec" {
    when    = destroy
    command = <<EOT
      # Run Helm uninstall command when Terraform destroy is triggered
      helm uninstall secretrotator -n ${self.triggers["namespace"]}
    EOT
  }
}