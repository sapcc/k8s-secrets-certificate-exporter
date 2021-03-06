TAG     = $(shell git rev-parse --short HEAD)
IMAGE   ?= sapcc/k8s-secrets-certificate-exporter
GOOS    ?= $(shell go env | grep GOOS | cut -d'"' -f2)
BINARY  := certificate-exporter

LDFLAGS := -X github.com/sapcc/k8s-secrets-certificate-exporter/pkg/exporter.VERSION=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS)"

SRCDIRS  := cmd pkg
PACKAGES := $(shell find $(SRCDIRS) -type d)
GOFILES  := $(addsuffix /*.go,$(PACKAGES))
GOFILES  := $(wildcard $(GOFILES))

GLIDE := $(shell command -v glide 2> /dev/null)

.PHONY: all clean vendor tests static-check

all: bin/$(GOOS)/$(BINARY)

bin/%/$(BINARY): $(GOFILES) Makefile
	GOOS=$* GOARCH=amd64 go build $(GOFLAGS) -v -i -o bin/$*/$(BINARY) ./cmd

build: bin/linux/$(BINARY)
	docker build -t $(IMAGE):$(TAG) .

push: build
	docker push $(IMAGE):$(TAG)

clean:
	rm -rf bin/*

vendor:
	dep ensure
