#!/usr/bin/env bash
# Runs go test and go vet in every module of the go.work workspace.
#
# The repo is a go.work workspace, so there is no root module and `go test ./...`
# at the top level fails. Each module has to be tested from inside its own
# directory.
#
# Usage: ./scripts/test.sh

set -uo pipefail

cd "$(dirname "$0")/.."

MODULES=(
    common
    frontend
    auth-service
    profile-service
    notification-service
    course-catalogue-service
    course-content-service
    assignment-service
)

FAILED=()

for m in "${MODULES[@]}"; do
    echo
    echo "==> $m"
    if ! (cd "$m" && go test ./... && go vet ./...); then
        FAILED+=("$m")
    fi
done

echo
echo "-------------------------------------------"
if [ "${#FAILED[@]}" -eq 0 ]; then
    echo "all ${#MODULES[@]} modules passed"
    exit 0
fi

echo "failed: ${FAILED[*]}"
exit 1
