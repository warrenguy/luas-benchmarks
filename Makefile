SHELL := /bin/bash

# scripts/luas-checkout clones Luas at the go.mod commit into .cache/luas.
LUAS_DIR := $(CURDIR)/.cache/luas
# Aeron ref from the Luas commit's specs/versions.yaml. Empty when missing.
AERON_REF = $(shell awk '/^[^ \#]/ { in_aeron = ($$1 == "aeron:") } \
	in_aeron && $$1 == "ref:" { print $$2; exit }' $(LUAS_DIR)/specs/versions.yaml 2>/dev/null)

export AERON_DIR ?= /dev/shm/luas-benchmarks
export AERONMD := $(LUAS_DIR)/bin/aeronmd

.PHONY: help
help: ## List Makefile targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

.PHONY: luas-checkout
luas-checkout: ## Clone or fetch Luas at the go.mod commit into .cache/luas
	scripts/luas-checkout

.PHONY: driver
driver: luas-checkout ## Build aeronmd at the Aeron ref Luas pins
	$(MAKE) -C $(LUAS_DIR) bin/aeronmd

.PHONY: build
build: build-go build-java ## Build every client

.PHONY: build-go
build-go: ## Build the luas, aerongo and run record binaries into build/
	go build -o build/luas ./luas/
	go build -o build/aerongo ./aerongo/
	go build -o build/record ./report/record/

.PHONY: build-java
build-java: luas-checkout ## Build target/bench.jar against the Aeron ref Luas pins
	@test -n "$(AERON_REF)" || { \
		echo "build-java: no aeron.ref in $(LUAS_DIR)/specs/versions.yaml" >&2; exit 1; }
	mvn -q -f pom.xml -Daeron.version=$(AERON_REF) package

.PHONY: histogram-fixture
histogram-fixture: build-java ## Write the Java HdrHistogram percentiles and encoding that the Go histogram tests expect
	java -cp target/bench.jar bench.HistogramFixture > target/histogram-java.json
	mv target/histogram-java.json bench/testdata/histogram-java.json

.PHONY: lint
lint: ## golangci-lint and shellcheck
	go tool golangci-lint run ./...
	shellcheck scripts/archive scripts/cluster scripts/driver scripts/ec2-matrix \
		scripts/luas-checkout scripts/run-matrix scripts/smoke

.PHONY: unit
unit: ## Unit and property tests
	go test -race -count=1 -timeout=30s ./...

.PHONY: check
check: lint unit ## Lint, unit and property tests

.PHONY: matrix
matrix: driver build ## Run every scenario of every client into results/<run-id>; SCENARIOS limits the scenarios
	scripts/run-matrix $(SCENARIOS)

.PHONY: smoke
smoke: driver build ## Run the connect check and every scenario of every client against the pinned driver, the Java archive and the Java cluster
	scripts/smoke
