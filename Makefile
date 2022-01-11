build: pre-commit
	CGO_ENABLED=0 go build -tags netgo -a -ldflags="-w -s" -o ./dist/zammadbridge ./cmd

build-linux: pre-commit
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags netgo -a -ldflags="-w -s" -o ./dist/zammadbridge ./cmd

pre-commit:
	goimports -w .

