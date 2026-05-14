# Code Examples

This document contains practical examples of using Zencached.

## Table of Contents

- [Basic Operations](#basic-operations)
- [Cluster Operations](#cluster-operations)
- [Compression](#compression)
- [Error Handling](#error-handling)
- [Metrics Collection](#metrics-collection)
- [Dynamic Node Discovery](#dynamic-node-discovery)
- [Context Usage](#context-usage)

## Basic Operations

### Simple Key-Value Storage

```go
package main

import (
    "context"
    "fmt"
    "github.com/rnojiri/zencached"
)

func main() {
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "localhost", Port: 11211},
        },
        NumConnectionsPerNode: 5,
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    ctx := context.Background()

    // Set a value
    result, err := client.Set(
        ctx,
        nil,
        []byte(""),
        []byte("mykey"),
        []byte("myvalue"),
        3600,
    )
    if err != nil {
        fmt.Printf("Set failed: %v\n", err)
        return
    }
    fmt.Printf("Set: %v\n", result.Type)

    // Get the value
    result, err = client.Get(ctx, nil, []byte(""), []byte("mykey"))
    if err != nil {
        fmt.Printf("Get failed: %v\n", err)
        return
    }
    fmt.Printf("Got: %s\n", string(result.Data))

    // Delete the value
    result, err = client.Delete(ctx, nil, []byte(""), []byte("mykey"))
    if err != nil {
        fmt.Printf("Delete failed: %v\n", err)
        return
    }
    fmt.Printf("Delete: %v\n", result.Type)
}
```

### Using Paths

```go
package main

import (
    "context"
    "fmt"
    "github.com/rnojiri/zencached"
)

func main() {
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "localhost", Port: 11211},
        },
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    ctx := context.Background()

    // Store user data with path prefix
    path := []byte("users")
    key := []byte("user123")
    value := []byte(`{"name":"John","email":"john@example.com"}`)

    result, err := client.Set(ctx, nil, path, key, value, 3600)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Stored user: %v\n", result.Type)

    // Retrieve user data
    result, err = client.Get(ctx, nil, path, key)
    if err != nil {
        panic(err)
    }
    fmt.Printf("User data: %s\n", string(result.Data))
}
```

## Cluster Operations

### Replicate Data Across Cluster

```go
package main

import (
    "context"
    "fmt"
    "github.com/rnojiri/zencached"
)

func main() {
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "memcached1", Port: 11211},
            {Host: "memcached2", Port: 11211},
            {Host: "memcached3", Port: 11211},
        },
        NumConnectionsPerNode: 5,
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    ctx := context.Background()

    // Set value on all nodes
    results, errs := client.ClusterSet(
        ctx,
        []byte("config"),
        []byte("app_settings"),
        []byte(`{"version":"1.0","debug":false}`),
        86400,
    )

    // Check results
    successCount := 0
    for i, result := range results {
        if errs[i] != nil {
            fmt.Printf("Node %d: Error - %v\n", i, errs[i])
        } else {
            fmt.Printf("Node %d: Success - %v\n", i, result.Type)
            successCount++
        }
    }
    fmt.Printf("Successfully replicated to %d/%d nodes\n", successCount, len(results))
}
```

### Read from All Nodes

```go
package main

import (
    "context"
    "fmt"
    "github.com/rnojiri/zencached"
)

func main() {
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "memcached1", Port: 11211},
            {Host: "memcached2", Port: 11211},
        },
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    ctx := context.Background()

    // Get from all nodes
    results, errs := client.ClusterGet(ctx, []byte("data"), []byte("key1"))

    // Verify consistency
    var data []byte
    inconsistent := false

    for i, result := range results {
        if errs[i] != nil {
            fmt.Printf("Node %d: Error - %v\n", i, errs[i])
            continue
        }

        if data == nil {
            data = result.Data
        } else if string(data) != string(result.Data) {
            inconsistent = true
            fmt.Printf("Node %d: Inconsistent data\n", i)
        } else {
            fmt.Printf("Node %d: Consistent\n", i)
        }
    }

    if inconsistent {
        fmt.Println("WARNING: Cluster data is inconsistent!")
    }
}
```

## Compression

### Enable Compression

```go
package main

import (
    "context"
    "fmt"
    "github.com/rnojiri/zencached"
)

func main() {
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "localhost", Port: 11211},
        },
        NumConnectionsPerNode: 5,
        CompressionType:       zencached.CompressionTypeZSTD,
        CompressionLevel:      3,
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    ctx := context.Background()

    // Large value that will be compressed
    largeValue := []byte(`[
        {"id":1,"name":"Item 1","description":"A very long description..."},
        {"id":2,"name":"Item 2","description":"Another long description..."},
        ...
    ]`)

    result, err := client.Set(
        ctx,
        nil,
        []byte("lists"),
        []byte("items"),
        largeValue,
        3600,
    )
    if err != nil {
        panic(err)
    }
    fmt.Printf("Stored (compressed): %v\n", result.Type)

    // Get will automatically decompress
    result, err = client.Get(ctx, nil, []byte("lists"), []byte("items"))
    if err != nil {
        panic(err)
    }
    fmt.Printf("Retrieved (decompressed): %s\n", string(result.Data[:50]))
}
```

## Error Handling

### Detailed Error Handling

```go
package main

import (
    "context"
    "fmt"
    "github.com/rnojiri/zencached"
)

func main() {
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "localhost", Port: 11211},
        },
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    ctx := context.Background()

    // Try to get a key
    result, err := client.Get(ctx, nil, []byte(""), []byte("nonexistent"))
    if err != nil {
        // Handle Zencached errors
        if ze, ok := err.(zencached.ZError); ok {
            switch ze.Code() {
            case zencached.ErrorTypeNotFound:
                fmt.Println("Key not found in cache")
            case zencached.ErrorTypeKeyInvalid:
                fmt.Println("Invalid key format")
            case zencached.ErrorTypeConnectionFailed:
                fmt.Println("Could not connect to memcached")
            default:
                fmt.Printf("Other error: %s\n", ze.String())
            }
        } else {
            fmt.Printf("Non-Zencached error: %v\n", err)
        }
        return
    }

    if result.Type == zencached.ResultTypeNotFound {
        fmt.Println("Key not found")
    } else {
        fmt.Printf("Value: %s\n", string(result.Data))
    }
}
```

### Validation Example

```go
package main

import (
    "fmt"
    "github.com/rnojiri/zencached"
)

func main() {
    // Invalid key: contains spaces
    err := zencached.ValidateKey([]byte("my path"), []byte("my key"))
    if err != nil {
        fmt.Printf("Validation failed: %v\n", err)
    }

    // Invalid key: too long (> 250 bytes)
    longKey := make([]byte, 300)
    err = zencached.ValidateKey([]byte(""), longKey)
    if err != nil {
        fmt.Printf("Validation failed: %v\n", err)
    }

    // Valid key
    err = zencached.ValidateKey([]byte("path"), []byte("validkey"))
    if err == nil {
        fmt.Println("Key is valid")
    }
}
```

## Metrics Collection

### Custom Metrics Implementation

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
    "github.com/rnojiri/zencached"
)

type MetricsCollector struct {
    mu              sync.Mutex
    operations      int64
    errors          int64
    cacheHits       int64
    cacheMisses     int64
    totalLatency    int64
}

func (m *MetricsCollector) CommandExecutionElapsedTime(node string, operation, path, key []byte, elapsedTime int64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.totalLatency += elapsedTime
}

func (m *MetricsCollector) CommandExecution(node string, operation, path, key []byte) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.operations++
}

func (m *MetricsCollector) CommandExecutionError(node string, operation, path, key []byte, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.errors++
}

func (m *MetricsCollector) CacheMissEvent(node string, operation, path, key []byte) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.cacheMisses++
}

func (m *MetricsCollector) CacheHitEvent(node string, operation, path, key []byte) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.cacheHits++
}

func (m *MetricsCollector) NodeRebalanceEvent(numNodes int) {}
func (m *MetricsCollector) NodeListingEvent(numNodes int) {}
func (m *MetricsCollector) NodeListingError() {}
func (m *MetricsCollector) NodeListingElapsedTime(elapsedTime int64) {}
func (m *MetricsCollector) NodeRebalanceElapsedTime(elapsedTime int64) {}

func (m *MetricsCollector) GetStats() {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    avgLatency := float64(0)
    if m.operations > 0 {
        avgLatency = float64(m.totalLatency) / float64(m.operations) / 1_000_000 // Convert to ms
    }
    
    hitRate := float64(0)
    if m.cacheHits+m.cacheMisses > 0 {
        hitRate = float64(m.cacheHits) / float64(m.cacheHits+m.cacheMisses) * 100
    }
    
    fmt.Printf("Operations: %d\n", m.operations)
    fmt.Printf("Errors: %d\n", m.errors)
    fmt.Printf("Cache Hit Rate: %.2f%%\n", hitRate)
    fmt.Printf("Avg Latency: %.2f ms\n", avgLatency)
}

func main() {
    metrics := &MetricsCollector{}

    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "localhost", Port: 11211},
        },
        NumConnectionsPerNode:  5,
        ZencachedMetricsCollector: metrics,
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    ctx := context.Background()

    // Perform operations...
    for i := 0; i < 100; i++ {
        key := fmt.Sprintf("key%d", i)
        client.Set(ctx, nil, []byte(""), []byte(key), []byte("value"), 3600)
        client.Get(ctx, nil, []byte(""), []byte(key))
    }

    // Print metrics
    metrics.GetStats()
}
```

## Dynamic Node Discovery

### Service Discovery Integration

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/rnojiri/zencached"
)

// Example service discovery function
func getNodesFromServiceDiscovery() ([]zencached.Node, error) {
    // In practice, this would query Consul, Kubernetes, etc.
    return []zencached.Node{
        {Host: "memcached-1.service", Port: 11211},
        {Host: "memcached-2.service", Port: 11211},
    }, nil
}

func main() {
    config := &zencached.Configuration{
        NumConnectionsPerNode:  5,
        NodeListFunction:       getNodesFromServiceDiscovery,
        NumNodeListRetries:     3,
        NodeListRetryTimeout:   5 * time.Second,
        RebalanceOnDisconnection: true,
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    // Periodically rebalance to discover new nodes
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            fmt.Println("Rebalancing nodes...")
            client.Rebalance()
            
            nodes := client.GetConnectedNodes()
            fmt.Printf("Connected to %d nodes\n", len(nodes))
            for _, node := range nodes {
                fmt.Printf("  - %s:%d\n", node.Host, node.Port)
            }
        }
    }()

    ctx := context.Background()

    // Use client...
    result, err := client.Set(ctx, nil, []byte(""), []byte("key"), []byte("value"), 3600)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    } else {
        fmt.Printf("Success: %v\n", result.Type)
    }
}
```

## Context Usage

### Timeout Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/rnojiri/zencached"
)

func main() {
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "localhost", Port: 11211},
        },
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    // Create context with 1 second timeout
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    result, err := client.Get(ctx, nil, []byte(""), []byte("key"))
    if err != nil {
        fmt.Printf("Operation failed: %v\n", err)
        return
    }

    fmt.Printf("Got: %s\n", string(result.Data))
}
```

### Cancellation Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/rnojiri/zencached"
)

func main() {
    config := &zencached.Configuration{
        Nodes: []zencached.Node{
            {Host: "localhost", Port: 11211},
        },
    }

    client, err := zencached.New(config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown()

    ctx, cancel := context.WithCancel(context.Background())

    // Cancel after 5 seconds
    go func() {
        time.Sleep(5 * time.Second)
        cancel()
    }()

    // This will be cancelled after 5 seconds
    result, err := client.Get(ctx, nil, []byte(""), []byte("key"))
    if err != nil {
        fmt.Printf("Operation cancelled or failed: %v\n", err)
        return
    }

    fmt.Printf("Got: %s\n", string(result.Data))
}
```
