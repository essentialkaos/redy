################################################################################

# This Makefile generated by GoMakeGen 0.8.0 using next command:
# gomakegen --race .

################################################################################

.DEFAULT_GOAL := help
.PHONY = fmt deps-test test help

################################################################################

deps-test: ## Download dependencies for tests
	git config --global http.https://pkg.re.followRedirects true
	go get -d -v pkg.re/check.v1

test: ## Run tests
	go test -race .
	go test -covermode=count .

fmt: ## Format source code with gofmt
	find . -name "*.go" -exec gofmt -s -w {} \;

help: ## Show this info
	@echo -e '\nSupported targets:\n'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[33m%-12s\033[0m %s\n", $$1, $$2}'
	@echo -e ''

################################################################################
