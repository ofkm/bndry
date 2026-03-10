set shell := ["zsh", "-cu"]

default:
	@just --list

build:
	mkdir -p bin
	go build -o bin/bndry ./cmd/bndry

test:
	go test -race ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

run:
	go run ./cmd/bndry