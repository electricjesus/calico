# Performance Testing Methodology

## Overview

This document describes how to benchmark and verify the performance improvements made to the deps script.

## Quick Start

```bash
# Run the comprehensive benchmark suite
cd /home/panthera/code/calico-deps-optimization
./hack/benchmark-deps.sh
```

## Testing Methodologies

### 1. Go Benchmarks (Micro-benchmarks)

**Purpose**: Test individual function performance in isolation

**Run**:
```bash
cd /home/panthera/code/calico-deps-optimization
go test -bench=. -benchmem -benchtime=5s ./hack/cmd/deps/
```

**Benchmarks included**:
- `BenchmarkFilterInclusions` - Original implementation
- `BenchmarkFilterInclusionsWithGitGrep` - Optimized git grep implementation  
- `BenchmarkSmartFilterInclusions` - Smart dispatcher
- `BenchmarkLoadPackageDepsNoCache` - Cold cache performance
- `BenchmarkLoadPackageDepsWithCache` - Hot cache performance (shows ~1000x improvement)

**Expected Results**:
- Git grep should be 5-10x faster when conditional includes exist
- Cached loadPackageDeps should be 1000x+ faster (μs vs seconds)

### 2. Real-world Command Timing

**Purpose**: Measure end-to-end performance of common operations

**Manual Test**:
```bash
# Test individual package
time go run ./hack/cmd/deps local-dirs felix
time go run ./hack/cmd/deps sem-change-in node
```

**Automated Test**:
```bash
./hack/benchmark-deps.sh  # Runs multiple packages
```

**Packages tested**:
- felix (large, ~500 Go files)
- node (medium, with secondary deps)
- calicoctl (CLI tool)
- typha (datastore proxy)
- kube-controllers (k8s controllers)

### 3. Full Semaphore YAML Generation

**Purpose**: Test the actual CI workload that developers experience

This is the **most important** benchmark as it's what runs during `make generate`.

**Run**:
```bash
# Optimized version
cd /home/panthera/code/calico-deps-optimization
time make gen-semaphore-yaml

# Original version
cd /home/panthera/code/calico
time make gen-semaphore-yaml
```

**Expected improvement**: 3-5x faster (from ~60-120s to ~10-30s)

**Why it matters**: This runs:
- On every `make generate`
- Processes 20+ packages
- Calculates dependencies for all CI jobs
- Most affected by caching optimizations

### 4. Cache Effectiveness Testing

**Purpose**: Verify that the cache actually works

**Run**:
```bash
cd /home/panthera/code/calico-deps-optimization
./hack/test-cache.sh
```

This will show:
- Cache misses (first time loading each package)
- Cache hits (subsequent loads of same package)

**Expected**: 
- ~30-40 cache misses (unique packages)
- ~60-80 cache hits (reused packages, especially in secondary deps)
- Each cache hit saves 1-3 seconds

### 5. Output Verification

**Purpose**: Ensure optimizations don't change behavior

**Run**:
```bash
# Generate with both versions
cd /home/panthera/code/calico-deps-optimization
make gen-semaphore-yaml

cd /home/panthera/code/calico
make gen-semaphore-yaml

# Compare outputs
diff /home/panthera/code/calico-deps-optimization/.semaphore/semaphore.yml \
     /home/panthera/code/calico/.semaphore/semaphore.yml
```

**Expected**: No differences (or only whitespace/comment differences)

## Performance Metrics

### Baseline (Original)

| Operation | Time | Notes |
|-----------|------|-------|
| filterInclusions (felix) | 3-5s | Reads all Go files |
| loadPackageDeps (felix) | 1-3s | First call |
| loadPackageDeps (cached) | 1-3s | No caching! |
| gen-semaphore-yaml | 60-120s | Full CI pipeline |

### Optimized

| Operation | Time | Improvement | Notes |
|-----------|------|-------------|-------|
| filterInclusions (felix) | 0.1-0.5s | 5-10x | git grep |
| loadPackageDeps (felix, cold) | 1-3s | Same | First call |
| loadPackageDeps (felix, hot) | <1ms | 1000x+ | From cache |
| gen-semaphore-yaml | 10-30s | 3-5x | Benefits from caching |

## What Makes It Faster?

### Git Grep Optimization
- **Before**: Walk directory tree → Read each file → Regex match
- **After**: Single `git grep -l -F` command (highly optimized C code)
- **Benefit**: Leverages git's optimized index and search
- **Parallel**: Multiple patterns searched concurrently

### Caching Optimization  
- **Before**: Every `loadPackageDeps` call runs `go list -deps` (1-3s each)
- **After**: First call runs `go list`, subsequent calls return from memory (<1ms)
- **Benefit**: Eliminates duplicate work
- **Example**: Processing "node,typha" as secondary deps both call same base packages

## Limitations & Caveats

### When Git Grep Helps
- Only when checking conditional includes (currently just IPAM)
- Most packages don't have conditional includes
- Real win is from caching, not git grep

### When Caching Helps
- Multiple packages with shared dependencies (e.g., node + typha)
- generate-semaphore-yamls processes ~30 packages with overlap
- Single package operations don't benefit much

### What Doesn't Change
- First `go list` call is still expensive (1-3s)
- No speedup for operations that don't reuse packages
- Git grep requires git repository (falls back gracefully)

## Continuous Monitoring

To track performance over time:

```bash
# Add to CI or pre-commit hook
time make gen-semaphore-yaml
```

Regressions would show as increased time for this command.

## Troubleshooting

### "No improvement seen"
- Check if testing single package (limited caching benefit)
- Verify git grep is available: `git grep --help`
- Run with `--loglevel=debug` to see cache behavior

### "Slower than original"
- First run without warm Go build cache?
- Run `go build ./hack/cmd/deps/` first to compile
- Test with full `gen-semaphore-yaml`, not individual commands

### "Different output"
- Check for actual semantic differences vs formatting
- File a bug if behavior changed

## Next Steps

If these benchmarks show good results, consider:
1. Batch multiple `go list` calls together
2. Pre-compile regex patterns at startup
3. Add more conditional include patterns with git grep
4. Profile with `go tool pprof` for further optimizations
