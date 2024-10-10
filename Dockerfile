# syntax=docker/dockerfile:1

FROM golang:1.23 AS build
COPY . /go/src/github.com/comcast/fishymetrics
WORKDIR /go/src/github.com/comcast/fishymetrics

ENV \
    GOARCH="amd64" \
    GOOS="linux"

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN go mod download

ARG VERSION
ARG REPO_REV
ARG DATE
RUN cd /go/src/github.com/comcast/fishymetrics && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags "\
    -X \"github.com/comcast/fishymetrics/buildinfo.gitVersion=${VERSION}\"\
    -X \"github.com/comcast/fishymetrics/buildinfo.gitRevision=${REPO_REV}\"\
    -X \"github.com/comcast/fishymetrics/buildinfo.date=${DATE}\"\
    " -v -o fishymetrics ./cmd/fishymetrics

RUN cd /; mkdir -p /sources; cd sources
# copy the vendor directory to comply with opensource license requirements
COPY vendor /sources/vendor/
# Build the sources tarball outside of /deps so it has to be copied explicitly
RUN cd /; tar -czf /sources.tgz sources

FROM alpine:latest AS certs
RUN apk --update --no-cache add ca-certificates

# 'bin' stage, copy in only the binary and dependencies
FROM scratch AS bin
WORKDIR /
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /go/src/github.com/comcast/fishymetrics/fishymetrics /

ENTRYPOINT ["/fishymetrics"]

# 'src' stage, build from 'bin', but with sources added
FROM bin AS src
COPY --from=build /sources.tgz /sources.tgz
