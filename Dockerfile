FROM golang:1.20-alpine AS build_deps

RUN apk add --no-cache git

WORKDIR /workspace

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

COPY . .

RUN CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .

FROM alpine:3.18

RUN apk add --no-cache ca-certificates

COPY --from=build /workspace/webhook /usr/local/bin/webhook

ARG VERSION
ARG VCS_REF
ARG BUILD_DATE
LABEL org.label-schema.build-date=${BUILD_DATE} \
      org.label-schema.vcs-ref=${VCS_REF} \
      org.label-schema.vcs-url="https://github.com/exoscale/cert-manager-webhook-exoscale" \
      org.label-schema.version=${VERSION} \
      org.label-schema.name="cert-manager-webhook-exoscale" \
      org.label-schema.vendor="Exoscale" \
      org.label-schema.description="Cert-manager Webhook for Exoscale" \
      org.label-schema.schema-version="1.0"

ENTRYPOINT ["webhook"]
