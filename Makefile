.PHONY: all build test generate clean fmt vet

BINARY  := opal
CMD     := ./cmd/opal
PKG_LEX := ./pkg/lex

all: generate build

# generate the asm stubs (datum.s + datum.go) from asm.go
generate:
	@echo "==> go generate $(PKG_LEX)"
	cd $(PKG_LEX) && go generate .

# build the opal binary
build:
	@echo "==> go build $(CMD)"
	go build -o $(BINARY) $(CMD)

# run all tests
test:
	@echo "==> go test ./..."
	go test -v -race ./...

# format all source
fmt:
	@echo "==> gofmt"
	gofmt -w -s .

# vet all packages
vet:
	@echo "==> go vet"
	go vet ./...

# remove build artifacts
clean:
	@echo "==> clean"
	rm -f $(BINARY)
