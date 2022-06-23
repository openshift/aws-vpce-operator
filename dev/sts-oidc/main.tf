provider "aws" {
  region  = "us-east-2"
  profile = "osd-staging-2"
  assume_role {
    role_arn = "arn:aws:iam::${var.aws_account_id}:role/OrganizationAccountAccessRole"
  }

  default_tags {
    tags = {
      owner = "terraform"
    }
  }
}

variable "aws_account_id" {
  type        = string
  description = "AWS Account ID to deploy this into"
}

variable "rosa_internal_id" {
  type        = string
  description = "Internal ID of the ROSA cluster"
}

locals {
  osd_staging_2_account_id = "811685182089"
  oidc_provider            = "rh-oidc-staging.s3.us-east-1.amazonaws.com/${var.rosa_internal_id}"
  namespace                = "awscli-sts-debug"
  service_account_name     = "rosa-to-iam"
}

resource "aws_iam_role" "iam_test" {
  name               = "iam-test-aws"
  assume_role_policy = data.aws_iam_policy_document.trust_policy.json
}

data "aws_iam_policy_document" "trust_policy" {
  version = "2012-10-17"

  statement {
    sid     = "LocalDev"
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${local.osd_staging_2_account_id}:root"]
    }
  }

  statement {
    sid     = "OpenshiftStsOidcTrustPolicy"
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = ["arn:aws:iam::${var.aws_account_id}:oidc-provider/${local.oidc_provider}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider}:sub"
      values   = ["system:serviceaccount:${local.namespace}:${local.service_account_name}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider}:aud"
      values   = ["openshift"]
    }
  }
}

data "aws_iam_policy_document" "iam_test" {
  version = "2012-10-17"

  statement {
    sid       = "HelloWorld"
    effect    = "Allow"
    actions   = ["sts:GetCallerIdentity"]
    resources = ["*"]
  }

  statement {
    sid    = "ManageTags"
    effect = "Allow"
    actions = [
      # Manage tags and filter based on tags
      "ec2:CreateTags",
      "ec2:DeleteTags",
      "ec2:DescribeTags"
    ]
    resources = ["*"]
  }

  statement {
    sid    = "ManageVPCE"
    effect = "Allow"
    actions = [
      # Extract vpc-id by searching for subnets by tag-key
      "ec2:DescribeSubnets",
      # Create and manage security group in specific VPC
      "ec2:CreateSecurityGroup",
      "ec2:DeleteSecurityGroup",
      "ec2:DescribeSecurityGroups",
      # Create and manage security group rules
      "ec2:AuthorizeSecurityGroupIngress",
      "ec2:AuthorizeSecurityGroupEgress",
      "ec2:DescribeSecurityGroupRules",
      # Create and manage a VPC endpoint
      "ec2:CreateVpcEndpoint",
      "ec2:DeleteVpcEndpoints",
      "ec2:DescribeVpcEndpoints",
      "ec2:ModifyVpcEndpoint",
      # Create and manage a Route53 Record
      "route53:ChangeResourceRecordSets",
      "route53:ListHostedZonesByName",
      "route53:ListResourceRecordSets"
    ]
    resources = ["*"]
  }

}

resource "aws_iam_policy" "iam_test" {
  name   = "iam-test-aws"
  policy = data.aws_iam_policy_document.iam_test.json
}

resource "aws_iam_role_policy_attachment" "iam_test" {
  role       = aws_iam_role.iam_test.name
  policy_arn = aws_iam_policy.iam_test.arn
}
