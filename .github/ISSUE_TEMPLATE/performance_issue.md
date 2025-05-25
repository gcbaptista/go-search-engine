---
name: ⚡ Performance Issue
about: Report a performance problem or regression
title: "[PERFORMANCE] "
labels: ["performance", "needs-triage"]
assignees: ""
---

## ⚡ Performance Issue Description

<!-- Describe the performance problem -->

## 📊 Performance Metrics

### Current Performance

- **Search Response Time:** <!-- e.g., 500ms average -->
- **Indexing Speed:** <!-- e.g., 100 docs/second -->
- **Memory Usage:** <!-- e.g., 2GB for 1M documents -->
- **CPU Usage:** <!-- e.g., 80% during search -->

### Expected Performance

- **Search Response Time:** <!-- e.g., <100ms -->
- **Indexing Speed:** <!-- e.g., 1000 docs/second -->
- **Memory Usage:** <!-- e.g., <1GB for 1M documents -->
- **CPU Usage:** <!-- e.g., <50% during search -->

## 🔍 Affected Operations

- [ ] Search queries
- [ ] Document indexing
- [ ] Index creation
- [ ] Typo tolerance
- [ ] Filtering
- [ ] Sorting
- [ ] Analytics
- [ ] Startup time

## 📋 Environment & Scale

- **Go Version:** <!-- e.g., 1.21.0 -->
- **OS:** <!-- e.g., macOS 14.0, Ubuntu 22.04 -->
- **Hardware:** <!-- e.g., 8 CPU cores, 16GB RAM, SSD -->
- **Index Size:** <!-- e.g., 1M documents, 5GB -->
- **Concurrent Users:** <!-- e.g., 100 simultaneous searches -->

## 🧪 Reproduction Steps

1.
2.
3.
4.

## 📊 Benchmark Results

<!-- Include benchmark output if available -->

```bash
# Benchmark commands used
go test -bench=. -benchmem ./internal/typoutil/
```

```
# Benchmark results
BenchmarkSearch-8    1000    1500000 ns/op    50000 B/op    100 allocs/op
```

## 📈 Performance Profile

<!-- If you have profiling data, include it -->

```bash
# Profiling commands
go tool pprof cpu.prof
go tool pprof mem.prof
```

## 🔄 Regression Information

<!-- If this is a performance regression -->

- **Last Known Good Version:** <!-- commit hash or version -->
- **First Bad Version:** <!-- commit hash or version -->
- **Suspected Changes:** <!-- link to commits or PRs -->

## 🎯 Performance Goals

<!-- What performance targets should we aim for? -->

- **Target Response Time:** <!-- e.g., <50ms for 95th percentile -->
- **Target Throughput:** <!-- e.g., 1000 QPS -->
- **Target Memory Usage:** <!-- e.g., <500MB per 100k docs -->

## 🔍 Additional Context

<!-- Any other context about the performance issue -->

## 📊 Impact

- [ ] Blocks production deployment
- [ ] Affects user experience
- [ ] Increases infrastructure costs
- [ ] Prevents scaling
- [ ] Minor performance concern
