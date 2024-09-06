package zencached

type IZencached interface {

	// Shutdown - closes all connections
	Shutdown()

	// Set - performs an storage set operation
	Set(routerHash, path, key, value []byte, ttl uint64) (*OperationResult, error)

	// Add - performs an storage add operation
	Add(routerHash, path, key, value []byte, ttl uint64) (*OperationResult, error)

	// Get - performs a get operation
	Get(routerHash, path, key []byte) (*OperationResult, error)

	// Delete - performs a delete operation
	Delete(routerHash, path, key []byte) (*OperationResult, error)

	// ClusterSet - performs an full storage set operation
	ClusterSet(path, key, value []byte, ttl uint64) ([]*OperationResult, []error)

	// ClusterAdd - performs an full storage add operation
	ClusterAdd(path, key, value []byte, ttl uint64) ([]*OperationResult, []error)

	// ClusterGet - returns a full replicated key stored in the cluster
	ClusterGet(path, key []byte) ([]*OperationResult, []error)

	// ClusterDelete - deletes a key from all cluster nodes
	ClusterDelete(path, key []byte) ([]*OperationResult, []error)

	// Rebalance - rebalance all nodes using the configured node listing function or the configured nodes by default
	Rebalance()

	// GetConnectedNodes - returns the currently connected nodes
	GetConnectedNodes() []Node
}

var _ IZencached = (*Zencached)(nil)

// ZencachedMetricsCollector - the interface to collect metrics from zencached
type ZencachedMetricsCollector interface {

	// CommandExecutionElapsedTime - command execution elapsed time
	CommandExecutionElapsedTime(node string, operation, path, key []byte, elapsedTime int64)

	// CommandExecution - an memcached command event
	CommandExecution(node string, operation, path, key []byte)

	// CommandExecutionError - signalizes an error executing a command (cast to the ZError interface to get extra metadata)
	CommandExecutionError(node string, operation, path, key []byte, err error)

	// CacheMissEvent - signalizes a cache miss event
	CacheMissEvent(node string, operation, path, key []byte)

	// CacheHitEvent - signalizes a cache hit event
	CacheHitEvent(node string, operation, path, key []byte)

	// NodeRebalanceEvent - signalizes a node rebalance event
	NodeRebalanceEvent(numNodes int)

	// NodeListingEvent - signalizes a node listing event
	NodeListingEvent(numNodes int)

	// NodeListingError - signalizes a node listing error
	NodeListingError()

	// NodeListingElapsedTime - signalizes a node listing elapsed time (nanoseconds)
	NodeListingElapsedTime(elapsedTime int64)

	// NodeRebalanceElapsedTime - signalizes a node rebalance event (nanoseconds)
	NodeRebalanceElapsedTime(elapsedTime int64)

	// NumResourcesChangeEvent - signalizes a node rebalance event
	NumResourcesChangeEvent(node string, numResources int)
}

// TelnetMetricsCollector - the interface to collect metrics from telnet
type TelnetMetricsCollector interface {

	// ResolveAddressElapsedTime - the time took to resolve a name address (nanoseconds)
	ResolveAddressElapsedTime(node string, elapsedTime int64)

	// DialElapsedTime - the time took to dial to a node (nanoseconds)
	DialElapsedTime(node string, elapsedTime int64)

	// CloseElapsedTime - the time took to disconnect from a node (nanoseconds)
	CloseElapsedTime(node string, elapsedTime int64)

	// SendElapsedTime - the time took to send data (full process with dial and close if needed) (nanoseconds)
	SendElapsedTime(node string, elapsedTime int64)

	// WriteElapsedTime - the time took to write data (nanoseconds)
	WriteElapsedTime(node string, elapsedTime int64)

	// ReadElapsedTime - the time took to read data (nanoseconds)
	ReadElapsedTime(node string, elapsedTime int64)

	// ReadDataSize - data read size (bytes)
	ReadDataSize(node string, sizeInBytes int)

	// WriteDataSize - data write size (bytes)
	WriteDataSize(node string, sizeInBytes int)
}

type ZError interface {
	error
	Code() ErrorType
	String() string
}
