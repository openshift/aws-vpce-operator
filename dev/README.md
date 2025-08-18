# Development Guide

Since aws-vpce-operator creates both Kubernetes and AWS resources, it is difficult to fully emulate a test environment
locally as unit tests will only get us so far. Furthermore, as an [OpenShift operator leveraging AWS STS](https://cloud.redhat.com/blog/what-is-aws-sts-and-how-does-red-hat-openshift-service-on-aws-rosa-use-sts),
this guide will also help you better understand the background mechanism of how the operator manages resources in AWS.

There are two main development environments:

   1. Running the operator locally against a remote STS ROSA cluster and associated AWS account
   2. Running the operator as a K8s deployment within a remote STS ROSA cluster and associated AWS account

The development environment chosen is mostly a matter of personal preference. Running the operator locally will allow
you to get faster feedback at the cost of additional local setup, while running it within a remote STS ROSA cluster will
allow you to test the operator without having to consider the local environment.

## Warnings and Caveats

- Unlike most other Kubernetes operators, AVO creates AWS resources and cannot necessarily be depended on to have the
opportunity to clean up after itself. Of course, as logic is written, the operator should still be able to correctly handle the
creation, modification, and deletion of custom resources, but a customer may also choose to simply uninstall their
cluster. This has the effect of immediately shutting down all EC2 instances underlying the cluster, leaving no time for
cleanup. Thankfully, [Hive will cleanup AWS resources with a specific tag](https://github.com/openshift/hive/blob/2801c6b99a37e915382ef0858586688efa9416f5/pkg/install/generate.go#L545-L577).
So when creating new AWS resources, ensure that all of them have the required tags. A helper function is available in 
[pkg/util/naming.go](https://github.com/openshift/aws-vpce-operator/blob/fb4ba0bc5f93bc4a10e0d8640b1ace8552d4bf0b/pkg/util/naming.go#L33-L35).

## Prerequisites

* An STS ROSA cluster (PrivateLink optional)

## Shared Setup

1. Generate and apply the CRD(s)

    > NOTE: CRD generation only needs to be done if you have modified the contents of `./api/` during development.

    ```bash
   # Generate the CRD
    ./boilerplate/_lib/container-make generate
    # Apply the CRD to the cluster
    make install
    ```

2. A sample CR is available in `vpce_example.yml`, but it will need a valid AWS VPC Endpoint Service name.
It's pretty straightforward to create one in the UI as you can pick the load balancers that are created by the cluster
as targets so that you don't need to manage your own load balancer. Once it exists note its name and fill it into `vpce_example.yml`

   > NOTE: Remember to delete the VPC endpoint service or else the normal cluster deletion process will fail

   * Name - Doesn't matter
   * Load balancer type - Network
   * Available load balancers - int or ext (doesn't matter)
   * Additional settings - Acceptance required

3. (Optional if running locally) Create an IAM role with an associated trust policy. There's a sample in [dev/sts-oidc/main.tf](./sts-oidc/main.tf) which will create an IAM role with the correct structure. The LocalDev statement is useful if you'd like to run the operator locally, while the OidcTrustPolicy statement is required if running the operator within an STS cluster.

    ```json
    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Sid": "LocalDev",
          "Effect": "Allow",
          "Principal": {
            "AWS": "arn:aws:iam::${ORG_MANAGEMENT_ACCOUT_ID}:root"
          },
          "Action": "sts:AssumeRole"
        },
        {
          "Sid": "OidcTrustPolicy",
          "Effect": "Allow",
          "Principal": {
            "Federated": "arn:aws:iam::${AWS_ACCOUNT_ID}:oidc-provider/rh-oidc-staging.s3.us-east-1.amazonaws.com/${INTERNAL_CLUSTER_ID}"
          },
          "Action": "sts:AssumeRoleWithWebIdentity",
          "Condition": {
            "StringEquals": {
              "rh-oidc-staging.s3.us-east-1.amazonaws.com/${INTERNAL_CLUSTER_ID}:sub": "system:serviceaccount:${SERVICE_ACCOUNT_NAMESPACE}:${SERVICE_ACCOUNT_NAME}"
            }
          }
        }
      ]
    }
    ```

## Running locally

This method is often lower lift since it doesn't require a container image to be built and pushed. The operator is
run locally (i.e. with `go run .`) and depends on local K8s and AWS credentials to interact with a K8s cluster and AWS account.

1. Setup local AWS environment variables for the locally running operator to use

    ```bash
    # Admin-level, for when you don't want to deal with a least-privilege IAM policy
    AWS_ACCOUNT_ID=
    export $(osdctl account cli -i ${AWS_ACCOUNT_ID} -p osd-staging-2 -o env | xargs)
    ```

    ```bash
    # Using a specific IAM role named "iam-test-aws" that can be created in AWS
    AWS_ACCOUNT_ID=
    OUT=$(aws sts assume-role --role-arn arn:aws:iam::${AWS_ACCOUNT_ID}:role/iam-test-aws --role-session-name anything --profile osd-staging-2);\
    export AWS_ACCESS_KEY_ID=$(echo $OUT | jq -r '.Credentials''.AccessKeyId');\
    export AWS_SECRET_ACCESS_KEY=$(echo $OUT | jq -r '.Credentials''.SecretAccessKey');\
    export AWS_SESSION_TOKEN=$(echo $OUT | jq -r '.Credentials''.SessionToken');
    ```

2. Run the operator

    ```bash
    # We currently have no webhooks anyway, so ENABLE_WEBHOOKS=false is optional
    make run ENABLE_WEBHOOKS=false
   
    # Running with debug logs enabled
    make run ARGS="--zap-log-level=debug" ENABLE_WEBHOOKS=false
    ```

## Running in a ROSA STS cluster

1. Build and push a container image for the operator (or use an existing image from [quay.io/app-sre/aws-vpce-operator](https://quay.io/repository/app-sre/aws-vpce-operator?tab=tags)).

2. Update the container image in `./deploy/20_operator.yml`

3. Create the namespace
    ```bash
    oc create namespace openshift-aws-vpce-operator
    ```

4. Create a secret with your AWS credentials
    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: avo-aws-iam-user-creds
      namespace: openshift-aws-vpce-operator
    type: Opaque
    data:
      aws_secret_access_key: <base64 $AWS_ACCESS_KEY_ID>
      aws_access_key_id: <base64 $AWS_SECRET_ACCESS_KEY>
    ```

5. Apply all the resources (namespace, RBAC, deployment) to the cluster.

    ```bash
    oc apply -f deploy
    ```

6. Apply all the CRDs on deploy/crds/
    ```bash
    oc apply -f deploy/crds/
    ```

## Profit

Once the operator is running somewhere, you're now ready to give it some CRs to work on!

```bash
oc apply -f vpce_example.yml
# Testing and other shenanigans
oc delete -f vpce_example.yml
```
