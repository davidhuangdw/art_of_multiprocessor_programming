GO   ?= go
PKGS ?= ./...
DIR  ?= ./...
CPU  := 1,2,4,8

.PHONY: all fmt vet tidy test race stress bench cover clean list

all: fmt vet race

fmt:
	$(GO) fmt $(PKGS)

vet:
	$(GO) vet $(PKGS)

tidy:
	$(GO) mod tidy

test:
	$(GO) test $(PKGS) -count=1

# Run every package with the race detector
race:
	$(GO) test $(PKGS) -race -count=1

# Repeated stress run for one package: make stress DIR=./ch07_spin
stress:
	$(GO) test $(DIR) -race -count=100

# Benchmarks: make bench DIR=./ch07_spin
bench:
	$(GO) test $(DIR) -run '^$$' -bench . -benchmem -cpu=$(CPU)

cover:
	$(GO) test $(PKGS) -race -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

list:
	$(GO) list $(PKGS)

clean:
	rm -f coverage.out coverage.html *.prof *.test
