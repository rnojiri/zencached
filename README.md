# Zencached - Advanced Memcached Client for Go

A high-performance, feature-rich memcached client library for Go with support for clustering, data compression, metrics collection, and automatic node rebalancing.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Advanced Features](#advanced-features)
- [Metrics Collection](#metrics-collection)
- [Error Handling](#error-handling)
- [Examples](#examples)
- [Testing](#testing)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Memcached Protocol Support**: Full implementation of memcached binary and text protocols via Telnet
- **Cluster Operations**: Perform operations across multiple memcached nodes simultaneously
- **Data Compression**: Optional compression of stored values (supports multiple compression types)
- **Metrics Collection**: Built-in metrics for monitoring performance and operations
- **Node Rebalancing**: Automatic node discovery and rebalancing with custom node listing functions
- **Connection Pooling**: Maintains connection pools per node for optimal performance
- **Error Handling**: Comprehensive error types with detailed context information
- **Context Support**: Full context.Context support for timeout and cancellation control
- **Graceful Shutdown**: Proper connection cleanup on shutdown

## Installation

```bash
go get github.com/rnojiri/zencached
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "github.com/rnojiri/zencached"
)

func main() {
    // Create configuration
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "localhost", Port: 11211},
            {Host: "localhost", Port: 11212},
        },
        NumConnectionsPerNode: 5,
    }

    // Create client
    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    // Set a value
    ctx := context.Background()
    result, err := client.Set(
        ctx,
        nil,                    // routerHash (optional)
        []byte("mypath"),       // path
        []byte("mykey"),        // key
        []byte("myvalue"),      // value
        3600,                   // TTL in seconds
    )
    if err != nil {
        panic(err)
    }

    // Get a value
    result, err = client.Get(ctx, nil, []byte("mypath"), []byte("mykey"))
    if err != nil {
        panic(err)
    }
    println("Value:", string(result.Data))

    // Delete a value
    client.Delete(ctx, nil, []byte("mypath"), []byte("mykey"))
}
```

## Configuration

See [Configuration Guide](./docs/CONFIGURATION.md) for detailed configuration options.

### Basic Configuration Example

```go
config := &zencached.Configuration{
    // Define memcached nodes
    Nodes: []zencached.Node{
        {Host: "memcached1", Port: 11211},
        {Host: "memcached2", Port: 11211},
    },

    // Connection settings
    NumConnectionsPerNode:      5,
    CommandExecutionBufferSize: 1000,

    // Node management
    RebalanceOnDisconnection: true,
    NumNodeListRetries:       3,
    NodeListRetryTimeout:     time.Second * 5,

    // Optional: Custom node listing function
    NodeListFunction: func() ([]zencached.Node, error) {
        // Fetch nodes from service discovery
        return getNodesFromServiceDiscovery()
    },

    // Optional: Compression
    CompressionType:  zencached.CompressionTypeZSTD,
    CompressionLevel: 3,

    // Optional: Metrics collection
    ZencachedMetricsCollector: myMetricsCollector,
}

client, err := zencached.New(config)
```

## API Reference

### Core Operations

#### `Set(ctx context.Context, routerHash, path, key, value []byte, ttl uint64)`
Sets a key-value pair. Creates or overwrites the value.

#### `Add(ctx context.Context, routerHash, path, key, value []byte, ttl uint64)`
Adds a key-value pair only if the key doesn't already exist.

#### `Get(ctx context.Context, routerHash, path, key []byte)`
Retrieves the value associated with a key.

#### `Delete(ctx context.Context, routerHash, path, key []byte)`
Deletes a key from the cache.

### Cluster Operations

#### `ClusterSet(ctx context.Context, path, key, value []byte, ttl uint64)`
Sets a key-value pair on all nodes in the cluster.

#### `ClusterAdd(ctx context.Context, path, key, value []byte, ttl uint64)`
Adds a key-value pair on all nodes in the cluster.

#### `ClusterGet(ctx context.Context, path, key []byte)`
Retrieves a key from all nodes (useful for replicated data).

#### `ClusterDelete(ctx context.Context, path, key []byte)`
Deletes a key from all nodes in the cluster.

### Node Management

#### `Rebalance()`
Triggers a rebalance of all nodes. Discovers nodes using the configured node listing function or uses static node configuration.

#### `GetConnectedNodes() []Node`
Returns the list of currently connected nodes.

#### `Shutdown()`
Gracefully shuts down all connections.

## Advanced Features

### Compression

Enable data compression to reduce network bandwidth and storage space:

```go
config := &zencached.Configuration{
    CompressionType:  zencached.CompressionTypeZSTD,
    CompressionLevel: 3,
    // ... other config
}
```

Supported compression types:
- `CompressionTypeNone` - No compression (default)
- `CompressionTypeZSTD` - Zstandard compression (recommended)

### Custom Node Discovery

Use a custom node listing function for dynamic node discovery:

```go
config.NodeListFunction = func() ([]zencached.Node, error) {
    // Connect to service discovery (Consul, Kubernetes, etc.)
    nodes, err := consul.GetServiceNodes("memcached")
    if err != nil {
        return nil, err
    }
    return nodes, nil
}
```

### Context Usage

All operations support context for timeout and cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := client.Get(ctx, nil, []byte("path"), []byte("key"))
```

## Metrics Collection

Implement the `ZencachedMetricsCollector` interface to collect metrics:

```go
type MyMetricsCollector struct{}

func (m *MyMetricsCollector) CommandExecutionElapsedTime(node string, operation, path, key []byte, elapsedTime int64) {
    // Track command execution time
}

func (m *MyMetricsCollector) CommandExecution(node string, operation, path, key []byte) {
    // Track command execution
}

func (m *MyMetricsCollector) CommandExecutionError(node string, operation, path, key []byte, err error) {
    // Track command errors
}

func (m *MyMetricsCollector) CacheMissEvent(node string, operation, path, key []byte) {
    // Track cache misses
}

func (m *MyMetricsCollector) CacheHitEvent(node string, operation, path, key []byte) {
    // Track cache hits
}

// ... implement other required methods

config.ZencachedMetricsCollector = &MyMetricsCollector{}
```

See [Metrics Reference](./docs/METRICS.md) for all available metrics hooks.

## Error Handling

Zencached provides detailed error information:

```go
result, err := client.Get(ctx, nil, []byte("path"), []byte("key"))
if err != nil {
    if ze, ok := err.(zencached.ZError); ok {
        errorCode := ze.Code()  // Get error type
        errorMsg := ze.String() // Get error message
    }
}
```

See [Error Types](./docs/ERROR_TYPES.md) for detailed error handling.

## Examples

For comprehensive examples, see the [Examples Guide](./docs/EXAMPLES.md):

- Basic operations
- Cluster operations
- Compression
- Metrics collection
- Custom node discovery
- Error handling
- Context usage

## Testing

Run tests:

```bash
go test -v ./...
```

Run benchmarks:

```bash
go test -bench=. ./...
```
 
## Documentation

Complete documentation is available in the [docs](./docs) folder:

- [API Reference](./docs/API.md) - Complete API documentation
- [Configuration Guide](./docs/CONFIGURATION.md) - Setup and configuration
- [Examples](./docs/EXAMPLES.md) - Practical code examples
- [Architecture](./docs/ARCHITECTURE.md) - System design and internals
- [Error Types](./docs/ERROR_TYPES.md) - Error handling reference
- [Metrics](./docs/METRICS.md) - Metrics collection reference
- [Troubleshooting](./docs/TROUBLESHOOTING.md) - Common issues and solutions
- [Contributing](./docs/CONTRIBUTING.md) - Contribution guidelines
- [Documentation Index](./docs/INDEX.md) - Full documentation index

## Contributing

Contributions are welcome! Please review the [Contributing Guide](./docs/CONTRIBUTING.md) for guidelines on code style, testing, and the pull request process.

## License

See the [LICENSE](./LICENSE) file for details.

## Support

For issues, questions, or suggestions, please visit the [project repository](https://github.com/rnojiri/zencached).
