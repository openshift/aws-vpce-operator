FROM quay.io/app-sre/boilerplate:image-v5.0.0 AS builder

WORKDIR /workdir
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
COPY . .
RUN make go-build

###
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.9-1161

ENV USER_UID=1001 \
    USER_NAME=aws-vpce-operator

COPY --from=builder /workdir/build/_output/bin/* /usr/local/bin/

COPY build/bin /usr/local/bin
RUN /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

LABEL io.openshift.managed.name="aws-vpce-operator" \
      io.openshift.managed.description="AWS VPCE Operator"
