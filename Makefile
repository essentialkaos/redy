################################################################################

.DEFAULT_GOAL := help
.PHONY = fmt deps-test test gen-fuzz help

################################################################################

deps-test: ## Download dependencies for tests
	git config --global http.https://pkg.re.followRedirects true
	go get -d -v pkg.re/check.v1

test: ## Run tests
	go test -race .
	go test -covermode=count .

fmt: ## Format source code with gofmt
	find . -name "*.go" -exec gofmt -s -w {} \;

gen-fuzz: ## Generate archives for fuzz testing
	go-fuzz-build -func FuzzInfoParser -o info-parser-fuzz.zip github.com/essentialkaos/redy
	go-fuzz-build -func FuzzConfigParser -o config-parser-fuzz.zip github.com/essentialkaos/redy
	go-fuzz-build -func FuzzRespReader -o resp-reader-fuzz.zip github.com/essentialkaos/redy

help: ## Show this info
	@echo -e '\nSupported targets:\n'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[33m%-12s\033[0m %s\n", $$1, $$2}'
	@echo -e ''

################################################################################
