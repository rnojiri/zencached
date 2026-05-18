# Architecture

This document describes the internal architecture of Zencached.

## Table of Contents

- [System Overview](#system-overview)
- [Core Components](#core-components)
- [Operation Flow](#operation-flow)
- [Connection Management](#connection-management)
- [Compression Pipeline](#compression-pipeline)
- [Node Rebalancing](#node-rebalancing)
- [Metrics Collection](#metrics-collection)

## System Overview

```
┌─────────────────────────────────────────┐
│         Client Application              │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│       Zencached Client                  │
│  - Coordinate operations                │
│  - Route requests                       │
│  - Handle rebalancing                   │
└──────────────┬──────────────────────────┘
               │
      ┌────────┼────────┐
      │        │        │
┌─────▼──┐ ┌──▼───┐ ┌──▼───┐
│ Node   │ │ Node │ │ Node │
│Workers │ │Worker│ │Worker│
└─────┬──┘ └──┬───┘ └──┬───┘
      │       │        │
┌─────▼───────▼────────▼──────┐
│   Compression (Optional)    │
│   - Compress on Set         │
│   - Decompress on Get       │
└─────┬───────┬────────┬──────┘
      │       │        │
┌─────▼───────▼────────▼──────┐
│  Telnet Protocol Handler    │
│  - Text-based protocol      │
│  - Connection pooling       │
│  - Error handling           │
└─────┬───────┬────────┬──────┘
      │       │        │
┌─────▼───────▼────────▼──────┐
│     Memcached Nodes         │
│   :11211  :11212  :11213    │
└─────────────────────────────┘
```

## Core Components

### 1. Zencached Client

The main entry point and coordinator.

**Responsibilities:**
- Initialize client with configuration
- Route operations to appropriate nodes
- Manage node workers
- Handle metrics collection
- Trigger node rebalancing

**Key Fields:**
```go
type Zencached struct {
    configuration         *Configuration
    logger                *logh.ContextualLogger
    shuttingDown          atomic.Bool
    rebalanceLock         atomic.Bool
    notAvailable          atomic.Bool
    nodeWorkerArray       []*nodeWorkers
    connectedNodes        []Node
    rebalanceChannel      chan struct{}
    metricsEnabled        bool
    compressionEnabled    bool
    dataCompressor        DataCompressor
}
```

### 2. Node Workers

Manages connections and operations for a specific node.

**Responsibilities:**
- Maintain connection pool
- Queue and execute commands
- Handle connection recovery
- Report node health

**Components:**
- Connection Pool: Maintains multiple persistent connections
- Command Queue: Buffers pending operations
- Worker Goroutines: Process queued commands

### 3. Telnet Protocol Handler

Implements memcached text protocol communication.

**Responsibilities:**
- Establish TCP connections
- Send memcached commands
- Parse memcached responses
- Handle timeouts and retries

**Memcached Commands Supported:**
- `set`: Store a value
- `add`: Store only if not exists
- `get`: Retrieve a value
- `delete`: Remove a value
- `version`: Get server version

### 4. Data Compressor

Optional compression/decompression layer.

**Responsibilities:**
- Compress values before storage
- Decompress values after retrieval
- Support multiple compression algorithms

**Supported Algorithms:**
- Zstandard (ZSTD) - Recommended for high compression ratio

### 5. Metrics Collector

Collects performance and operational metrics.

**Responsibilities:**
- Track operation timing
- Record cache hit/miss events
- Monitor node health
- Report errors

## Operation Flow

### Single Node Set Operation

```
1. Client calls Set()
        │
2. Validate key
        │
3. Select target node (based on routerHash or consistent hashing)
        │
4. Compress value (if enabled)
        │
5. Encode memcached command
        │
6. Queue command in node worker
        │
7. Node worker sends command via Telnet
        │
8. Node worker receives response
        │
9. Parse response
        │
10. Collect metrics
        │
11. Return OperationResult
```

### Cluster Set Operation

```
1. Client calls ClusterSet()
        │
2. Validate key
        │
3. Compress value (if enabled)
        │
4. For each connected node:
   │
   ├─> Get node worker
   ├─> Encode memcached command
   ├─> Queue command
   ├─> Collect metrics
        │
5. Wait for all operations to complete
        │
6. Return results and errors
```

### Get Operation with Cache Hit

```
1. Client calls Get()
        │
2. Validate key
        │
3. Select target node
        │
4. Node worker sends GET command
        │
5. Node returns VALUE with data
        │
6. Decompress value (if compressed)
        │
7. Record cache hit metric
        │
8. Return OperationResult with data
```

### Get Operation with Cache Miss

```
1. Client calls Get()
        │
2. Validate key
        │
3. Select target node
        │
4. Node worker sends GET command
        │
5. Node returns END (not found)
        │
6. Record cache miss metric
        │
7. Return OperationResult with ResultTypeNotFound
```

## Connection Management

### Connection Pool Architecture

Each node has a pool of persistent TCP connections:

```
┌─────────────────────────────┐
│     Node Worker             │
├─────────────────────────────┤
│ Connection Pool             │
│ ┌─────────────────────────┐ │
│ │ Conn 1 ────-> [Telnet]  │ │
│ │ Conn 2 ────-> [Telnet]  │ │
│ │ Conn 3 ----X  [Closed]  │ │
│ │ Conn 4 ────-> [Telnet]  │ │
│ │ Conn 5 ────-> [Telnet]  │ │
│ └─────────────────────────┘ │
│                             │
│ Command Queue (FIFO)        │
│ ┌─────────────────────────┐ │
│ │ Set user1 -> value1     │ │
│ │ Get user2               │ │
│ │ Delete user3            │ │
│ └─────────────────────────┘ │
└─────────────────────────────┘
```

### Connection Lifecycle

1. **Establishment**: Connection created on demand from pool
2. **Usage**: Connection reused for multiple operations
3. **Health Check**: Periodic connection validation
4. **Reconnection**: Automatic reconnect on failure with backoff
5. **Cleanup**: Connection closed on shutdown

### Connection Retry Strategy

```
Operation Attempt
        │
        ├─> Try Connection 1 -> Success? ✓ Return
        │
        └─> Try Connection 1 -> Fail?
                │
                ├─> Try Connection 2 -> Success? ✓ Return
                │
                └─> Try Connection 2 -> Fail?
                        │
                        ├─> Try Connection 3 -> Success? ✓ Return
                        │
                        └─> Try Connection 3 -> Fail?
                                │
                                └─> Return Error
```

## Compression Pipeline

### Compression Flow (Set Operation)

```
Original Value (100KB)
        │
        ▼
[Compress]
        │
        ▼
Compressed Value (15KB)
        │
        ▼
[Memcached Store]
        │
        ▼
Value stored on node
```

### Decompression Flow (Get Operation)

```
Value from Node (15KB)
        │
        ▼
[Decompress]
        │
        ▼
Original Value (100KB)
        │
        ▼
[Return to Client]
```

### Compression Configuration Impact

| Setting | Throughput | CPU | Memory | Network |
|---------|-----------|-----|--------|---------|
| None | High | Low | High | High |
| ZSTD-3 | Medium | Medium | Medium | Low |
| ZSTD-9 | Low | High | Low | Very Low |

## Node Rebalancing

### Rebalancing Triggers

1. **Automatic on Disconnection** (if enabled)
   - Triggered when node becomes unavailable
   - Attempts to discover new nodes

2. **Manual Rebalancing**
   - Called explicitly: `client.Rebalance()`
   - Useful for periodic node discovery

### Rebalancing Process

```
1. Acquire rebalance lock
        │
2. Call NodeListFunction (if configured)
   or use static node list
        │
3. Compare with current nodes
   ├─> New nodes? -> Connect
   ├─> Dead nodes? -> Disconnect
   └─> Existing nodes? -> Keep
        │
4. Create/update node workers
        │
5. Update connected nodes list
        │
6. Release rebalance lock
        │
7. Emit metrics
```

### Dynamic Node Discovery Example

```
Service Discovery (Consul/K8s)
        │
        ├─> memcached-1:11211  ✓
        ├─> memcached-2:11211  ✓
        └─> memcached-3:11211  ✓
        │
        ▼
Zencached Rebalance
        │
        ├─> Connect to memcached-1
        ├─> Connect to memcached-2
        ├─> Disconnect memcached-old (not in list)
        └─> Update connected nodes
```

## Metrics Collection

### Metrics Pipeline

```
Operation Execution
        │
        ├─> Start timer
        │
        ├─> Execute command
        │
        ├─> End timer
        │
        ├─> Emit metrics:
        │   ├─> CommandExecutionElapsedTime
        │   ├─> CommandExecution
        │   ├─> CacheHitEvent / CacheMissEvent
        │   └─> CommandExecutionError (on error)
        │
        └─> Return result
```

### Metrics Categories

1. **Operation Metrics**
   - Command execution count
   - Command execution time
   - Command execution errors

2. **Cache Metrics**
   - Cache hits
   - Cache misses
   - Hit rate

3. **Node Metrics**
   - Node rebalance events
   - Node listing events
   - Node listing errors

4. **Telnet Metrics**
   - Connection timing
   - Data transfer size
   - Protocol overhead

### Example Metrics Collection

```go
// Timing a Set operation
start := time.Now()
err := client.Set(ctx, nil, path, key, value, ttl)
elapsed := time.Since(start).Nanoseconds()

metrics.CommandExecutionElapsedTime(node, "set", path, key, elapsed)

if err != nil {
    metrics.CommandExecutionError(node, "set", path, key, err)
} else {
    metrics.CommandExecution(node, "set", path, key)
}
```

## Error Handling

### Error Propagation

```
Memcached Node
        │
        ├─> Connection error?
        │   └─> Propagate to operation
        │
        ├─> Protocol error?
        │   └─> Propagate to operation
        │
        ├─> Key not found?
        │   └─> Return ResultTypeNotFound
        │
        └─> Success
            └─> Return result
        │
        ▼
Operation Error Handler
        │
        ├─> Retry?
        ├─> Collect metrics
        └─> Return to client
```

### Error Recovery

- **Transient Errors**: Automatic retry with backoff
- **Connection Errors**: Reconnect to node
- **Protocol Errors**: Log and propagate
- **Node Unavailable**: Mark node down, trigger rebalance (if enabled)

## Concurrency Model

### Goroutine Architecture

```
Main Client Thread
        │
        ├─> Rebalance Worker (goroutine)
        │   └─> Monitors rebalance channel
        │
        ├─> Node Worker 1 (goroutine)
        │   └─> Processes command queue
        │
        ├─> Node Worker 2 (goroutine)
        │   └─> Processes command queue
        │
        └─> Node Worker N (goroutine)
            └─> Processes command queue
```

### Synchronization Primitives

- **atomic.Bool**: Shutdown and rebalance flags
- **Channels**: Rebalance signaling
- **Goroutines**: Worker parallelism
- **Mutexes**: Protected by node workers internally

## Performance Characteristics

### Latency

- **Local memcached**: 1-5ms
- **Network latency**: 5-50ms (depending on distance)
- **Compression overhead**: 1-10ms (depending on data size and compression level)

### Throughput

- **Single connection**: 1,000-10,000 ops/sec
- **Connection pool (5 conn)**: 5,000-50,000 ops/sec
- **Multiple nodes**: Scales linearly

### Memory Usage

- **Base client**: ~1MB
- **Per connection**: ~100KB
- **Command queue**: Depends on buffer size
- **Compression buffer**: ~1-2MB temporary

## Scalability Considerations

1. **Horizontal Scaling**: Add more memcached nodes
2. **Vertical Scaling**: Increase connections per node
3. **Connection Pooling**: Balance between overhead and throughput
4. **Compression**: Trade CPU for network bandwidth
5. **Metrics**: Optional for performance-critical paths
