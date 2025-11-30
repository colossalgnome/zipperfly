# Prometheus Metrics

This document describes all Prometheus metrics exposed by the egress server on the `/metrics` endpoint.

## Custom Zipperfly Metrics

### HTTP Request Metrics

#### `zipperfly_requests_total`
**Type:** Counter  
**Labels:** `status` (HTTP status code)  
**Description:** Total number of HTTP requests by status code.

Example labels:
- `status="200"` - Successful requests
- `status="400"` - Bad requests
- `status="401"` - Unauthorized
- `status="404"` - Not found
- `status="410"` - Expired

**Example queries:**
```promql
# Request rate by status  
rate(zipperfly_requests_total[5m])  

# Total 4xx errors  
sum(rate(zipperfly_requests_total{status=~"4.."}[5m]))  
```

### Download Outcome Metrics

#### `zipperfly_downloads_total`
**Type:** Counter  
**Labels:** `status` (download outcome)  
**Description:** Total number of download attempts by outcome.

Labels:
- `status="completed"` - All requested files successfully fetched and zipped
- `status="partial"` - Some files missing but download succeeded (requires `IGNORE_MISSING=true`)
- `status="failed"` - Download failed due to errors

**Example queries:**
```promql
# Download success rate  
rate(zipperfly_downloads_total{status="completed"}[5m]) / rate(zipperfly_downloads_total[5m])  

# Partial download rate  
rate(zipperfly_downloads_total{status="partial"}[5m])  

# Failed downloads  
rate(zipperfly_downloads_total{status="failed"}[5m])  
```

### File-Level Metrics

#### `zipperfly_files_requested`
**Type:** Histogram  
**Description:** Distribution of the number of files requested per download.

Buckets: 1, 5, 10, 20, 50, 100, 200, 500, 1000, 5000

**Example queries:**
```promql
# Average files per download  
rate(zipperfly_files_requested_sum[5m]) / rate(zipperfly_files_requested_count[5m])  

# 95th percentile of files per download  
histogram_quantile(0.95, rate(zipperfly_files_requested_bucket[5m]))  
```

#### `zipperfly_files_success`
**Type:** Histogram  
**Description:** Distribution of the number of files successfully fetched per download.

Buckets: 1, 5, 10, 20, 50, 100, 200, 500, 1000, 5000

**Example queries:**
```promql
# Average successful files per download  
rate(zipperfly_files_success_sum[5m]) / rate(zipperfly_files_success_count[5m])  

# Compare requested vs successful  
rate(zipperfly_files_requested_sum[5m]) - rate(zipperfly_files_success_sum[5m])  
```

#### `zipperfly_files_fetch_total`
**Type:** Counter  
**Labels:** `result` (fetch result)  
**Description:** Total file fetch attempts by result.

Labels:
- `result="success"` - File successfully fetched
- `result="missing"` - File not found (counted when `IGNORE_MISSING=true`)
- `result="error"` - Fetch failed due to error

**Example queries:**
```promql
# File fetch success rate  
rate(zipperfly_files_fetch_total{result="success"}[5m]) / rate(zipperfly_files_fetch_total[5m])  

# Missing file rate  
rate(zipperfly_files_fetch_total{result="missing"}[5m])  

# Error rate per file  
rate(zipperfly_files_fetch_total{result="error"}[5m])  
```

#### `zipperfly_missing_files_total`
**Type:** Counter  
**Description:** Total count of missing files encountered across all downloads.

**Example queries:**
```promql
# Rate of missing files  
rate(zipperfly_missing_files_total[5m])  

# Total missing files today  
increase(zipperfly_missing_files_total[24h])  
```

### Performance Metrics

#### `zipperfly_request_duration_seconds`
**Type:** Histogram  
**Description:** Request duration in seconds (from request start to completion).

Buckets: 1, 5, 10, 30, 60, 120, 300, 600, 1200, 1800

**Example queries:**
```promql
# 95th percentile latency  
histogram_quantile(0.95, rate(zipperfly_request_duration_seconds_bucket[5m]))  

# Average request duration  
rate(zipperfly_request_duration_seconds_sum[5m]) / rate(zipperfly_request_duration_seconds_count[5m])  
```

#### `zipperfly_outgoing_bytes`
**Type:** Histogram  
**Description:** Outgoing bytes per response (compressed ZIP size).

Buckets: 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576, 2097152, 4194304, 8388608, 16777216, 33554432, 67108864, 134217728, 268435456, 536870912, 1073741824, 2147483648, 4294967296, 8589934592, 17179869184, 34359738368, 68719476736, 137438953472, 274877906944, 549755813888, 1099511627776, 2199023255552, 4398046511104, 8796093022208, 17592186044416

**Example queries:**
```promql
# Average ZIP size  
rate(zipperfly_outgoing_bytes_sum[5m]) / rate(zipperfly_outgoing_bytes_count[5m])  

# Total bandwidth out  
rate(zipperfly_outgoing_bytes_sum[5m])  

# 95th percentile ZIP size  
histogram_quantile(0.95, rate(zipperfly_outgoing_bytes_bucket[5m]))  
```

#### `zipperfly_incoming_bytes`
**Type:** Histogram  
**Description:** Incoming bytes from storage per request (uncompressed size of all files).

Buckets: 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576, 2097152, 4194304, 8388608, 16777216, 33554432, 67108864, 134217728, 268435456, 536870912, 1073741824, 2147483648, 4294967296, 8589934592, 17179869184, 34359738368, 68719476736, 137438953472, 274877906944, 549755813888, 1099511627776, 2199023255552, 4398046511104, 8796093022208, 17592186044416

**Example queries:**
```promql
# Compression ratio  
rate(zipperfly_outgoing_bytes_sum[5m]) / rate(zipperfly_incoming_bytes_sum[5m])  

# Total storage bandwidth  
rate(zipperfly_incoming_bytes_sum[5m])  
```

#### `zipperfly_compression_ratio`
**Type:** Histogram  
**Description:** Compression ratio (compressed/uncompressed).

Buckets: 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1

**Example queries:**
```promql
# Average compression ratio  
rate(zipperfly_compression_ratio_sum[5m]) / rate(zipperfly_compression_ratio_count[5m])  

# 95th percentile compression ratio  
histogram_quantile(0.95, rate(zipperfly_compression_ratio_bucket[5m]))  
```

### System Metrics

#### `zipperfly_memory_heap_alloc_bytes`
**Type:** Gauge  
**Description:** Current heap allocation in bytes (updated every 10 seconds).

**Example queries:**
```promql
# Current memory usage  
zipperfly_memory_heap_alloc_bytes  

# Memory usage over time  
zipperfly_memory_heap_alloc_bytes[1h]  
```

#### `zipperfly_goroutines`
**Type:** Gauge  
**Description:** Number of goroutines (updated every 10 seconds).

**Example queries:**
```promql
# Current goroutine count  
zipperfly_goroutines  

# Goroutine growth  
rate(zipperfly_goroutines[5m])  
```

### Active Metrics

#### `zipperfly_active_downloads`
**Type:** Gauge  
**Description:** Number of currently active downloads.

**Example queries:**
```promql
# Current active downloads  
zipperfly_active_downloads  

# Average active downloads over time  
avg_over_time(zipperfly_active_downloads[5m])  
```

#### `zipperfly_active_file_fetches`
**Type:** Gauge  
**Description:** Number of currently active file fetches.

**Example queries:**
```promql
# Current active file fetches  
zipperfly_active_file_fetches  

# Average active file fetches over time  
avg_over_time(zipperfly_active_file_fetches[5m])  
```

### Callback Metrics

#### `zipperfly_callback_retries_total`
**Type:** Counter  
**Description:** Total number of callback retry attempts.

**Example queries:**
```promql
# Rate of callback retries  
rate(zipperfly_callback_retries_total[5m])  

# Total callback retries today  
increase(zipperfly_callback_retries_total[24h])  
```

### Client Metrics

#### `zipperfly_client_disconnects_total`
**Type:** Counter  
**Description:** Total number of client disconnects during download.

**Example queries:**
```promql
# Rate of client disconnects  
rate(zipperfly_client_disconnects_total[5m])  

# Total client disconnects today  
increase(zipperfly_client_disconnects_total[24h])  
```

### Database Metrics

#### `zipperfly_database_query_duration_seconds`
**Type:** Histogram  
**Labels:** `db_type` (e.g., "postgres")  
**Description:** Database query duration in seconds.

Buckets: 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5

**Example queries:**
```promql
# 95th percentile query latency by db_type  
histogram_quantile(0.95, sum(rate(zipperfly_database_query_duration_seconds_bucket[5m])) by (le, db_type))  

# Average query duration  
sum(rate(zipperfly_database_query_duration_seconds_sum[5m])) by (db_type) / sum(rate(zipperfly_database_query_duration_seconds_count[5m])) by (db_type)  
```

### Request Validation Metrics

#### `zipperfly_expired_requests_total`
**Type:** Counter  
**Description:** Total number of requests with expired timestamps.

**Example queries:**
```promql
# Rate of expired requests  
rate(zipperfly_expired_requests_total[5m])  

# Total expired requests today  
increase(zipperfly_expired_requests_total[24h])  
```

#### `zipperfly_signature_failures_total`
**Type:** Counter  
**Description:** Total number of failed signature verifications.

**Example queries:**
```promql
# Rate of signature failures  
rate(zipperfly_signature_failures_total[5m])  

# Total signature failures today  
increase(zipperfly_signature_failures_total[24h])  
```

### Health Metrics

#### `zipperfly_health_status`
**Type:** Gauge  
**Labels:** `component` (e.g., "database", "storage")  
**Description:** Health status by component (1=healthy, 0=unhealthy).

**Example queries:**
```promql
# Current health status by component  
zipperfly_health_status  

# Number of unhealthy components  
count(zipperfly_health_status == 0)  
```

#### `zipperfly_health_checks_failed_total`
**Type:** Counter  
**Labels:** `component` (e.g., "storage")  
**Description:** Total number of failed health checks by component.

**Example queries:**
```promql
# Rate of failed health checks by component  
rate(zipperfly_health_checks_failed_total[5m])  

# Total failed health checks today by component  
increase(zipperfly_health_checks_failed_total[24h])  
```

## Go Runtime Metrics

These are standard metrics from the Go runtime.

#### `go_gc_duration_seconds`
**Type:** Summary  
**Labels:** `quantile`  
**Description:** A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.

#### `go_gc_gogc_percent`
**Type:** Gauge  
**Description:** Heap size target percentage configured by the user, otherwise 100. This value is set by the GOGC environment variable, and the runtime/debug.SetGCPercent function. Sourced from /gc/gogc:percent.

#### `go_gc_gomemlimit_bytes`
**Type:** Gauge  
**Description:** Go runtime memory limit configured by the user, otherwise math.MaxInt64. This value is set by the GOMEMLIMIT environment variable, and the runtime/debug.SetMemoryLimit function. Sourced from /gc/gomemlimit:bytes.

#### `go_goroutines`
**Type:** Gauge  
**Description:** Number of goroutines that currently exist.

#### `go_info`
**Type:** Gauge  
**Labels:** `version`  
**Description:** Information about the Go environment.

#### `go_memstats_alloc_bytes`
**Type:** Gauge  
**Description:** Number of bytes allocated in heap and currently in use. Equals to /memory/classes/heap/objects:bytes.

#### `go_memstats_alloc_bytes_total`
**Type:** Counter  
**Description:** Total number of bytes allocated in heap until now, even if released already. Equals to /gc/heap/allocs:bytes.

#### `go_memstats_buck_hash_sys_bytes`
**Type:** Gauge  
**Description:** Number of bytes used by the profiling bucket hash table. Equals to /memory/classes/profiling/buckets:bytes.

#### `go_memstats_frees_total`
**Type:** Counter  
**Description:** Total number of heap objects frees. Equals to /gc/heap/frees:objects + /gc/heap/tiny/allocs:objects.

#### `go_memstats_gc_sys_bytes`
**Type:** Gauge  
**Description:** Number of bytes used for garbage collection system metadata. Equals to /memory/classes/metadata/other:bytes.

#### `go_memstats_heap_alloc_bytes`
**Type:** Gauge  
**Description:** Number of heap bytes allocated and currently in use, same as go_memstats_alloc_bytes. Equals to /memory/classes/heap/objects:bytes.

#### `go_memstats_heap_idle_bytes`
**Type:** Gauge  
**Description:** Number of heap bytes waiting to be used. Equals to /memory/classes/heap/released:bytes + /memory/classes/heap/free:bytes.

#### `go_memstats_heap_inuse_bytes`
**Type:** Gauge  
**Description:** Number of heap bytes that are in use. Equals to /memory/classes/heap/objects:bytes + /memory/classes/heap/unused:bytes

#### `go_memstats_heap_objects`
**Type:** Gauge  
**Description:** Number of currently allocated objects. Equals to /gc/heap/objects:objects.

#### `go_memstats_heap_released_bytes`
**Type:** Gauge  
**Description:** Number of heap bytes released to OS. Equals to /memory/classes/heap/released:bytes.

#### `go_memstats_heap_sys_bytes`
**Type:** Gauge  
**Description:** Number of heap bytes obtained from system. Equals to /memory/classes/heap/objects:bytes + /memory/classes/heap/unused:bytes + /memory/classes/heap/released:bytes + /memory/classes/heap/free:bytes.

#### `go_memstats_last_gc_time_seconds`
**Type:** Gauge  
**Description:** Number of seconds since 1970 of last garbage collection.

#### `go_memstats_mallocs_total`
**Type:** Counter  
**Description:** Total number of heap objects allocated, both live and gc-ed. Semantically a counter version for go_memstats_heap_objects gauge. Equals to /gc/heap/allocs:objects + /gc/heap/tiny/allocs:objects.

#### `go_memstats_mcache_inuse_bytes`
**Type:** Gauge  
**Description:** Number of bytes in use by mcache structures. Equals to /memory/classes/metadata/mcache/inuse:bytes.

#### `go_memstats_mcache_sys_bytes`
**Type:** Gauge  
**Description:** Number of bytes used for mcache structures obtained from system. Equals to /memory/classes/metadata/mcache/inuse:bytes + /memory/classes/metadata/mcache/free:bytes.

#### `go_memstats_mspan_inuse_bytes`
**Type:** Gauge  
**Description:** Number of bytes in use by mspan structures. Equals to /memory/classes/metadata/mspan/inuse:bytes.

#### `go_memstats_mspan_sys_bytes`
**Type:** Gauge  
**Description:** Number of bytes used for mspan structures obtained from system. Equals to /memory/classes/metadata/mspan/inuse:bytes + /memory/classes/metadata/mspan/free:bytes.

#### `go_memstats_next_gc_bytes`
**Type:** Gauge  
**Description:** Number of heap bytes when next garbage collection will take place. Equals to /gc/heap/goal:bytes.

#### `go_memstats_other_sys_bytes`
**Type:** Gauge  
**Description:** Number of bytes used for other system allocations. Equals to /memory/classes/other:bytes.

#### `go_memstats_stack_inuse_bytes`
**Type:** Gauge  
**Description:** Number of bytes obtained from system for stack allocator in non-CGO environments. Equals to /memory/classes/heap/stacks:bytes.

#### `go_memstats_stack_sys_bytes`
**Type:** Gauge  
**Description:** Number of bytes obtained from system for stack allocator. Equals to /memory/classes/heap/stacks:bytes + /memory/classes/os-stacks:bytes.

#### `go_memstats_sys_bytes`
**Type:** Gauge  
**Description:** Number of bytes obtained from system. Equals to /memory/classes/total:byte.

#### `go_sched_gomaxprocs_threads`
**Type:** Gauge  
**Description:** The current runtime.GOMAXPROCS setting, or the number of operating system threads that can execute user-level Go code simultaneously. Sourced from /sched/gomaxprocs:threads.

#### `go_threads`
**Type:** Gauge  
**Description:** Number of OS threads created.

## Process Metrics

These are standard process metrics.

#### `process_cpu_seconds_total`
**Type:** Counter  
**Description:** Total user and system CPU time spent in seconds.

#### `process_max_fds`
**Type:** Gauge  
**Description:** Maximum number of open file descriptors.

#### `process_open_fds`
**Type:** Gauge  
**Description:** Number of open file descriptors.

#### `process_resident_memory_bytes`
**Type:** Gauge  
**Description:** Resident memory size in bytes.

#### `process_start_time_seconds`
**Type:** Gauge  
**Description:** Start time of the process since unix epoch in seconds.

#### `process_virtual_memory_bytes`
**Type:** Gauge  
**Description:** Virtual memory size in bytes.

#### `process_virtual_memory_max_bytes`
**Type:** Gauge  
**Description:** Maximum amount of virtual memory available in bytes.

## Promhttp Metrics

These are metrics from the Prometheus HTTP handler.

#### `promhttp_metric_handler_requests_in_flight`
**Type:** Gauge  
**Description:** Current number of scrapes being served.

#### `promhttp_metric_handler_requests_total`
**Type:** Counter  
**Labels:** `code` (HTTP status code)  
**Description:** Total number of scrapes by HTTP status code.

## Example Dashboards

### Success Rate Panel
```promql
# Overall download success rate
sum(rate(zipperfly_downloads_total{status="completed"}[5m])) / sum(rate(zipperfly_downloads_total[5m])) * 100
```

### Missing Files Alert
```promql
# Alert if missing file rate exceeds threshold
rate(zipperfly_missing_files_total[5m]) > 0.1
```

### Performance Overview
```promql
# P50, P95, P99 latencies
histogram_quantile(0.50, rate(zipperfly_request_duration_seconds_bucket[5m]))
histogram_quantile(0.95, rate(zipperfly_request_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(zipperfly_request_duration_seconds_bucket[5m]))
```

### Bandwidth Panel
```promql
# Outgoing bandwidth (MB/s)
rate(zipperfly_outgoing_bytes_sum[5m]) / 1024 / 1024

# Incoming bandwidth from storage (MB/s)
rate(zipperfly_incoming_bytes_sum[5m]) / 1024 / 1024
```

### Health Overview
```promql
# Unhealthy components count
count(zipperfly_health_status == 0)

# Failed health checks rate
sum(rate(zipperfly_health_checks_failed_total[5m]))
```

### Compression Overview
```promql
# Average compression ratio
rate(zipperfly_compression_ratio_sum[5m]) / rate(zipperfly_compression_ratio_count[5m])
```

### Validation Failures
```promql
# Rate of expired requests and signature failures
rate(zipperfly_expired_requests_total[5m]) + rate(zipperfly_signature_failures_total[5m])
```

## Monitoring Best Practices

1. **Alert on failed downloads:** Set alerts when `zipperfly_downloads_total{status="failed"}` exceeds threshold
2. **Monitor missing files:** Track `zipperfly_missing_files_total` to detect storage issues
3. **Watch partial downloads:** High rates of `status="partial"` may indicate data integrity issues
4. **Track latency:** Set SLOs on p95/p99 of `zipperfly_request_duration_seconds`
5. **Memory monitoring:** Alert on sustained high `zipperfly_memory_heap_alloc_bytes`
6. **File fetch errors:** Monitor `zipperfly_files_fetch_total{result="error"}` for storage backend issues
7. **Health monitoring:** Alert when `zipperfly_health_status == 0` for any component
8. **Database performance:** Set alerts on high p95 of `zipperfly_database_query_duration_seconds`
9. **Validation errors:** Monitor rates of `zipperfly_expired_requests_total` and `zipperfly_signature_failures_total` for security issues
10. **Active operations:** Alert on high `zipperfly_active_downloads` or `zipperfly_active_file_fetches` indicating overload
11. **Callback retries:** Track `zipperfly_callback_retries_total` for integration issues
12. **Client disconnects:** High `zipperfly_client_disconnects_total` may indicate network problems