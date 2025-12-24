#!/bin/bash
# Quick test to demonstrate cache effectiveness within the generate-semaphore-yamls command

echo "=== Cache Effectiveness Test ==="
echo "Running gen-semaphore-yaml with debug logging to show cache hits..."
echo "This processes multiple packages, so we should see cache reuse"
echo

DEFAULT_BRANCH_OVERRIDE=master SEMAPHORE_GIT_BRANCH=master RELEASE_BRANCH_PREFIX=release \
  go run ./hack/cmd/deps --loglevel=debug generate-semaphore-yamls 2>&1 | \
  grep -E "(Using cached|Calculating package deps)" | \
  head -30

echo
echo "=== Summary ==="
echo "Count of 'Calculating' (cache miss):"
DEFAULT_BRANCH_OVERRIDE=master SEMAPHORE_GIT_BRANCH=master RELEASE_BRANCH_PREFIX=release \
  go run ./hack/cmd/deps generate-semaphore-yamls 2>&1 | \
  grep -c "Calculating package deps" || echo "0"

echo "Count of 'Using cached' (cache hit):"
DEFAULT_BRANCH_OVERRIDE=master SEMAPHORE_GIT_BRANCH=master RELEASE_BRANCH_PREFIX=release \
  go run ./hack/cmd/deps --loglevel=debug generate-semaphore-yamls 2>&1 | \
  grep -c "Using cached" || echo "0"
