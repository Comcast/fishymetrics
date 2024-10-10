REPO_VERSION ?= $(shell git rev-parse --git-dir > /dev/null 2>&1 && git fetch -q origin --tags && git describe --always --dirty --tags)
REPO_REV ?= $(shell git rev-parse --git-dir > /dev/null 2>&1 && git rev-parse HEAD 2>/dev/null)
BUILD_DATE := $(shell date -u +%FT%T)
TEST_PKGS := $(shell find . -name "*_test.go" -not -wholename "*/vendor/*" -exec dirname {} \; | uniq)

build:
	@mkdir -p build/usr/bin
	go build -a -ldflags "\
		-X \"github.com/comcast/fishymetrics/buildinfo.gitVersion=${REPO_VERSION}\"\
		-X \"github.com/comcast/fishymetrics/buildinfo.gitRevision=${REPO_REV}\"\
		-X \"github.com/comcast/fishymetrics/buildinfo.date=${BUILD_DATE}\"\
	" -v -o build/usr/bin/fishymetrics $(shell pwd)/cmd/fishymetrics

docker:
	docker build \
	--platform linux/amd64 \
	--build-arg VERSION=${REPO_VERSION} \
	--build-arg REPO_REV=${REPO_REV} \
	--build-arg DATE=${BUILD_DATE} \
	--target bin \
	-t comcast/fishymetrics:${REPO_VERSION} \
	.

docker-src:
	docker build \
	--build-arg VERSION=${REPO_VERSION} \
	--build-arg REPO_REV=${REPO_REV} \
	--build-arg DATE=${BUILD_DATE} \
	--target src \
	-t comcast/fishymetrics-src:${REPO_VERSION} \
	-t comcast/fishymetrics-src:latest \
	.

test:
	go test -v -cover -coverprofile cp.out -p 1 -race ${FLAGS} ${TEST_PKGS}

clean:
	rm -rf build/

.PHONY: clean test
