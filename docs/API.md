# API Reference

Complete API reference for Zencached.

## Table of Contents

- [Initialization](#initialization)
- [Basic Operations](#basic-operations)
- [Cluster Operations](#cluster-operations)
- [Node Management](#node-management)
- [Data Types](#data-types)
- [Interfaces](#interfaces)

## Initialization

### Creating a Client

```go
func New(configuration *Configuration) (*Zencached, error)
```

Creates a new Zencached client instance.

**Parameters:**
- `configuration`: Configuration structure with all settings

**Returns:**
- `*Zencached`: Client instance
- `error`: Error if initialization fails

**Example:**
```go
config := &zencached.Configuration{
    Nodes: []zencached.Node{
        {Host: "localhost", Port: 11211},
    },
    NumConnectionsPerNode: 5,
}

client, err := zencached.New(config)
if err != nil {
    log.Fatal(err)
}
```

## Basic Operations

All operations accept a `context.Context` parameter for timeout and cancellation control.

### Set

```go
func (z *Zencached) Set(
    ctx context.Context,
    routerHash []byte,
    path []byte,
    key []byte,
    value []byte,
    ttl uint64,
) (*OperationResult, error)
```

Stores a key-value pair in the cache. Creates or overwrites the value if it already exists.

**Parameters:**
- `ctx`: Context for timeout/cancellation
- `routerHash`: Optional hash for node selection (if nil, consistent hashing is used)
- `path`: Optional path prefix for the key
- `key`: The key name
- `value`: The value to store
- `ttl`: Time-to-live in seconds (0 = never expires)

**Returns:**
- `*OperationResult`: Result information including node and operation type
- `error`: Error if operation fails

**Example:**
```go
result, err := client.Set(
    ctx,
    nil,
    []byte("users"),
    []byte("user123"),
    []byte(`{"name":"John","age":30}`),
    3600,  // 1 hour
)
if err != nil {
    log.Printf("Set failed: %v", err)
    return
}
fmt.Printf("Operation: %v on node %s\n", result.Type, result.Node.Host)
```

### Add

```go
func (z *Zencached) Add(
    ctx context.Context,
    routerHash []byte,
    path []byte,
    key []byte,
    value []byte,
    ttl uint64,
) (*OperationResult, error)
```

Stores a key-value pair only if the key doesn't already exist. Similar to Set but fails if the key exists.

**Parameters:**
- Same as Set

**Returns:**
- `*OperationResult`: Result type will be `ResultTypeStored` or `ResultTypeNotStored`
- `error`: Error if operation fails

**Example:**
```go
result, err := client.Add(
    ctx,
    nil,
    []byte("sessions"),
    []byte("session123"),
    []byte("session_data"),
    1800,  // 30 minutes
)
if err != nil {
    log.Printf("Add failed: %v", err)
}
if result.Type == zencached.ResultTypeNotStored {
    fmt.Println("Key already exists")
}
```

### Get

```go
func (z *Zencached) Get(
    ctx context.Context,
    routerHash []byte,
    path []byte,
    key []byte,
) (*OperationResult, error)
```

Retrieves a value from the cache.

**Parameters:**
- `ctx`: Context for timeout/cancellation
- `routerHash`: Optional hash for node selection
- `path`: Optional path prefix for the key
- `key`: The key name

**Returns:**
- `*OperationResult`: Contains the value data and node information
- `error`: Error if operation fails

**Example:**
```go
result, err := client.Get(
    ctx,
    nil,
    []byte("users"),
    []byte("user123"),
)
if err != nil {
    log.Printf("Get failed: %v", err)
    return
}

if result.Type == zencached.ResultTypeFound {
    fmt.Printf("Value: %s\n", string(result.Data))
} else {
    fmt.Println("Key not found")
}
```

### Delete

```go
func (z *Zencached) Delete(
    ctx context.Context,
    routerHash []byte,
    path []byte,
    key []byte,
) (*OperationResult, error)
```

Deletes a key from the cache.

**Parameters:**
- `ctx`: Context for timeout/cancellation
- `routerHash`: Optional hash for node selection
- `path`: Optional path prefix for the key
- `key`: The key name

**Returns:**
- `*OperationResult`: Result type will be `ResultTypeDeleted` or `ResultTypeNotFound`
- `error`: Error if operation fails

**Example:**
```go
result, err := client.Delete(
    ctx,
    nil,
    []byte("users"),
    []byte("user123"),
)
if err != nil {
    log.Printf("Delete failed: %v", err)
}
if result.Type == zencached.ResultTypeDeleted {
    fmt.Println("Key deleted successfully")
}
```

## Cluster Operations

Cluster operations perform the same operation on all nodes in the cluster.

### ClusterSet

```go
func (z *Zencached) ClusterSet(
    ctx context.Context,
    path []byte,
    key []byte,
    value []byte,
    ttl uint64,
) ([]*OperationResult, []error)
```

Sets a key-value pair on all connected nodes.

**Parameters:**
- `ctx`: Context for timeout/cancellation
- `path`: Optional path prefix for the key
- `key`: The key name
- `value`: The value to store
- `ttl`: Time-to-live in seconds

**Returns:**
- `[]*OperationResult`: Results from each node
- `[]error`: Errors from each node (can be partial failures)

**Example:**
```go
results, errs := client.ClusterSet(
    ctx,
    []byte("config"),
    []byte("app_settings"),
    []byte(`{"debug":true}`),
    86400,  // 24 hours
)

for i, result := range results {
    if errs[i] != nil {
        fmt.Printf("Node %d: Error: %v\n", i, errs[i])
    } else {
        fmt.Printf("Node %d: Success (%v)\n", i, result.Type)
    }
}
```

### ClusterAdd

```go
func (z *Zencached) ClusterAdd(
    ctx context.Context,
    path []byte,
    key []byte,
    value []byte,
    ttl uint64,
) ([]*OperationResult, []error)
```

Adds a key-value pair on all connected nodes (only if key doesn't exist).

**Parameters:**
- Same as ClusterSet

**Returns:**
- `[]*OperationResult`: Results from each node
- `[]error`: Errors from each node

### ClusterGet

```go
func (z *Zencached) ClusterGet(
    ctx context.Context,
    path []byte,
    key []byte,
) ([]*OperationResult, []error)
```

Retrieves a key from all connected nodes. Useful for replicated data.

**Parameters:**
- `ctx`: Context for timeout/cancellation
- `path`: Optional path prefix for the key
- `key`: The key name

**Returns:**
- `[]*OperationResult`: Results from each node
- `[]error`: Errors from each node

**Example:**
```go
results, errs := client.ClusterGet(
    ctx,
    []byte("replicated"),
    []byte("data_key"),
)

// Check results from all nodes
for i, result := range results {
    if errs[i] == nil && result.Type == zencached.ResultTypeFound {
        fmt.Printf("Node %d value: %s\n", i, string(result.Data))
    }
}
```

### ClusterDelete

```go
func (z *Zencached) ClusterDelete(
    ctx context.Context,
    path []byte,
    key []byte,
) ([]*OperationResult, []error)
```

Deletes a key from all connected nodes.

**Parameters:**
- `ctx`: Context for timeout/cancellation
- `path`: Optional path prefix for the key
- `key`: The key name

**Returns:**
- `[]*OperationResult`: Results from each node
- `[]error`: Errors from each node

## Node Management

### Rebalance

```go
func (z *Zencached) Rebalance()
```

Triggers node discovery and rebalancing. Uses the `NodeListFunction` if provided, otherwise uses static node configuration.

**Example:**
```go
// Manual rebalancing
client.Rebalance()

// Or enable automatic rebalancing on disconnection
config.RebalanceOnDisconnection = true
```

### GetConnectedNodes

```go
func (z *Zencached) GetConnectedNodes() []Node
```

Returns the list of currently connected nodes.

**Returns:**
- `[]Node`: Slice of connected nodes

**Example:**
```go
nodes := client.GetConnectedNodes()
fmt.Printf("Connected to %d nodes\n", len(nodes))
for _, node := range nodes {
    fmt.Printf("  - %s:%d\n", node.Host, node.Port)
}
```

### Shutdown

```go
func (z *Zencached) Shutdown()
```

Gracefully shuts down all connections and stops background workers. Always call this before exiting.

**Example:**
```go
defer client.Shutdown()
```

## Data Types

### Configuration

```go
type Configuration struct {
    Nodes                         []Node
    NumConnectionsPerNode         int
    CommandExecutionBufferSize    uint32
    NumNodeListRetries            int
    RebalanceOnDisconnection      bool
    ZencachedMetricsCollector     ZencachedMetricsCollector
    NodeListFunction              GetNodeList
    NodeListRetryTimeout          time.Duration
    CompressionType               CompressionType
    CompressionLevel              int
    TelnetConfiguration           *TelnetConfiguration
}
```

### Node

```go
type Node struct {
    Host string  // Hostname or IP
    Port int     // Port number
}
```

### OperationResult

```go
type OperationResult struct {
    Type ResultType  // Result type (Stored, NotStored, Found, NotFound, etc.)
    Data []byte      // Value data (for Get operations)
    Node Node        // Node where operation was performed
}
```

### ResultType

```go
type ResultType int

const (
    ResultTypeStored    ResultType = iota
    ResultTypeNotStored
    ResultTypeFound
    ResultTypeNotFound
    ResultTypeDeleted
)
```

### CompressionType

```go
type CompressionType int

const (
    CompressionTypeNone CompressionType = iota
    CompressionTypeZSTD
)
```

## Interfaces

### IZencached

Main interface for Zencached operations:

```go
type IZencached interface {
    Shutdown()
    Set(ctx context.Context, routerHash, path, key, value []byte, ttl uint64) (*OperationResult, error)
    Add(ctx context.Context, routerHash, path, key, value []byte, ttl uint64) (*OperationResult, error)
    Get(ctx context.Context, routerHash, path, key []byte) (*OperationResult, error)
    Delete(ctx context.Context, routerHash, path, key []byte) (*OperationResult, error)
    ClusterSet(ctx context.Context, path, key, value []byte, ttl uint64) ([]*OperationResult, []error)
    ClusterAdd(ctx context.Context, path, key, value []byte, ttl uint64) ([]*OperationResult, []error)
    ClusterGet(ctx context.Context, path, key []byte) ([]*OperationResult, []error)
    ClusterDelete(ctx context.Context, path, key []byte) ([]*OperationResult, []error)
    Rebalance()
    GetConnectedNodes() []Node
}
```

### ZencachedMetricsCollector

Metrics collection interface:

```go
type ZencachedMetricsCollector interface {
    CommandExecutionElapsedTime(node string, operation, path, key []byte, elapsedTime int64)
    CommandExecution(node string, operation, path, key []byte)
    CommandExecutionError(node string, operation, path, key []byte, err error)
    CacheMissEvent(node string, operation, path, key []byte)
    CacheHitEvent(node string, operation, path, key []byte)
    NodeRebalanceEvent(numNodes int)
    NodeListingEvent(numNodes int)
    NodeListingError()
    NodeListingElapsedTime(elapsedTime int64)
    NodeRebalanceElapsedTime(elapsedTime int64)
}
```

### ZError

Error interface with error codes:

```go
type ZError interface {
    error
    Code() ErrorType
    String() string
}
```

## Common Patterns

### With Timeout Context

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := client.Get(ctx, nil, []byte("path"), []byte("key"))
```

### Error Handling

```go
result, err := client.Get(ctx, nil, []byte("path"), []byte("key"))
if err != nil {
    if ze, ok := err.(zencached.ZError); ok {
        fmt.Printf("Error: %s (code: %v)\n", ze.String(), ze.Code())
    }
    return
}
```

### Deferred Shutdown

```go
client, err := zencached.New(config)
if err != nil {
    log.Fatal(err)
}
defer client.Shutdown()

// Use client...
```
