# Troubleshooting Guide

This guide covers common issues and solutions when using Zencached.

## Table of Contents

- [Connection Issues](#connection-issues)
- [Performance Issues](#performance-issues)
- [Data Issues](#data-issues)
- [Cluster Issues](#cluster-issues)
- [Compression Issues](#compression-issues)
- [Metrics Issues](#metrics-issues)
- [Debugging Techniques](#debugging-techniques)

## Connection Issues

### Problem: "Connection refused" or "No connection could be made"

**Symptoms:**
```
Error: connection refused
or
Error: Unable to connect to memcached
```

**Causes:**
- Memcached server is not running
- Wrong hostname or port
- Firewall blocking connection
- Network unreachable

**Solutions:**

1. **Check memcached is running**:
   ```bash
   telnet localhost 11211
   # Should connect successfully
   ```

2. **Verify configuration**:
   ```go
   config := &zencached.Configuration{
       Nodes: []zencached.Node{
           {Host: "localhost", Port: 11211},  // Verify these values
       },
   }
   ```

3. **Check firewall**:
   ```bash
   # Linux
   sudo iptables -L | grep 11211
   
   # macOS with Little Snitch
   # Check network settings
   ```

4. **Test connectivity**:
   ```bash
   telnet memcached-server 11211
   # If this works, issue is in Go code
   ```

5. **Enable debug logs**:
   ```go
   config.TelnetConfiguration.EnableTracerLogs = true
   ```

### Problem: "Connection timeout"

**Symptoms:**
```
Error: i/o timeout
or
Error: context deadline exceeded
```

**Causes:**
- Network latency too high
- Memcached server is slow/overloaded
- Timeout value too short
- Network congestion

**Solutions:**

1. **Check network latency**:
   ```bash
   ping memcached-server
   # Look for high latency values
   ```

2. **Increase timeout**:
   ```go
   config.TelnetConfiguration.MaxReadTimeout = 10 * time.Second
   config.TelnetConfiguration.MaxWriteTimeout = 10 * time.Second
   ```

3. **Check memcached load**:
   ```bash
   # Connect to memcached
   echo "stats" | nc localhost 11211
   # Look for high bytes_read/bytes_written
   ```

4. **Use context timeout**:
   ```go
   ctx, cancel := context.WithTimeout(
       context.Background(),
       10 * time.Second,
   )
   defer cancel()
   
   result, err := client.Get(ctx, nil, path, key)
   ```

### Problem: "Too many connections" error

**Symptoms:**
```
Error: resource temporarily unavailable
or
Error: connection limit exceeded
```

**Causes:**
- Too many connections configured
- Connections not being properly closed
- Memcached connection limit reached

**Solutions:**

1. **Reduce connections per node**:
   ```go
   config.NumConnectionsPerNode = 5  // Reduce if too high
   ```

2. **Check memcached limit**:
   ```bash
   echo "stats" | nc localhost 11211 | grep "maxbytes_mem"
   ```

3. **Ensure proper shutdown**:
   ```go
   defer client.Shutdown()  // Always defer shutdown
   ```

4. **Monitor connections**:
   ```bash
   # Check active connections
   echo "stats" | nc localhost 11211 | grep "curr_conns"
   ```

## Performance Issues

### Problem: "Slow operations"

**Symptoms:**
- Operations take longer than expected
- Latency high

**Causes:**
- Network latency
- Memcached server overloaded
- Data compression overhead
- Buffer size too small

**Solutions:**

1. **Check operation latency**:
   ```go
   type LatencyMetrics struct{}

   func (m *LatencyMetrics) CommandExecutionElapsedTime(
       node string, operation, path, key []byte, elapsedTime int64) {
       ms := float64(elapsedTime) / 1_000_000
       fmt.Printf("Operation took: %.2f ms\n", ms)
   }
   ```

2. **Disable compression if not needed**:
   ```go
   config.CompressionType = zencached.CompressionTypeNone
   ```

3. **Increase read buffer**:
   ```go
   config.TelnetConfiguration.ReadBufferSize = 65536  // 64KB
   ```

4. **Increase connections**:
   ```go
   config.NumConnectionsPerNode = 20  // More parallelism
   ```

5. **Check memcached stats**:
   ```bash
   echo "stats" | nc localhost 11211
   # Look for get_hits, get_misses, evictions
   ```

### Problem: "Low throughput"

**Symptoms:**
- Operations per second is lower than expected

**Causes:**
- Single-threaded usage
- Too few connections
- Buffer too small

**Solutions:**

1. **Increase parallelism**:
   ```go
   // Use multiple goroutines
   for i := 0; i < 10; i++ {
       go func() {
           for j := 0; j < 1000; j++ {
               client.Set(ctx, nil, path, key, value, ttl)
           }
       }()
   }
   ```

2. **Increase connections**:
   ```go
   config.NumConnectionsPerNode = 10
   ```

3. **Increase command buffer**:
   ```go
   config.CommandExecutionBufferSize = 5000
   ```

4. **Use cluster operations** for replicated data:
   ```go
   // Instead of multiple Set calls
   client.ClusterSet(ctx, path, key, value, ttl)
   ```

### Problem: "Memory bloat"

**Symptoms:**
- Memory usage keeps increasing
- Garbage collection doesn't help

**Causes:**
- Buffers not being released
- Metrics accumulating data
- Connection leak

**Solutions:**

1. **Ensure shutdown is called**:
   ```go
   defer client.Shutdown()
   ```

2. **Check for goroutine leaks**:
   ```bash
   # Use pprof to check goroutines
   import _ "net/http/pprof"
   go func() {
       log.Println(http.ListenAndServe("localhost:6060", nil))
   }()
   // Check http://localhost:6060/debug/pprof/goroutine
   ```

3. **Limit buffer size**:
   ```go
   config.CommandExecutionBufferSize = 100  // Smaller buffer
   ```

4. **Implement bounded metrics collection**:
   ```go
   type BoundedMetrics struct {
       maxSize int
       size    int
       data    map[string]int64
   }

   func (m *BoundedMetrics) record(key string) {
       if m.size >= m.maxSize {
           // Clear old data
           m.data = make(map[string]int64)
           m.size = 0
       }
       m.data[key]++
       m.size++
   }
   ```

## Data Issues

### Problem: "Key not found" when should exist

**Symptoms:**
```
ResultTypeNotFound when value should exist
```

**Causes:**
- TTL expired
- Key deleted by someone else
- Wrong key format
- Wrong node selection

**Solutions:**

1. **Check TTL**:
   ```go
   // Set with longer TTL
   result, err := client.Set(ctx, nil, path, key, value, 86400)  // 24 hours
   ```

2. **Verify key format**:
   ```go
   err := zencached.ValidateKey(path, key)
   if err != nil {
       fmt.Printf("Invalid key: %v\n", err)
   }
   ```

3. **Check consistent hashing**:
   ```go
   // If using custom router hash, verify consistency
   routerHash := []byte("consistent-key")
   result1, _ := client.Get(ctx, routerHash, path, key)
   result2, _ := client.Get(ctx, routerHash, path, key)
   // Should get same result
   ```

### Problem: "Corrupted or unexpected data"

**Symptoms:**
```
Data doesn't match what was stored
or
Decompression error
```

**Causes:**
- Data corruption during transmission
- Compression/decompression mismatch
- Encoding issues

**Solutions:**

1. **Verify data integrity**:
   ```go
   originalValue := []byte("test-value")
   client.Set(ctx, nil, path, key, originalValue, 3600)

   result, _ := client.Get(ctx, nil, path, key)
   if string(result.Data) != string(originalValue) {
       fmt.Println("Data mismatch!")
   }
   ```

2. **Check compression settings**:
   ```go
   // Ensure compression is enabled for both Set and Get
   config.CompressionType = zencached.CompressionTypeZSTD
   ```

3. **Disable compression temporarily**:
   ```go
   // Create new client without compression
   config.CompressionType = zencached.CompressionTypeNone
   result, _ := client.Get(ctx, nil, path, key)
   ```

### Problem: "Data expires too quickly"

**Symptoms:**
- Cached data disappears before expected TTL

**Causes:**
- Wrong TTL value
- Memcached eviction policy
- Memcached memory full

**Solutions:**

1. **Check TTL value**:
   ```go
   // TTL is in seconds
   client.Set(ctx, nil, path, key, value, 86400)  // 24 hours, not 24 days
   ```

2. **Check memcached eviction**:
   ```bash
   echo "stats" | nc localhost 11211 | grep "evictions"
   # Non-zero means items being evicted due to memory
   ```

3. **Increase memcached memory**:
   ```bash
   # Start memcached with more memory
   memcached -m 512  # 512 MB
   ```

## Cluster Issues

### Problem: "Inconsistent data across nodes"

**Symptoms:**
```
ClusterGet returns different values from different nodes
```

**Causes:**
- Cluster operation failed partially
- Network partition
- Nodes out of sync

**Solutions:**

1. **Check cluster consistency**:
   ```go
   results, errs := client.ClusterGet(ctx, path, key)

   var firstData []byte
   for i, result := range results {
       if errs[i] == nil && result.Type == zencached.ResultTypeFound {
           if firstData == nil {
               firstData = result.Data
           } else if string(firstData) != string(result.Data) {
               fmt.Printf("Node %d has inconsistent data\n", i)
           }
       }
   }
   ```

2. **Resync cluster**:
   ```go
   // Set on all nodes again
   client.ClusterSet(ctx, path, key, value, ttl)
   ```

3. **Monitor node health**:
   ```go
   nodes := client.GetConnectedNodes()
   fmt.Printf("Connected to %d nodes\n", len(nodes))
   for _, node := range nodes {
       fmt.Printf("  - %s:%d\n", node.Host, node.Port)
   }
   ```

### Problem: "Node rebalancing failing"

**Symptoms:**
```
Rebalance doesn't discover new nodes
or
Connected to fewer nodes than expected
```

**Causes:**
- Node listing function failing
- Service discovery unavailable
- Network issues

**Solutions:**

1. **Check node listing function**:
   ```go
   nodes, err := config.NodeListFunction()
   if err != nil {
       fmt.Printf("Node listing error: %v\n", err)
   }
   fmt.Printf("Found %d nodes\n", len(nodes))
   ```

2. **Enable debug logging**:
   ```go
   config.TelnetConfiguration.EnableTracerLogs = true
   ```

3. **Increase retry attempts**:
   ```go
   config.NumNodeListRetries = 5
   config.NodeListRetryTimeout = 10 * time.Second
   ```

4. **Manual rebalancing**:
   ```go
   client.Rebalance()
   nodes := client.GetConnectedNodes()
   fmt.Printf("After rebalance: %d nodes\n", len(nodes))
   ```

## Compression Issues

### Problem: "Compression failed"

**Symptoms:**
```
Error: compression failed
```

**Causes:**
- Memory exhaustion
- Compression algorithm error
- Data too large

**Solutions:**

1. **Disable compression**:
   ```go
   config.CompressionType = zencached.CompressionTypeNone
   ```

2. **Reduce compression level**:
   ```go
   config.CompressionLevel = 1  // Faster, less memory
   ```

3. **Check available memory**:
   ```bash
   free -h  # Linux
   # or
   vm_stat  # macOS
   ```

### Problem: "Decompression failed"

**Symptoms:**
```
Error: decompression failed
```

**Causes:**
- Compressed data is corrupted
- Compression settings changed
- Data manually inserted without compression

**Solutions:**

1. **Delete corrupted key**:
   ```go
   client.Delete(ctx, nil, path, key)
   ```

2. **Check compression was enabled when stored**:
   ```go
   // Store with compression
   config.CompressionType = zencached.CompressionTypeZSTD
   ```

3. **Verify compression consistency**:
   ```go
   // All clients must have same compression settings
   ```

## Metrics Issues

### Problem: "Metrics not being collected"

**Symptoms:**
- Metrics collector methods not being called

**Causes:**
- Metrics collector not set in configuration
- Metrics collector is nil

**Solutions:**

1. **Verify metrics is configured**:
   ```go
   config.ZencachedMetricsCollector = &MyMetricsCollector{}
   // Not nil
   ```

2. **Check metrics implementation**:
   ```go
   type MyMetricsCollector struct{}

   func (m *MyMetricsCollector) CommandExecution(
       node string, operation, path, key []byte) {
       fmt.Printf("Operation: %s\n", string(operation))
   }
   ```

## Debugging Techniques

### Technique 1: Enable Tracer Logs

```go
config.TelnetConfiguration.EnableTracerLogs = true
```

This will log detailed information about:
- Connection establishment
- Data transmission
- Response parsing

### Technique 2: Add Custom Logging

```go
type LoggingMetricsCollector struct {
    log *log.Logger
}

func (l *LoggingMetricsCollector) CommandExecution(
    node string, operation, path, key []byte) {
    l.log.Printf("Command: %s on %s\n", string(operation), node)
}

// ... implement other methods
```

### Technique 3: Use pprof for Profiling

```go
import _ "net/http/pprof"

go func() {
    http.ListenAndServe("localhost:6060", nil)
}()

// In browser:
// http://localhost:6060/debug/pprof/
```

### Technique 4: Add Metrics Dashboard

Create a simple HTTP handler to view metrics:

```go
http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    metrics.PrintStats(w)
})
http.ListenAndServe(":8080", nil)
```

### Technique 5: Test Isolation

Test each component separately:

```go
// Test connection
conn, err := net.Dial("tcp", "localhost:11211")

// Test compression
compressor, _ := zencached.NewDataCompressor(
    zencached.CompressionTypeZSTD, 3)
compressed, _ := compressor.Compress([]byte("test"))

// Test client
client, _ := zencached.New(config)
```

## Getting Help

If you can't resolve the issue:

1. Check the [ERROR_TYPES.md](./ERROR_TYPES.md) documentation
2. Review [EXAMPLES.md](./EXAMPLES.md) for usage patterns
3. Open an issue on GitHub with:
   - Clear description of problem
   - Steps to reproduce
   - Error messages/logs
   - Configuration used
   - Environment (OS, Go version, memcached version)

## Additional Resources

- [Configuration Guide](./CONFIGURATION.md)
- [API Reference](./API.md)
- [Examples](./EXAMPLES.md)
- [Architecture Guide](./ARCHITECTURE.md)
- [Memcached Protocol](https://github.com/memcached/memcached/blob/master/doc/protocol.txt)
