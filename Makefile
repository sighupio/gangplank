# Copyright 2017-present SIGHUP s.r.l
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
SHELL := /bin/sh
PROJECT_NAME := gangway

.DEFAULT_GOAL := help

# ----------------------------------------------------------------------------------------------------------------------
# Private variables
# ----------------------------------------------------------------------------------------------------------------------

_DOCKER_FILELINT_IMAGE=cytopia/file-lint:latest-0.8
_DOCKER_HADOLINT_IMAGE=hadolint/hadolint:v2.12.0
_DOCKER_JSONLINT_IMAGE=cytopia/jsonlint:1.6
_DOCKER_MAKEFILELINT_IMAGE=cytopia/checkmake:latest-0.5
_DOCKER_MARKDOWNLINT_IMAGE=davidanson/markdownlint-cli2:v0.8.1
_DOCKER_SHELLCHECK_IMAGE=koalaman/shellcheck-alpine:v0.9.0
_DOCKER_SHFMT_IMAGE=mvdan/shfmt:v3-alpine
_DOCKER_YAMLLINT_IMAGE=cytopia/yamllint:1
_DOCKER_CHART_TESTING_IMAGE=quay.io/helmpack/chart-testing:v3.9.0

# TODO: replace this image with a permission-monitor-specific one
_DOCKER_TOOLS_IMAGE=omissis/go-jsonschema:tools-latest

_PROJECT_DIRECTORY=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))

# ----------------------------------------------------------------------------------------------------------------------
# Utility functions
# ----------------------------------------------------------------------------------------------------------------------

#1: docker image
#2: script name
define run-script-docker
	@docker run --rm \
		-v ${_PROJECT_DIRECTORY}:/data \
		-w /data \
		--entrypoint /bin/sh \
		$(1) scripts/$(2).sh
endef

# check-variable-%: Check if the variable is defined.
check-variable-%:
	@[[ "${${*}}" ]] || (echo '*** Please define variable `${*}` ***' && exit 1)

# ----------------------------------------------------------------------------------------------------------------------
# Linting Targets
# ----------------------------------------------------------------------------------------------------------------------

.PHONY: license-check
license-check:
	@addlicense -c "SIGHUP s.r.l" -y 2017-present -v -l apache \
	-ignore "deployments/helm/permission-monitor/templates/**/*" \
	-ignore "dist/**/*" \
	-ignore "web-client/src/gen/**/*" \
	-ignore "web-client/node_modules/**/*" \
	-ignore "vendor/**/*" \
	-ignore "*.gen.go" \
	-ignore ".idea/*" \
	-ignore ".vscode/*" \
	-ignore ".go/**/*" \
	--check .

# ----------------------------------------------------------------------------------------------------------------------
# Formatting Targets
# ----------------------------------------------------------------------------------------------------------------------

.PHONY: license-add
license-add:
	@addlicense -c "SIGHUP s.r.l" -y 2017-present -v -l apache \
	-ignore "deployments/helm/permission-monitor/templates/**/*" \
	-ignore "dist/**/*" \
	-ignore "web-client/src/gen/**/*" \
	-ignore "web-client/node_modules/**/*" \
	-ignore "vendor/**/*" \
	-ignore "*.gen.go" \
	-ignore ".idea/*" \
	-ignore ".vscode/*" \
	-ignore ".go/**/*" \
	.

# ----------------------------------------------------------------------------------------------------------------------
# Golang Targets
# ----------------------------------------------------------------------------------------------------------------------

.PHONY: tools-go
tools-go:
	@scripts/tools-golang.sh

.PHONY: tools-brew
tools-brew:
	@scripts/tools-brew.sh

.PHONY: build
build:
	@scripts/build.sh

.PHONY: release
release:
	@scripts/release.sh
