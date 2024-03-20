# Use go-toolset as the builder image
# Once built, copy to a smaller image and run from there
FROM registry.access.redhat.com/ubi8/go-toolset as builder

WORKDIR /go/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY internal/ internal/
COPY cmd/ cmd/
COPY db/ db/
COPY Makefile .

USER root

RUN make build

# Using ubi8-minimal due to its smaller footprint
FROM registry.access.redhat.com/ubi8/ubi-minimal

RUN microdnf update -y

WORKDIR /

# Copy executable files from the builder image
COPY --from=builder /go/src/app/cloud-connector /cloud-connector
COPY --from=builder /go/src/app/migrate_db /migrate_db
COPY --from=builder /go/src/app/db/migrations /db/migrations/
COPY --from=builder /go/src/app/db_schema_dumper /db_schema_dumper
COPY --from=builder /go/src/app/stage_db_fixer /stage_db_fixer
COPY --from=builder /go/src/app/load_tester /load_tester
COPY --from=builder /go/src/app/test_script.sh /test_script.sh
COPY --from=builder /go/src/app/sleeper_script.sh /sleeper_script.sh

USER 1001

EXPOSE 8000 10000
