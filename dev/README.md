# Development Guide

## Prerequisites

* An STS ROSA cluster

## Running locally

1. Applying manifests to the running ROSA cluster

    ```bash
    ./boilerplate/_lib/container-make generate
    make install
    ```

2. Setup local AWS environment variables for the locally running operator to use

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

3. Create a dummy VPC Endpoint Service in AWS to connect to. It's pretty straightforward to do in the UI as you can pick the load balancers that are created by
the cluster as targets so that you don't need to manage your own. Once it exists note its name and fill it into `vpce_example.yml`

    > NOTE: Remember to delete the VPC endpoint service or else the normal cluster deletion process will fail

    * Name - Doesn't matter
    * Load balancer type - Network
    * Available load balancers - int or ext (doesn't matter)
    * Additional settings - Acceptance required

4. ???

5. Profit

    ```bash
    make run ENABLE_WEBHOOKS=false
    ```

    ```bash
    oc apply -f vpce_example.yml
    # Testing and other shenanigans
    oc delete -f vpce_example.yml
    ```
