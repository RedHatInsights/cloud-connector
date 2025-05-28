# Use go-toolset as the builder image
# Once built, copy to a smaller image and run from there
FROM registry.access.redhat.com/ubi9/go-toolset as builder

WORKDIR /go/src/app

# The current ubi9 image does not include Go 1.24, so we specify it.
# Adding "auto" will allow a newer version to be downloaded if specified in go.mod
ARG GOTOOLCHAIN=go1.24.3+auto

COPY go.mod go.sum ./

RUN go mod download

COPY internal/ internal/
COPY cmd/ cmd/
COPY db/ db/
COPY Makefile .

USER root

RUN make build

# Using ubi8-minimal due to its smaller footprint
FROM registry.access.redhat.com/ubi9/ubi-minimal

RUN microdnf update -y

WORKDIR /

# Copy executable files from the builder image
COPY --from=builder /go/src/app/cloud-connector /cloud-connector
COPY --from=builder /go/src/app/migrate_db /migrate_db
COPY --from=builder /go/src/app/db/migrations /db/migrations/
COPY --from=builder /go/src/app/db_schema_dumper /db_schema_dumper
COPY --from=builder /go/src/app/stage_db_fixer /stage_db_fixer

COPY licenses/LICENSE /licenses/LICENSE

USER 1001

EXPOSE 8000 10000
