FROM registry.access.redhat.com/ubi8/go-toolset

WORKDIR /go/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

USER root

RUN make build

RUN REMOVE_PKGS="binutils kernel-headers nodejs nodejs-full-i18n npm" && \
    yum remove -y $REMOVE_PKGS && \
    yum clean all

USER 1001

EXPOSE 8000 10000
