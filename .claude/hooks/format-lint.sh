#!/bin/sh
# Claude Code PostToolUse hook: format the edited Go file, then block on lint
# findings. Exit code 2 feeds stderr back to the agent.
f=$(jq -r '.tool_input.file_path // .tool_response.filePath // empty')
case "$f" in
*.go) ;;
*) exit 0 ;;
esac

cd "${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}" || exit 0

if ! command -v golangci-lint >/dev/null 2>&1; then
	echo "format-lint: golangci-lint not found on PATH; install v2: https://golangci-lint.run/docs/welcome/install/" >&2
	exit 2
fi

if ! out=$(golangci-lint fmt "$f" 2>&1); then
	echo "$out" >&2
	exit 2
fi

if ! out=$(golangci-lint run ./... 2>&1); then
	echo "$out" >&2
	exit 2
fi
