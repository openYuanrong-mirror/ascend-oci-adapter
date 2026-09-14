# Copyright (c) 2026 Ant Group Corporation.
# SPDX-License-Identifier: Apache-2.0

GO ?= go
GOOS ?= linux
GOARCH ?= amd64
OUTPUT ?= output/ascend-oci-adapter
VERSION ?= devel

.PHONY: build test fmt vet clean release

build:
	@mkdir -p "$(dir $(OUTPUT))"
	cd cmd/ascend-oci-adapter && \
		CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build -trimpath -ldflags="-s -w -X main.adapterVersion=$(VERSION)" \
		-o "../../$(OUTPUT)" .

test:
	cd cmd/ascend-oci-adapter && $(GO) test ./...

fmt:
	@test -z "$$(cd cmd/ascend-oci-adapter && $(GO) fmt ./...)"

vet:
	cd cmd/ascend-oci-adapter && $(GO) vet ./...

release:
	@test -n "$(VERSION)"
	./scripts/build-release.sh "$(VERSION)"

clean:
	rm -rf output dist
