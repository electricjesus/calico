package main

import (
"testing"

"github.com/projectcalico/calico/libcalico-go/lib/set"
)

// Benchmark the original filterInclusions implementation
func BenchmarkFilterInclusions(b *testing.B) {
inclusions := set.New[string]()
inclusions.Add("/libcalico-go/lib/ipam")
inclusions.Add("/api/pkg/apis/projectcalico/v3")
inclusions.Add("/felix/bpf")

b.ResetTimer()
for i := 0; i < b.N; i++ {
_ = filterInclusions("felix", inclusions)
}
}

// Benchmark the git grep implementation
func BenchmarkFilterInclusionsWithGitGrep(b *testing.B) {
if !checkGitGrepAvailable() {
b.Skip("git grep not available")
}

inclusions := set.New[string]()
inclusions.Add("/libcalico-go/lib/ipam")
inclusions.Add("/api/pkg/apis/projectcalico/v3")
inclusions.Add("/felix/bpf")

b.ResetTimer()
for i := 0; i < b.N; i++ {
_ = filterInclusionsWithGitGrep("felix", inclusions)
}
}

// Benchmark the smart dispatcher
func BenchmarkSmartFilterInclusions(b *testing.B) {
inclusions := set.New[string]()
inclusions.Add("/libcalico-go/lib/ipam")
inclusions.Add("/api/pkg/apis/projectcalico/v3")
inclusions.Add("/felix/bpf")

b.ResetTimer()
for i := 0; i < b.N; i++ {
_ = smartFilterInclusions("felix", inclusions)
}
}

// Benchmark loadPackageDeps without cache (cold)
func BenchmarkLoadPackageDepsNoCacheFelix(b *testing.B) {
for i := 0; i < b.N; i++ {
// Clear cache between runs
packageDepsMutex.Lock()
packageDepsCache = make(map[string][]string)
packageDepsMutex.Unlock()

_, err := loadPackageDeps("felix", false)
if err != nil {
b.Fatal(err)
}
}
}

// Benchmark loadPackageDeps with cache (hot)
func BenchmarkLoadPackageDepsWithCacheFelix(b *testing.B) {
// Prime the cache
_, err := loadPackageDeps("felix", false)
if err != nil {
b.Fatal(err)
}

b.ResetTimer()
for i := 0; i < b.N; i++ {
_, err := loadPackageDeps("felix", false)
if err != nil {
b.Fatal(err)
}
}
}
