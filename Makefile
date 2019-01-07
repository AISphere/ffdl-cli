#--------------------------------------------------------------------------#
#                                                                          #
# Copyright 2019 IBM Corporation                                           #
#                                                                          #
# Licensed under the Apache License, Version 2.0 (the "License");          #
# you may not use this file except in compliance with the License.         #
# You may obtain a copy of the License at                                  #
#                                                                          #
# http://www.apache.org/licenses/LICENSE-2.0                               #
#                                                                          #
# Unless required by applicable law or agreed to in writing, software      #
# distributed under the License is distributed on an "AS IS" BASIS,        #
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. #
# See the License for the specific language governing permissions and      #
# limitations under the License.                                           #
#--------------------------------------------------------------------------#

CLI_NAME=ffdl

WHOAMI ?= $(shell whoami)

# BLUEMIX_DIR = $(GOPATH)/src/github.ibm.com/Bluemix
# BLUEMIX_CLI_COMMON_DIR = $(BLUEMIX_DIR)/bluemix-cli-common
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
LDFLAGS = -ldflags "-X main.buildstamp=`date -u +%Y%m%d.%H%M%S` -X main.githash=`git rev-parse HEAD`"

all: build

build:
	go build $(LDFLAGS) -o bin/${CLI_NAME}

vet:
	go vet $(shell glide nv)

lint:              ## Run code linter
	-go list ./... | grep -v /vendor/ | xargs -L1 golint

cli-config:
	@echo "# To use the DLaaS gRPC CLI, set the following environment variables:"
	@echo "export DLAAS_USERID=user-$(WHOAMI)  # replace with your name"
	@echo "export DLAAS_GRPC=$(shell ./bin/clikubecontext.sh trainer-url)"
build-release:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/${CLI_NAME}_linux
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/${CLI_NAME}_osx
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/${CLI_NAME}_windows.exe

test:
	go test ./tests/... -v -timeout 20m
