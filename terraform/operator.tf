provider "aws" {
  region = var.eks_region
}

provider "helm" {
  kubernetes = {
    config_path = "~/.kube/config"  # or use var.kubeconfig
  }
}

variable "namespace" {
  description = "The Kubernetes namespace"
  type        = string
  default     = "jfrogoperator"
}

variable "service_accounts" {
  description = "The Kubernetes service accounts (comma-separated)"
  type        = string
  default     = "jfrog-operator-sa"
}

variable "service_account_namespace_pairs" {
  description = "The Kubernetes namespaces for each service account (comma-separated)"
  type        = string
  default     = "jfrog-operator-sa:jfrogoperator"
}

variable "aws_iam_role_names" {
  description = "Aws iam role names (comma-separated)"
  type        = string
  default     = "jfrogoperatorrole"
}

variable "aws_iam_policy_names" {
  description = "Aws iam policy names (comma-separated)"
  type        = string
  default     = "jfrogoperatorpolicy"
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

variable "jfrog_scoped_tokens" {
  description = "Scoped tokens for the users with admin permissions (comma-separated)"
  type        = string
}

variable "service_users" {
  description = "Service users for JFrog IAM role mapping (comma-separated)"
  type        = string
  default     = "admin"
}

variable "operator_version" {
  description = "Version of jfrog registry operator helm chart"
  type        = string
  default     = ""
}

# New variables for Helm configuration
variable "helm_chart_name" {
  description = "The name of the JFrog Helm chart"
  type        = string
  default     = "jfrog/jfrog-registry-operator"
}

variable "helm_release_name" {
  description = "The release name for the JFrog Helm deployment"
  type        = string
  default     = "secretrotator"
}

variable "install_update_crd" {
  type    = bool
  default = false
}

variable "scope" {
  type    = string
  default = "cluster"
}

# Locals to split the comma-separated strings into lists
locals {
  service_account_list = split(",", var.service_accounts)
  iam_role_name_list    = split(",", var.aws_iam_role_names)
  iam_policy_name_list = split(",", var.aws_iam_policy_names)
  jfrog_token_list      = split(",", var.jfrog_scoped_tokens)
  service_user_list    = split(",", var.service_users)

  # Split the namespace mapping string into a list of "sa:ns" pairs
  service_account_namespace_pairs_list = split(",", var.service_account_namespace_pairs)

  # Convert pairs into a map { sa => namespace }
  service_account_namespace_map = {
    for idx, pair in local.service_account_namespace_pairs_list :
    split(":", pair)[0] => (
      # If there is only one service account, override namespace with var.namespace
      length(local.service_account_list) == 1 && idx == 0 ?
      local.default_namespace :
      split(":", pair)[1]
    )
  }

  default_namespace = var.namespace
}

resource "null_resource" "install_crd" {
  count = var.install_update_crd ? 1 : 0

  provisioner "local-exec" {
    command = <<EOT
kubectl apply -f ${
  var.scope == "cluster"
  ? "https://raw.githubusercontent.com/jfrog/jfrog-registry-operator/refs/heads/v3.0.0/config/crd/bases/apps.jfrog.com_secretrotators_cluster_scope.yaml"
  : "https://raw.githubusercontent.com/jfrog/jfrog-registry-operator/refs/heads/v3.0.0/config/crd/bases/apps.jfrog.com_secretrotators_namespaced_scope.yaml"
}
EOT
  }
}

# Ensure the lists have the same number of elements and explicitly fail during the apply phase if not (for older Terraform versions)
resource "null_resource" "list_length_check" {
  count = 1

  provisioner "local-exec" {
    command = <<EOT
      expected_count=$(echo "${var.service_accounts}" | tr ',' '\n' | wc -l)
      role_count=$(echo "${var.aws_iam_role_names}" | tr ',' '\n' | wc -l)
      policy_count=$(echo "${var.aws_iam_policy_names}" | tr ',' '\n' | wc -l)
      token_count=$(echo "${var.jfrog_scoped_tokens}" | tr ',' '\n' | wc -l)
      user_count=$(echo "${var.service_users}" | tr ',' '\n' | wc -l)

      if [ "$expected_count" -ne "$role_count" ] || [ "$expected_count" -ne "$policy_count" ] || [ "$expected_count" -ne "$token_count" ]; then
        echo "Error: The number of elements in service_accounts, aws_iam_role_names, aws_iam_policy_names, jfrog_scoped_tokens, and service_users must be the same."
        exit 1
      else
        echo "List length check passed."
      fi
    EOT
  }
}

# Fetch OIDC issuer URL from EKS
data "aws_eks_cluster" "eks" {
  name = var.eks_cluster_name
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
  url             = "https://${data.external.eks_oidc_issuer.result["issuer"]}"
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

# Create IAM Roles for each service account
resource "aws_iam_role" "aws_reg_operator_role" {
  count = length(local.service_account_list)
  name  = element(local.iam_role_name_list, count.index)

  assume_role_policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Principal" : {
          "Federated" : "arn:aws:iam::${data.external.account_id.result["account_id"]}:oidc-provider/${data.external.eks_oidc_issuer.result["issuer"]}"
        },
        "Action" : "sts:AssumeRoleWithWebIdentity",
        "Condition" : {
          "StringEquals" : {
            # Dynamically use the namespace for this service account
            "${data.external.eks_oidc_issuer.result["issuer"]}:aud" : "sts.amazonaws.com",
            "${data.external.eks_oidc_issuer.result["issuer"]}:sub" : "system:serviceaccount:${lookup(local.service_account_namespace_map, element(local.service_account_list, count.index), local.default_namespace)}:${element(local.service_account_list, count.index)}"
          }
        }
      }
    ]
  })
}

# Create IAM Policies
resource "aws_iam_policy" "aws_reg_operator_policy" {
  count       = length(local.service_account_list)
  name        = element(local.iam_policy_name_list, count.index)
  description = "Policy for ${element(local.iam_role_name_list, count.index)}"
  policy      = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : "sts:GetCallerIdentity",
        "Resource" : aws_iam_role.aws_reg_operator_role[count.index].arn
      },
      {
        "Sid" : "Statement1",
        "Effect" : "Allow",
        "Action" : [
          "iam:GetRole"
        ],
        "Resource" : [
          aws_iam_role.aws_reg_operator_role[count.index].arn
        ]
      }
    ]
  })
}

# Attach IAM Policies to the Roles
resource "aws_iam_role_policy_attachment" "aws_operator_attachment" {
  count      = length(local.service_account_list)
  role       = aws_iam_role.aws_reg_operator_role[count.index].name
  policy_arn = aws_iam_policy.aws_reg_operator_policy[count.index].arn
}

# Configure JFrog Platform for Passwordless Access to EKS using the curl operations
resource "null_resource" "curl_operations" {
  count = length(local.service_account_list)

  provisioner "local-exec" {
    command = <<EOT
      curl -XPUT -H "Content-type: application/json" -H "Authorization: Bearer ${element(local.jfrog_token_list, count.index)}" \
      ${var.jfrog_url}/access/api/v1/aws/iam_role \
      -d '{"username":"${element(local.service_user_list, count.index)}", "iam_role": "${aws_iam_role.aws_reg_operator_role[count.index].arn}"}' -vvv

      curl -H "Content-type: application/json" -H "Authorization: Bearer ${element(local.jfrog_token_list, count.index)}" \
      ${var.jfrog_url}/access/api/v1/aws/iam_role/${element(local.service_user_list, count.index)} -vvv
    EOT
  }
  depends_on = [null_resource.list_length_check]
}


resource "helm_release" "jfrog_operator" {
  name       = var.helm_release_name
  chart      = var.helm_chart_name
  namespace  = var.namespace
  version    = length(var.operator_version) > 0 ? var.operator_version : null
  atomic = true
  force_update = true
  dependency_update = true
  create_namespace = true

  values = [
    length(local.service_account_list) > 1 || local.service_account_list[0] != "jfrog-operator-sa" ? 
    # If service_account_list has entries, generate them dynamically
    "exchangedServiceAccounts:\n${join("\n", [
      for i, sa in local.service_account_list : <<EOL
      - name: ${sa}
        namespace: ${lookup(local.service_account_namespace_map, sa, local.default_namespace)}
        annotations:
          eks.amazonaws.com/role-arn: ${aws_iam_role.aws_reg_operator_role[i].arn}
EOL
    ])}" :
    # If service_account_list is empty, use default serviceAccount
    <<EOL
    serviceAccount:
      annotations: |
        eks.amazonaws.com/role-arn: ${aws_iam_role.aws_reg_operator_role[0].arn}
EOL
  ]

  depends_on = [aws_iam_role_policy_attachment.aws_operator_attachment]
}
