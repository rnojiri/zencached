package zencached

// ITelnet - the interace for the telnet implementation
type ITelnet interface {

	// Connect - try to Connect the telnet server
	Connect() error

	// Close - closes the active connection
	Close()

	// Send - send some command to the server
	Send(command ...[]byte) error

	// Read - reads the payload from the active connection
	Read(endConnInput [][]byte) ([]byte, error)

	// GetAddress - returns this node address
	GetAddress() string

	// GetHost - returns this node host
	GetHost() string

	// GetPort - returns this node port
	GetPort() int
}

var _ ITelnet = (*Telnet)(nil)

type IZencached interface {

	// Shutdown - closes all connections
	Shutdown()

	// Store - performs an storage operation
	Store(cmd MemcachedCommand, routerHash, path, key, value []byte, ttl uint64) (bool, error)

	// Get - performs a get operation
	Get(routerHash, path, key []byte) ([]byte, bool, error)

	// Delete - performs a delete operation
	Delete(routerHash, path, key []byte) (bool, error)

	// ClusterStore - performs an full operation operation
	ClusterStore(cmd MemcachedCommand, path, key, value []byte, ttl uint64) ([]bool, []error)

	// ClusterGet - returns a full replicated key stored in the cluster
	ClusterGet(path, key []byte) ([]byte, bool, error)

	// ClusterDelete - deletes a key from all cluster nodes
	ClusterDelete(path, key []byte) ([]bool, []error)
}

var _ IZencached = (*Zencached)(nil)

// MetricsCollector - the interface
type MetricsCollector interface {

	// NodeDistributionEvent - signalizes a node distribution event
	NodeDistributionEvent(node string)

	// NodeConnectionAvailabilityTime - the elapsed time waiting for an available connection in nanosecs
	NodeConnectionAvailabilityTime(node string, elapsedTime int64)

	// CommandExecutionElapsedTime - command execution elapsed time
	CommandExecutionElapsedTime(operation MemcachedCommand, node string, path, key []byte, elapsedTime int64)

	// CommandExecution - an memcached command event
	CommandExecution(operation MemcachedCommand, node string, path, key []byte)

	// CacheMissEvent - signalizes a cache miss event
	CacheMissEvent(operation MemcachedCommand, node string, path, key []byte)

	// CacheHitEvent - signalizes a cache hit event
	CacheHitEvent(operation MemcachedCommand, node string, path, key []byte)
}
