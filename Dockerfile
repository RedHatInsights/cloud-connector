#FROM scratch
FROM golang:alpine
MAINTAINER dhorton@redhat.com

EXPOSE 8081

COPY ./connector_service ./connector_service
COPY ./connector-service-cert.pem ./connector-service-cert.pem
COPY ./connector-service-key.pem ./connector-service-key.pem

ENTRYPOINT ["./connector_service"]
