# Metrics Reference

This document describes all available metrics in Zencached.

## Zencached Metrics

The `ZencachedMetricsCollector` interface provides metrics about cache operations.

### Operation Execution Metrics

#### CommandExecutionElapsedTime

```go
func (c *MetricsCollector) CommandExecutionElapsedTime(
    node string,
    operation, path, key []byte,
    elapsedTime int64,
)
```

**Purpose:** Track the execution time of each operation.

**Parameters:**
- `node`: Node hostname/IP where operation was executed
- `operation`: Operation type (e.g., "set", "get", "delete")
- `path`: Key path (or nil)
- `key`: Key name
- `elapsedTime`: Elapsed time in nanoseconds

**Use Cases:**
- Performance monitoring
- Latency tracking
- SLA compliance
- Performance degradation detection

**Example Implementation:**
```go
func (c *MetricsCollector) CommandExecutionElapsedTime(
    node string, operation, path, key []byte, elapsedTime int64) {
    
    opName := fmt.Sprintf("%s_%s", string(operation), string(path))
    latencyMs := float64(elapsedTime) / 1_000_000
    
    c.histogram.WithLabelValues(node, opName).Observe(latencyMs)
}
```

#### CommandExecution

```go
func (c *MetricsCollector) CommandExecution(
    node string,
    operation, path, key []byte,
)
```

**Purpose:** Track successful operation execution count.

**Parameters:**
- `node`: Target node
- `operation`: Operation type
- `path`: Key path
- `key`: Key name

**Use Cases:**
- Operation counting
- Throughput measurement
- Load distribution analysis

**Example Implementation:**
```go
func (c *MetricsCollector) CommandExecution(
    node string, operation, path, key []byte) {
    
    opName := string(operation)
    c.operationCounter.WithLabelValues(node, opName).Inc()
}
```

#### CommandExecutionError

```go
func (c *MetricsCollector) CommandExecutionError(
    node string,
    operation, path, key []byte,
    err error,
)
```

**Purpose:** Track operation errors with error details.

**Parameters:**
- `node`: Target node
- `operation`: Operation type
- `path`: Key path
- `key`: Key name
- `err`: The error that occurred (can be cast to ZError for more details)

**Use Cases:**
- Error rate tracking
- Error categorization
- Alerting on error spikes
- Root cause analysis

**Example Implementation:**
```go
func (c *MetricsCollector) CommandExecutionError(
    node string, operation, path, key []byte, err error) {
    
    errorType := "unknown"
    if ze, ok := err.(zencached.ZError); ok {
        errorType = fmt.Sprintf("%v", ze.Code())
    }
    
    c.errorCounter.WithLabelValues(
        node, string(operation), errorType).Inc()
}
```

### Cache Hit/Miss Metrics

#### CacheHitEvent

```go
func (c *MetricsCollector) CacheHitEvent(
    node string,
    operation, path, key []byte,
)
```

**Purpose:** Record cache hit events (Get operation found the key).

**Parameters:**
- `node`: Node where hit occurred
- `operation`: Operation type (usually "get")
- `path`: Key path
- `key`: Key name

**Use Cases:**
- Hit rate calculation
- Cache effectiveness analysis
- Performance baseline

**Example Implementation:**
```go
func (c *MetricsCollector) CacheHitEvent(
    node string, operation, path, key []byte) {
    
    c.cacheHits.WithLabelValues(node).Inc()
}
```

#### CacheMissEvent

```go
func (c *MetricsCollector) CacheMissEvent(
    node string,
    operation, path, key []byte,
)
```

**Purpose:** Record cache miss events (Get operation didn't find the key).

**Parameters:**
- `node`: Node where miss occurred
- `operation`: Operation type (usually "get")
- `path`: Key path
- `key`: Key name

**Use Cases:**
- Miss rate calculation
- Cache effectiveness analysis
- Cache warming strategy

**Example Implementation:**
```go
func (c *MetricsCollector) CacheMissEvent(
    node string, operation, path, key []byte) {
    
    c.cacheMisses.WithLabelValues(node).Inc()
}
```

### Node Management Metrics

#### NodeRebalanceEvent

```go
func (c *MetricsCollector) NodeRebalanceEvent(numNodes int)
```

**Purpose:** Track node rebalancing events.

**Parameters:**
- `numNodes`: Number of nodes after rebalancing

**Use Cases:**
- Node availability tracking
- Cluster topology changes
- Rebalancing frequency analysis

**Example Implementation:**
```go
func (c *MetricsCollector) NodeRebalanceEvent(numNodes int) {
    c.nodeCount.Set(float64(numNodes))
    c.rebalanceCounter.Inc()
}
```

#### NodeListingEvent

```go
func (c *MetricsCollector) NodeListingEvent(numNodes int)
```

**Purpose:** Track node discovery/listing events.

**Parameters:**
- `numNodes`: Number of nodes discovered

**Use Cases:**
- Node discovery tracking
- Service discovery monitoring

**Example Implementation:**
```go
func (c *MetricsCollector) NodeListingEvent(numNodes int) {
    c.discoveredNodeCount.Set(float64(numNodes))
}
```

#### NodeListingError

```go
func (c *MetricsCollector) NodeListingError()
```

**Purpose:** Track node discovery errors.

**Use Cases:**
- Service discovery reliability
- Error alerting
- Health monitoring

**Example Implementation:**
```go
func (c *MetricsCollector) NodeListingError() {
    c.nodeListingErrorCounter.Inc()
}
```

### Performance Metrics

#### NodeListingElapsedTime

```go
func (c *MetricsCollector) NodeListingElapsedTime(elapsedTime int64)
```

**Purpose:** Track node discovery/listing performance.

**Parameters:**
- `elapsedTime`: Time taken in nanoseconds

**Use Cases:**
- Service discovery latency tracking
- Performance optimization

**Example Implementation:**
```go
func (c *MetricsCollector) NodeListingElapsedTime(elapsedTime int64) {
    discoveryMs := float64(elapsedTime) / 1_000_000
    c.discoveryLatency.Observe(discoveryMs)
}
```

#### NodeRebalanceElapsedTime

```go
func (c *MetricsCollector) NodeRebalanceElapsedTime(elapsedTime int64)
```

**Purpose:** Track rebalancing performance.

**Parameters:**
- `elapsedTime`: Time taken in nanoseconds

**Use Cases:**
- Rebalancing performance monitoring
- Cluster stability analysis

**Example Implementation:**
```go
func (c *MetricsCollector) NodeRebalanceElapsedTime(elapsedTime int64) {
    rebalanceMs := float64(elapsedTime) / 1_000_000
    c.rebalanceLatency.Observe(rebalanceMs)
}
```

## Telnet Metrics

The `TelnetMetricsCollector` interface provides low-level protocol metrics.

### Connection Metrics

#### ResolveAddressElapsedTime

DNS resolution time for node addresses.

```go
func (c *TelnetMetrics) ResolveAddressElapsedTime(
    node string, elapsedTime int64) {
    // Track DNS resolution latency
}
```

#### DialElapsedTime

Time to establish TCP connection.

```go
func (c *TelnetMetrics) DialElapsedTime(
    node string, elapsedTime int64) {
    // Track connection establishment latency
}
```

#### CloseElapsedTime

Time to close TCP connection.

```go
func (c *TelnetMetrics) CloseElapsedTime(
    node string, elapsedTime int64) {
    // Track connection closure latency
}
```

### I/O Metrics

#### SendElapsedTime

Total time for full send operation (including dial and close if needed).

```go
func (c *TelnetMetrics) SendElapsedTime(
    node string, elapsedTime int64) {
    // Track end-to-end send latency
}
```

#### WriteElapsedTime

Time to write data to socket.

```go
func (c *TelnetMetrics) WriteElapsedTime(
    node string, elapsedTime int64) {
    // Track write latency
}
```

#### ReadElapsedTime

Time to read response from socket.

```go
func (c *TelnetMetrics) ReadElapsedTime(
    node string, elapsedTime int64) {
    // Track read latency
}
```

#### WriteDataSize

Size of data written in bytes.

```go
func (c *TelnetMetrics) WriteDataSize(
    node string, sizeInBytes int) {
    // Track bytes written
}
```

#### ReadDataSize

Size of data read in bytes.

```go
func (c *TelnetMetrics) ReadDataSize(
    node string, sizeInBytes int) {
    // Track bytes read
}
```

## Metrics Implementation Example

### Using Prometheus

```go
import "github.com/prometheus/client_golang/prometheus"

type PrometheusMetrics struct {
    operationDuration prometheus.HistogramVec
    operationCount    prometheus.CounterVec
    cacheHits         prometheus.CounterVec
    cacheMisses       prometheus.CounterVec
    errors            prometheus.CounterVec
}

func NewPrometheusMetrics() *PrometheusMetrics {
    return &PrometheusMetrics{
        operationDuration: *prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "zencached_operation_duration_ms",
                Help: "Operation duration in milliseconds",
            },
            []string{"node", "operation"},
        ),
        operationCount: *prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "zencached_operations_total",
                Help: "Total operations",
            },
            []string{"node", "operation"},
        ),
        cacheHits: *prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "zencached_cache_hits_total",
                Help: "Total cache hits",
            },
            []string{"node"},
        ),
        cacheMisses: *prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "zencached_cache_misses_total",
                Help: "Total cache misses",
            },
            []string{"node"},
        ),
        errors: *prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "zencached_errors_total",
                Help: "Total errors",
            },
            []string{"node", "error_type"},
        ),
    }
}

func (pm *PrometheusMetrics) CommandExecutionElapsedTime(
    node string, operation, path, key []byte, elapsedTime int64) {
    pm.operationDuration.
        WithLabelValues(node, string(operation)).
        Observe(float64(elapsedTime) / 1_000_000)
}

func (pm *PrometheusMetrics) CommandExecution(
    node string, operation, path, key []byte) {
    pm.operationCount.
        WithLabelValues(node, string(operation)).
        Inc()
}

func (pm *PrometheusMetrics) CommandExecutionError(
    node string, operation, path, key []byte, err error) {
    errorType := "unknown"
    if ze, ok := err.(zencached.ZError); ok {
        errorType = fmt.Sprintf("%v", ze.Code())
    }
    pm.errors.
        WithLabelValues(node, errorType).
        Inc()
}

// ... implement other methods
```

## Metrics Calculations

### Hit Rate

```
hitRate = (cacheHits / (cacheHits + cacheMisses)) * 100
```

### Error Rate

```
errorRate = (errors / totalOperations) * 100
```

### Average Latency

```
avgLatency = totalLatency / operationCount
```

### Throughput

```
throughput = operationCount / timeWindow
```

## Best Practices

1. **Batch Metrics**: Accumulate and report periodically rather than per-operation
2. **Label Cardinality**: Be careful with high-cardinality labels (path, key)
3. **Performance Impact**: Metrics collection adds overhead; consider sampling
4. **Retention**: Store metrics for appropriate time windows
5. **Alerting**: Set up alerts on error rates and latency anomalies

## Metrics Collection Strategies

### Strategy 1: Async Collection

```go
type AsyncMetrics struct {
    ch chan MetricEvent
}

func (am *AsyncMetrics) CommandExecution(
    node string, operation, path, key []byte) {
    select {
    case am.ch <- MetricEvent{Type: "execution", Node: node}:
    default:
        // Prevent blocking
    }
}
```

### Strategy 2: Sampled Collection

```go
func (m *MetricsCollector) CommandExecution(
    node string, operation, path, key []byte) {
    if rand.Intn(100) < 5 { // 5% sampling
        m.recordMetric(node, string(operation))
    }
}
```

### Strategy 3: Conditional Collection

```go
func (m *MetricsCollector) CommandExecution(
    node string, operation, path, key []byte) {
    
    if !m.metricsEnabled {
        return
    }
    
    if m.isHighPriorityKey(key) {
        m.recordMetric(node, string(operation))
    }
}
```
