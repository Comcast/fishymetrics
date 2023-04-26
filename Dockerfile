# syntax=docker/dockerfile:1

FROM golang:1.19 as build-env
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
RUN cd /go/src/github.com/comcast/fishymetrics && go build -a -ldflags "\
  -X \"github.com/comcast/fishymetrics/buildinfo.gitVersion=${VERSION}\"\
  -X \"github.com/comcast/fishymetrics/buildinfo.gitRevision=${REPO_REV}\"\
  -X \"github.com/comcast/fishymetrics/buildinfo.date=${DATE}\"\
" -v -o fishymetrics ./cmd/fishymetrics

FROM alpine
RUN echo "http://dl-cdn.alpinelinux.org/alpine/v3.16/main" > /etc/apk/repositories && \
    echo -e "http://dl-cdn.alpinelinux.org/alpine/v3.16/community" >> /etc/apk/repositories && \
    rm -rf /var/cache/apk/*
RUN apk update && apk add ca-certificates libc6-compat
WORKDIR /
COPY --from=build-env /go/src/github.com/comcast/fishymetrics/fishymetrics /bin

ENTRYPOINT ["/bin/fishymetrics"]
