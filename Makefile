SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

GOLANGCI_LINT_VERSION ?= v1.60.3
BIN_DIR ?= $(CURDIR)/bin

.PHONY: golangci-install
golangci-install:
	mkdir -p "$(BIN_DIR)"
	CI= SANDBOX= \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
	| sh -s -- -b "$(BIN_DIR)" $(GOLANGCI_LINT_VERSION)
	"$(BIN_DIR)/golangci-lint" version


## setup: install all build dependencies for ci
setup: golangci-install mod-download

init-temporal-docker:
	@echo "  >  Temporal install & configuration "
	docker run --name temporal-dev --rm -d -p 7233:7233 -p 8233:8233 temporalio/temporal:latest server start-dev --ip 0.0.0.0
	@sleep 5
	docker exec -it temporal-dev temporal operator search-attribute create --namespace default --name CustomerID --type Keyword
	docker exec -it temporal-dev temporal operator search-attribute create --namespace default --name CustomerID --type Keyword
	docker exec -it temporal-dev temporal operator search-attribute create --namespace default --name BillingPeriodNum --type Int
	docker exec -it temporal-dev temporal operator search-attribute create --namespace default --name BillStatus --type Keyword
	docker exec -it temporal-dev temporal operator search-attribute create --namespace default --name BillCurrency --type Keyword
	docker exec -it temporal-dev temporal operator search-attribute create --namespace default --name BillItemCount --type Int
	docker exec -it temporal-dev temporal operator search-attribute create --namespace default --name BillTotalCents --type Int

init-temporal:
	temporal operator search-attribute create --namespace default --name CustomerID --type Keyword
	temporal operator search-attribute create --namespace default --name CustomerID --type Keyword
	temporal operator search-attribute create --namespace default --name BillingPeriodNum --type Int
	temporal operator search-attribute create --namespace default --name BillStatus --type Keyword
	temporal operator search-attribute create --namespace default --name BillCurrency --type Keyword
	temporal operator search-attribute create --namespace default --name BillItemCount --type Int
	temporal operator search-attribute create --namespace default --name BillTotalCents --type Int

## compile: compiles project in current system
compile: clean mod-download test

clean:
	@echo "  >  Cleaning "
	@rm -rf .encore && go clean ./...

mod-download:
	@echo "  >  Download dependencies..."
	go mod download && go mod tidy

test:
	@echo "  >  Executing tests"
	encore test ./...

run:
	@echo "  >  Running "
	@encore run

lint:
	@echo "  >  Linting"
	golangci-lint run
