# AWS STS CLI Debugging

This example requires an STS ROSA cluster and can be used to run arbitrary AWS CLI commands from a pod on the ROSA cluster that is restricted by an IAM policy.

This is more for exploration/demo purposes on how the STS/OIDC flow between ROSA and AWS works. Often it is more practical to iterate with the running operator directly.

## Prerequisites

* A running STS ROSA cluster
* terraform
* osdtcl
* aws

## Creating

1. Set convenience environment variables:

    ```bash
    AWS_ACCOUNT_ID=
    ROSA_INTERNAL_ID=
    ```

2. Get AWS CLI credentials:

    ```bash
    export $(osdctl account cli -i ${AWS_ACCOUNT_ID} -p osd-staging-2 -o env | xargs)
    ```

3. Provision the "iam-test-aws" AWS IAM role which will allow pods run by the "rosa-to-iam" K8s serviceaccount to assume it via the ROSA cluster's OIDC provider:

    ```bash
    terraform init
    terraform apply -var aws_account_id="${AWS_ACCOUNT_ID}" -var rosa_internal_id="${ROSA_INTERNAL_ID}"
    ```

4. Ensure the current Kubernetes context is set to your STS ROSA cluster and fill in the AWS_ACCOUNT_ID:

    ```bash
    oc apply -f aws-sts-debug.yml
    ```

5. ???

6. Profit

    ```bash
    oc exec -it -n aws-sts-debug awscli-debug -- bash
    aws sts get-caller-identity
    ```

## Cleanup

1. The Terraform resources can be cleaned up if desired (no resources costing money get provisioned)

    ```bash
    terraform destroy -var aws_account_id="${AWS_ACCOUNT_ID}" -var rosa_internal_id="${ROSA_INTERNAL_ID}"
    ```

## References

* [Fine-grained IAM roles for ROSA workloads with STS](https://aws.amazon.com/blogs/containers/fine-grained-iam-roles-for-red-hat-openshift-service-on-aws-rosa-workloads-with-sts/)
* [How does ROSA use STS?](https://cloud.redhat.com/blog/what-is-aws-sts-and-how-does-red-hat-openshift-service-on-aws-rosa-use-sts)
