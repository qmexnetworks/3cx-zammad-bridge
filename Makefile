build: pre-commit
	CGO_ENABLED=0 go build -tags netgo -a -ldflags="-w -s" -o ./dist/zammadbridge ./cmd

pre-commit:
	goimports -w .

