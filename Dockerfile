### This is a generated file from Dockerfile.in ###
#@follow_tag(registry-proxy.engineering.redhat.com/rh-osbs/openshift-golang-builder:rhel_8_golang_1.17)
FROM registry.redhat.io/ubi8/go-toolset:1.17.12 AS builder

ENV BUILD_VERSION=${CI_CONTAINER_VERSION}
ENV OS_GIT_MAJOR=${CI_X_VERSION}
ENV OS_GIT_MINOR=${CI_Y_VERSION}
ENV OS_GIT_PATCH=${CI_Z_VERSION}
ENV SOURCE_GIT_COMMIT=${CI_CLUSTER_LOGGING_OPERATOR_UPSTREAM_COMMIT}
ENV SOURCE_GIT_URL=${CI_CLUSTER_LOGGING_OPERATOR_UPSTREAM_URL}
ENV REMOTE_SOURCES=${REMOTE_SOURCES:-.}
ENV REMOTE_SOURCES_DIR=${REMOTE_SOURCES_DIR:-.}
ENV APP_DIR=.
COPY $REMOTE_SOURCES $REMOTE_SOURCES_DIR

USER 0
RUN make build

#@follow_tag(registry-proxy.engineering.redhat.com/rh-osbs/openshift-ose-cli:v4.9)
FROM quay.io/openshift/origin-cli:4.11 AS origincli

#@follow_tag(registry.redhat.io/ubi8:latest)
FROM registry.redhat.io/ubi8:8.6

ENV APP_DIR=/opt/app-root/src

RUN INSTALL_PKGS=" \
      openssl \
      rsync \
      file \
      xz \
      " && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    mkdir /tmp/ocp-clo && \
    chmod og+w /tmp/ocp-clo

COPY --from=builder $APP_DIR/bin/cluster-logging-operator /usr/bin/
COPY --from=builder $APP_DIR/scripts/* /usr/bin/scripts/

RUN mkdir -p /usr/share/logging/

COPY --from=builder $APP_DIR/files/ /usr/share/logging/

COPY --from=builder $APP_DIR/bundle/manifests /manifests

COPY --from=origincli /usr/bin/oc /usr/bin
COPY --from=builder $APP_DIR/must-gather/collection-scripts/* /usr/bin/

# this is required because the operator invokes a script as `bash scripts/cert_generation.sh`
WORKDIR /usr/bin
CMD ["/usr/bin/cluster-logging-operator"]

LABEL \
        io.k8s.display-name="Cluster Logging Operator" \
        io.k8s.description="This is a component of OpenShift Container Platform that manages the lifecycle of the Aggregated logging stack." \
        io.openshift.tags="openshift,logging" \
        com.redhat.delivery.appregistry="false" \
        maintainer="AOS Logging <team-logging@redhat.com>" \
        License="Apache-2.0" \
        name="openshift/ose-cluster-logging-operator" \
        com.redhat.component="cluster-logging-operator-container" \
        io.openshift.maintainer.product="OpenShift Container Platform" \
        io.openshift.build.commit.id=${CI_CLUSTER_LOGGING_OPERATOR_UPSTREAM_COMMIT} \
        io.openshift.build.source-location=${CI_CLUSTER_LOGGING_OPERATOR_UPSTREAM_URL} \
        io.openshift.build.commit.url=${CI_CLUSTER_LOGGING_OPERATOR_UPSTREAM_URL}/commit/${CI_CLUSTER_LOGGING_OPERATOR_UPSTREAM_COMMIT} \
        version=${CI_CONTAINER_VERSION}

