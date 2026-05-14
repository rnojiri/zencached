# Error Types Reference

This document describes all error types and their meanings in Zencached.

## Error Type Enumeration

Zencached defines several error types to help with debugging and error handling.

### ErrorType Constants

```go
type ErrorType int

const (
    ErrorTypeConnected           ErrorType = iota
    ErrorTypeNotConnected
    ErrorTypeKeyInvalid
    ErrorTypeValueInvalid
    ErrorTypeNotFound
    ErrorTypeAlreadyStored
    ErrorTypeNotStored
    ErrorTypeConnectionFailed
    ErrorTypeReadFailed
    ErrorTypeWriteFailed
    ErrorTypeTimeout
    ErrorTypeBufferFull
    ErrorTypeCompressionFailed
    ErrorTypeDecompressionFailed
)
```

## Error Descriptions

### Connection Errors

#### ErrorTypeConnected
**When:** A connection operation is attempted when already connected.
**Cause:** Internal state mismatch.
**Action:** This is typically an internal error; report to maintainers.

#### ErrorTypeNotConnected
**When:** An operation is attempted without an active connection.
**Cause:** Node is not connected to memcached server.
**Action:** Trigger rebalancing or check memcached server status.

```go
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeNotConnected {
    fmt.Println("Rebalancing due to no connection...")
    client.Rebalance()
}
```

#### ErrorTypeConnectionFailed
**When:** Unable to establish connection to memcached node.
**Cause:** 
- Memcached server is down
- Network is unreachable
- Firewall is blocking connection
- Wrong host/port configuration

**Action:** 
- Check memcached server is running: `telnet localhost 11211`
- Verify host/port configuration
- Check network connectivity
- Check firewall rules

```go
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeConnectionFailed {
    fmt.Println("Cannot connect to memcached node")
    // Check server status, then retry or rebalance
}
```

### Key/Value Errors

#### ErrorTypeKeyInvalid
**When:** Key validation fails.
**Cause:** 
- Key is empty
- Key is too long (> 250 bytes)
- Key contains spaces
- Key contains newline characters
- Combined path + key exceeds limits

**Action:** Validate key before using:

```go
err := zencached.ValidateKey([]byte("path"), []byte("key"))
if err != nil {
    fmt.Printf("Invalid key: %v\n", err)
    // Use a different key
}
```

**Valid Key Rules:**
- 1-250 bytes total length
- No spaces
- No newlines (`\r` or `\n`)
- No control characters

```go
// Valid keys
ValidateKey([]byte(""), []byte("user123"))           // ✓
ValidateKey([]byte("cache"), []byte("key:456"))     // ✓
ValidateKey([]byte("session"), []byte("id_789"))    // ✓

// Invalid keys
ValidateKey([]byte("cache"), []byte("key 456"))     // ✗ Contains space
ValidateKey([]byte(""), []byte(""))                 // ✗ Empty
ValidateKey([]byte(""), make([]byte, 300))          // ✗ Too long
```

#### ErrorTypeValueInvalid
**When:** Value validation fails.
**Cause:** 
- Value is too large for memcached
- Value encoding error

**Action:** Check value size and encoding.

### Operation Errors

#### ErrorTypeNotFound
**When:** Get operation on non-existent key.
**Cause:** Key not found in cache.
**Action:** This is normal for cache misses; handle gracefully.

```go
result, err := client.Get(ctx, nil, []byte("cache"), []byte("key"))
if err == nil && result.Type == zencached.ResultTypeNotFound {
    fmt.Println("Cache miss - fetch from source")
    // Fetch from database
}
```

#### ErrorTypeAlreadyStored
**When:** Add operation on existing key.
**Cause:** Key already exists in cache.
**Action:** Use Set instead if you want to overwrite, or use Add with different key.

```go
result, err := client.Add(ctx, nil, []byte(""), []byte("key"), []byte("value"), 3600)
if err == nil && result.Type == zencached.ResultTypeNotStored {
    fmt.Println("Key already exists")
    // Use Set to overwrite, or choose different key
}
```

#### ErrorTypeNotStored
**When:** Set/Add operation fails to store.
**Cause:** 
- Server error
- Protocol error
- Connection lost during operation

**Action:** Retry operation or handle as cache write failure.

```go
result, err := client.Set(ctx, nil, []byte(""), []byte("key"), []byte("value"), 3600)
if err != nil || result.Type == zencached.ResultTypeNotStored {
    fmt.Println("Failed to store value")
    // Retry or fallback
}
```

### I/O Errors

#### ErrorTypeReadFailed
**When:** Reading response from memcached fails.
**Cause:** 
- Network connection interrupted
- Timeout reading from socket
- Malformed response from server
- Buffer too small for response

**Action:** Retry operation or rebalance.

```go
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeReadFailed {
    fmt.Println("Failed to read from memcached")
    // Retry or check network
}
```

#### ErrorTypeWriteFailed
**When:** Writing command to memcached fails.
**Cause:** 
- Network connection interrupted
- Timeout writing to socket
- Connection closed by server

**Action:** Retry operation or rebalance.

```go
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeWriteFailed {
    fmt.Println("Failed to write to memcached")
    // Retry or rebalance
}
```

#### ErrorTypeTimeout
**When:** Operation exceeds configured timeout.
**Cause:** 
- Memcached server is slow
- Network latency too high
- Memcached server is overloaded

**Action:** Increase timeout or investigate server performance.

```go
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeTimeout {
    fmt.Println("Operation timed out")
    // Increase timeout or check server load
}
```

### Buffer/Queue Errors

#### ErrorTypeBufferFull
**When:** Command execution buffer is full.
**Cause:** 
- Too many pending operations
- Node is slow processing commands
- Insufficient buffer size configured

**Action:** 
- Reduce request rate
- Increase `CommandExecutionBufferSize` in configuration
- Add more nodes or connections per node

```go
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeBufferFull {
    fmt.Println("Too many pending operations")
    // Wait before retrying or increase buffer size
    time.Sleep(time.Millisecond * 100)
    retry()
}
```

### Compression Errors

#### ErrorTypeCompressionFailed
**When:** Data compression fails.
**Cause:** 
- Compression algorithm error
- Memory exhaustion
- Corrupted data during compression

**Action:** Check compression configuration or disable compression.

```go
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeCompressionFailed {
    fmt.Println("Failed to compress data")
    // Disable compression or use smaller values
}
```

#### ErrorTypeDecompressionFailed
**When:** Data decompression fails.
**Cause:** 
- Compressed data is corrupted
- Decompression algorithm mismatch
- Memory exhaustion

**Action:** The cached value is likely corrupted; delete it or disable compression.

```go
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeDecompressionFailed {
    fmt.Println("Failed to decompress data - cache may be corrupted")
    // Delete the key or investigate
    client.Delete(ctx, nil, path, key)
}
```

## Error Handling Strategies

### Strategy 1: Retry with Backoff

```go
func getWithRetry(client zencached.IZencached, ctx context.Context, 
    path, key []byte, maxRetries int) (*zencached.OperationResult, error) {
    
    var lastErr error
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        result, err := client.Get(ctx, nil, path, key)
        if err == nil {
            return result, nil
        }
        
        lastErr = err
        
        // Exponential backoff
        backoff := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
        time.Sleep(backoff)
    }
    
    return nil, lastErr
}
```

### Strategy 2: Fallback to Source

```go
func getValue(client zencached.IZencached, ctx context.Context, 
    path, key []byte) ([]byte, error) {
    
    // Try cache first
    result, err := client.Get(ctx, nil, path, key)
    if err == nil && result.Type == zencached.ResultTypeFound {
        return result.Data, nil
    }
    
    // Cache miss or error - fetch from source
    data, err := fetchFromDatabase(key)
    if err != nil {
        return nil, err
    }
    
    // Try to cache the result (best effort)
    client.Set(ctx, nil, path, key, data, 3600)
    
    return data, nil
}
```

### Strategy 3: Error Categorization

```go
func handleError(err error) {
    if err == nil {
        return
    }
    
    if ze, ok := err.(zencached.ZError); ok {
        switch ze.Code() {
        // Transient errors - retry
        case zencached.ErrorTypeTimeout,
            zencached.ErrorTypeNotConnected:
            fmt.Println("Transient error - will retry")
            
        // Connection errors - rebalance
        case zencached.ErrorTypeConnectionFailed:
            fmt.Println("Connection error - rebalancing")
            // client.Rebalance()
            
        // Input errors - fix and retry
        case zencached.ErrorTypeKeyInvalid:
            fmt.Println("Invalid input - fix key format")
            
        // Data errors - skip
        case zencached.ErrorTypeDecompressionFailed:
            fmt.Println("Data error - removing corrupted key")
            
        default:
            fmt.Printf("Unexpected error: %v\n", ze)
        }
    } else {
        fmt.Printf("Non-Zencached error: %v\n", err)
    }
}
```

## Testing Error Scenarios

### Test Connection Error

```go
config := &zencached.Configuration{
    Nodes: []zencached.Node{
        {Host: "invalid-host-123", Port: 11211},
    },
}

client, err := zencached.New(config)
if err != nil {
    // This is expected
    fmt.Printf("Expected error: %v\n", err)
}
```

### Test Timeout

```go
telnetConfig := &zencached.TelnetConfiguration{
    MaxReadTimeout: 1 * time.Millisecond,  // Very short timeout
}

// Operations will likely timeout
result, err := client.Get(ctx, nil, []byte(""), []byte("key"))
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeTimeout {
    fmt.Println("Timeout as expected with very short timeout")
}
```

### Test Invalid Key

```go
err := zencached.ValidateKey([]byte(""), []byte("key with spaces"))
if ze, ok := err.(zencached.ZError); ok && ze.Code() == zencached.ErrorTypeKeyInvalid {
    fmt.Println("Invalid key detected as expected")
}
```

## Common Error Patterns

### Pattern 1: Operation Failed, Trigger Rebalance

```go
result, err := client.Get(ctx, nil, []byte(""), []byte("key"))
if err != nil {
    if ze, ok := err.(zencached.ZError); ok {
        if ze.Code() == zencached.ErrorTypeConnectionFailed ||
           ze.Code() == zencached.ErrorTypeNotConnected {
            client.Rebalance()
        }
    }
}
```

### Pattern 2: Cache Operation Succeeded, But...

```go
result, err := client.Get(ctx, nil, []byte(""), []byte("key"))
if err == nil {
    if result.Type == zencached.ResultTypeNotFound {
        // Cache miss - normal
    } else {
        // Found
    }
} else {
    // Error occurred
}
```

### Pattern 3: Cluster Operation with Partial Failures

```go
results, errs := client.ClusterSet(ctx, []byte(""), []byte("key"), 
    []byte("value"), 3600)

successCount := 0
for i, result := range results {
    if errs[i] != nil {
        fmt.Printf("Node %d failed: %v\n", i, errs[i])
    } else if result.Type == zencached.ResultTypeStored {
        successCount++
    }
}

if successCount < len(results)/2 {
    fmt.Println("Warning: Most nodes failed")
}
```
