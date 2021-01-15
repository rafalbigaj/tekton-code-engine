	BIN_DIR=_output/bin

# If tag not explicitly set in users default to the git sha.
TAG ?= v0.0.1

.EXPORT_ALL_VARIABLES:

all: local

init:
	mkdir -p ${BIN_DIR}

local: init
	go build -o=${BIN_DIR}/code-engine-controller ./cmd/controller
	go build -o=${BIN_DIR}/code-engine-webhook ./cmd/webhook

build-linux: init
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o=${BIN_DIR}/code-engine-controller ./cmd/controller
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o=${BIN_DIR}/code-engine-webhook ./cmd/webhook

images: build-linux
	docker build --build-arg bin_name=code-engine-controller . -t rafalbigaj/code-engine-controller:$(TAG)
	docker build --build-arg bin_name=code-engine-webhook . -t rafalbigaj/code-engine-webhook:$(TAG)

update:
	go mod download
	go mod tidy
	go mod vendor

clean:
	rm -rf _output/
	rm -f *.log
