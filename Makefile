build: pre-commit
	CGO_ENABLED=0 go build -tags netgo -a -ldflags="-w -s" -o ./dist/zammadbridge ./cmd

build-linux: pre-commit
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags netgo -a -ldflags="-w -s" -o ./dist/zammadbridge ./cmd

# Formatting (gofumpt, goimports) is part of the golangci-lint config.
pre-commit:
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...
