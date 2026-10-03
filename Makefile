.PHONY: fmt fmt-check test

GO_FILES := $(shell git ls-files '*.go')

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

test:
	go test -count=1 ./...
