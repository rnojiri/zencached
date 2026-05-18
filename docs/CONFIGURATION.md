# Configuration Guide

This guide covers all configuration options for Zencached.

## Table of Contents

- [Basic Configuration](#basic-configuration)
- [Node Configuration](#node-configuration)
- [Connection Settings](#connection-settings)
- [Compression Settings](#compression-settings)
- [Metrics Configuration](#metrics-configuration)
- [Dynamic Node Discovery](#dynamic-node-discovery)
- [Best Practices](#best-practices)

## Basic Configuration

The main configuration structure is `zencached.Configuration`:

```go
type Configuration struct {
    // Static node list
    Nodes []Node
    
    // Connection pool size per node
    NumConnectionsPerNode int
    
    // Buffer size for command execution queue
    CommandExecutionBufferSize uint32
    
    // Node discovery and rebalancing
    NumNodeListRetries int
    RebalanceOnDisconnection bool
    NodeListFunction GetNodeList
    NodeListRetryTimeout time.Duration
    
    // Optional features
    CompressionType CompressionType
    CompressionLevel int
    ZencachedMetricsCollector ZencachedMetricsCollector
}
```

## Node Configuration

### Static Nodes

Define a fixed list of memcached nodes:

```go
config := &zencached.Configuration{
    Nodes: []zencached.Node{
        {Host: "memcached-1.example.com", Port: 11211},
        {Host: "memcached-2.example.com", Port: 11211},
        {Host: "memcached-3.example.com", Port: 11211},
    },
    NumConnectionsPerNode: 5,
}
```

### Node Structure

```go
type Node struct {
    Host string  // Hostname or IP address
    Port int     // Port number (default: 11211)
}
```

## Connection Settings

### NumConnectionsPerNode

Number of connection pool connections per node:

```go
config.NumConnectionsPerNode = 5  // Default reasonable value
```

**Recommendations:**
- Small deployments: 2-5
- Medium deployments: 5-10
- High throughput: 10-20

### CommandExecutionBufferSize

Buffer size for queued commands per node:

```go
config.CommandExecutionBufferSize = 1000  // Default
```

This prevents memory exhaustion under high load. Adjust based on your workload.

### Telnet Configuration

Detailed Telnet protocol settings:

```go
config.TelnetConfiguration = &zencached.TelnetConfiguration{
    // Retry configuration
    ReconnectionTimeout: time.Second,
    
    // Timeout settings
    MaxWriteTimeout:             5 * time.Second,
    MaxReadTimeout:              5 * time.Second,
    HostConnectionTimeout:       5 * time.Second,
    ConnectionCheckTimeout:      3 * time.Second,
    ConnectionCheckIdleWait:     100 * time.Millisecond,
    
    // Buffer settings
    ReadBufferSize: 8192,  // Minimum recommended is 8KB
    
    // Debugging
    EnableTracerLogs: false,
    
    // Metrics (optional)
    TelnetMetricsCollector: telnetMetrics,
}
```

**Timeout Considerations:**
- Set timeouts based on network latency and expected response times
- Lower timeouts: Better responsiveness but may cause false failures
- Higher timeouts: More reliable but slower error detection

## Compression Settings

### Enable Compression

```go
config.CompressionType = zencached.CompressionTypeZSTD
config.CompressionLevel = 3  // 1-22, higher = better compression but slower
```

### Compression Types

- `CompressionTypeNone`: No compression (default)
- `CompressionTypeZSTD`: Zstandard algorithm (recommended)

### Compression Level Guidelines

| Level | Use Case | Speed | Compression |
|-------|----------|-------|-------------|
| 1-3   | High throughput | Very fast | Good |
| 4-8   | Balanced | Fast | Better |
| 9-15  | Storage optimization | Slower | Best |
| 16+   | Archival | Very slow | Maximum |

**When to Use Compression:**
- Large values (> 1KB)
- Limited bandwidth
- Text-based data (highly compressible)

**When NOT to Use:**
- Small values (< 1KB)
- Already compressed data (images, videos, archives)
- CPU-constrained environments

## Metrics Configuration

### Zencached Metrics Collector

Implement custom metrics collection:

```go
type MyMetricsCollector struct{}

func (m *MyMetricsCollector) CommandExecutionElapsedTime(node string, operation, path, key []byte, elapsedTime int64) {
    // Track operation timing (nanoseconds)
}

func (m *MyMetricsCollector) CommandExecution(node string, operation, path, key []byte) {
    // Track operation count
}

func (m *MyMetricsCollector) CommandExecutionError(node string, operation, path, key []byte, err error) {
    // Track errors
}

func (m *MyMetricsCollector) CacheMissEvent(node string, operation, path, key []byte) {
    // Track cache misses
}

func (m *MyMetricsCollector) CacheHitEvent(node string, operation, path, key []byte) {
    // Track cache hits
}

func (m *MyMetricsCollector) NodeRebalanceEvent(numNodes int) {
    // Track rebalance events
}

func (m *MyMetricsCollector) NodeListingEvent(numNodes int) {
    // Track node listing
}

func (m *MyMetricsCollector) NodeListingError() {
    // Track node listing errors
}

func (m *MyMetricsCollector) NodeListingElapsedTime(elapsedTime int64) {
    // Track node listing performance
}

func (m *MyMetricsCollector) NodeRebalanceElapsedTime(elapsedTime int64) {
    // Track rebalance performance
}

config.ZencachedMetricsCollector = &MyMetricsCollector{}
```

### Telnet Metrics Collector

```go
type MyTelnetMetricsCollector struct{}

func (m *MyTelnetMetricsCollector) ResolveAddressElapsedTime(node string, elapsedTime int64) {}
func (m *MyTelnetMetricsCollector) DialElapsedTime(node string, elapsedTime int64) {}
func (m *MyTelnetMetricsCollector) CloseElapsedTime(node string, elapsedTime int64) {}
func (m *MyTelnetMetricsCollector) SendElapsedTime(node string, elapsedTime int64) {}
func (m *MyTelnetMetricsCollector) WriteElapsedTime(node string, elapsedTime int64) {}
func (m *MyTelnetMetricsCollector) ReadElapsedTime(node string, elapsedTime int64) {}
func (m *MyTelnetMetricsCollector) ReadDataSize(node string, sizeInBytes int) {}
func (m *MyTelnetMetricsCollector) WriteDataSize(node string, sizeInBytes int) {}

config.TelnetConfiguration.TelnetMetricsCollector = &MyTelnetMetricsCollector{}
```

## Dynamic Node Discovery

### Using Custom Node Listing Function

For service discovery integration (Consul, Kubernetes, etc.):

```go
config.NodeListFunction = func() ([]zencached.Node, error) {
    // Query service discovery system
    services, err := myServiceDiscovery.GetServices("memcached")
    if err != nil {
        return nil, err
    }
    
    nodes := make([]zencached.Node, len(services))
    for i, svc := range services {
        nodes[i] = zencached.Node{
            Host: svc.Hostname,
            Port: svc.Port,
        }
    }
    return nodes, nil
}

// Configure rebalancing
config.NodeListRetryTimeout = 5 * time.Second
config.NumNodeListRetries = 3
config.RebalanceOnDisconnection = true
```

### Rebalancing

Trigger rebalancing manually:

```go
client.Rebalance()  // Discovers nodes and reconnects
```

Or enable automatic rebalancing on disconnection:

```go
config.RebalanceOnDisconnection = true
```

## Best Practices

### 1. Connection Pool Sizing

```go
// For typical workloads
config.NumConnectionsPerNode = 5

// For high-concurrency workloads
config.NumConnectionsPerNode = 10

// For very high throughput
config.NumConnectionsPerNode = 20
```

### 2. Timeout Configuration

```go
// Aggressive timeouts (LAN environment)
telnetConfig.MaxWriteTimeout = 1 * time.Second
telnetConfig.MaxReadTimeout = 1 * time.Second

// Conservative timeouts (WAN or unstable network)
telnetConfig.MaxWriteTimeout = 10 * time.Second
telnetConfig.MaxReadTimeout = 10 * time.Second
```

### 3. Command Buffer Size

```go
// Balance between memory and throughput
config.CommandExecutionBufferSize = 1000  // 1000 pending commands per node
```

### 4. Context Timeouts

Always use context with timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := client.Get(ctx, nil, path, key)
```

### 5. Error Handling

```go
result, err := client.Get(ctx, nil, path, key)
if err != nil {
    if ze, ok := err.(zencached.ZError); ok {
        // Handle specific error type
        switch ze.Code() {
        case zencached.ErrorTypeNotFound:
            // Handle not found
        case zencached.ErrorTypeKeyInvalid:
            // Handle invalid key
        default:
            // Handle other errors
        }
    }
}
```

### 6. Resource Cleanup

Always defer shutdown:

```go
client, err := zencached.New(config)
if err != nil {
    panic(err)
}
defer client.Shutdown()
```

## Performance Tuning

### For Low Latency

```go
config := &zencached.Configuration{
    NumConnectionsPerNode:      10,
    CommandExecutionBufferSize: 5000,
}

telnetConfig := &zencached.TelnetConfiguration{
    ReadBufferSize:         32768,  // Larger buffer
    MaxWriteTimeout:        500 * time.Millisecond,
    MaxReadTimeout:         500 * time.Millisecond,
}
```

### For High Throughput

```go
config := &zencached.Configuration{
    NumConnectionsPerNode:      20,
    CommandExecutionBufferSize: 10000,
}

telnetConfig := &zencached.TelnetConfiguration{
    ReadBufferSize:         65536,  // Very large buffer
    MaxWriteTimeout:        2 * time.Second,
    MaxReadTimeout:         2 * time.Second,
}
```

### For Limited Resources

```go
config := &zencached.Configuration{
    NumConnectionsPerNode:      2,
    CommandExecutionBufferSize: 100,
    CompressionType:            zencached.CompressionTypeZSTD,
    CompressionLevel:           9,  // Better compression
}

telnetConfig := &zencached.TelnetConfiguration{
    ReadBufferSize: 4096,  // Smaller buffer
}
```
