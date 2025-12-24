#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Deps Script Performance Benchmark${NC}"
echo -e "${BLUE}========================================${NC}"
echo

OPTIMIZED_DIR="/home/panthera/code/calico-deps-optimization"
ORIGINAL_DIR="/home/panthera/code/calico"

# Test 1: Go Benchmarks (Unit level)
echo -e "${GREEN}Test 1: Go Benchmark Tests${NC}"
echo "Running Go benchmarks in optimized version..."
cd "$OPTIMIZED_DIR"
go test -bench=. -benchmem -benchtime=3s ./hack/cmd/deps/ 2>/dev/null || echo "Benchmarks require being in repo root"

echo
echo -e "${YELLOW}Note: filterInclusions benchmarks only show improvement when conditional includes exist${NC}"
echo

# Test 2: Real-world command timing
echo -e "${GREEN}Test 2: Real-world Command Timing${NC}"
echo

PACKAGES=("felix" "node" "calicoctl" "typha" "kube-controllers")

for pkg in "${PACKAGES[@]}"; do
    echo -e "${BLUE}Testing package: $pkg${NC}"
    
    echo "  Optimized version:"
    cd "$OPTIMIZED_DIR"
    time_opt=$( (time go run ./hack/cmd/deps local-dirs "$pkg" > /dev/null 2>&1) 2>&1 | grep real | awk '{print $2}')
    echo "    Time: $time_opt"
    
    echo "  Original version:"
    cd "$ORIGINAL_DIR"
    time_orig=$( (time go run ./hack/cmd/deps local-dirs "$pkg" > /dev/null 2>&1) 2>&1 | grep real | awk '{print $2}')
    echo "    Time: $time_orig"
    echo
done

# Test 3: Full semaphore-yaml generation (most expensive)
echo -e "${GREEN}Test 3: Full Semaphore YAML Generation${NC}"
echo "This is the real-world workload that matters most"
echo

echo "Optimized version:"
cd "$OPTIMIZED_DIR"
echo "  Running: make gen-semaphore-yaml"
time make gen-semaphore-yaml 2>&1 | tail -3

echo
echo "Original version:"
cd "$ORIGINAL_DIR"
echo "  Running: make gen-semaphore-yaml"
time make gen-semaphore-yaml 2>&1 | tail -3

# Test 4: Cache effectiveness test
echo
echo -e "${GREEN}Test 4: Cache Effectiveness${NC}"
echo "Running with debug logging to see cache hits..."
cd "$OPTIMIZED_DIR"
echo "First run (cold cache):"
go run ./hack/cmd/deps --loglevel=debug sem-change-in node,typha 2>&1 | grep -E "(cache|Calculating)" | head -10
echo
echo "Second run (warm cache) - should show cache hits:"
go run ./hack/cmd/deps --loglevel=debug sem-change-in node,typha 2>&1 | grep -E "(cache|Calculating)" | head -10

echo
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Benchmark Complete${NC}"
echo -e "${BLUE}========================================${NC}"
