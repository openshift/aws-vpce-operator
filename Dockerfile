FROM registry.ci.openshift.org/openshift/release:golang-1.18 AS builder

ENV PKG=/go/src/github.com/openshift/aws-vpce-operator/
WORKDIR ${PKG}

# compile test binary
COPY . .
RUN make osde2e

FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

COPY --from=builder /go/src/github.com/openshift/aws-vpce-operator/osde2e.test osde2e.test

ENTRYPOINT [ "/osde2e.test" ]