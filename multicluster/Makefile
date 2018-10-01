# Licensed Materials - Property of IBM
# (C) Copyright IBM Corp. 2018. All Rights Reserved.
# US Government Users Restricted Rights - Use, duplication or
# disclosure restricted by GSA ADP Schedule Contract with IBM Corp.
# Copyright 2018 IBM Corporation

# Docker image and hub parameters. Replace with a valid repository.
DOCKER_REPO ?= docker.io/mc
IMAGE_TAG:=develop

# Go parameters
GOCMD=go

# Lint
gometalinter=gometalinter.v2

.PHONY: build

all: test lint

test: 
	$(GOCMD) test -v ./...

clean:
	$(GOCMD) clean
	rm -rf ./build

build:
	mkdir -p ./build/docker
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./build/docker/mcagent $(LDFLAGS) -ldflags "-X main.version=$(IMAGE_TAG)" ./cmd/mc-agent
	docker build -t $(DOCKER_REPO)/mcagent:$(IMAGE_TAG) -f mcagent.Dockerfile ./build/docker
	
push:
	docker push $(DOCKER_REPO)/mcagent:$(IMAGE_TAG)
	
lint:
	$(gometalinter) --config=../lintconfig_base.json ./...