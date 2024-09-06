package zencached

import "time"

// TelnetConfiguration - contains the telnet connection configuration
type TelnetConfiguration struct {

	// ReconnectionTimeout - the time duration between connection retries
	ReconnectionTimeout time.Duration

	// HostConnectionTimeout - the max time duration to wait to connect to a host
	HostConnectionTimeout time.Duration

	// MaxWriteRetries - the maximum number of write retries
	MaxWriteRetries int

	// ReadBufferSize - the size of the read buffer in bytes
	ReadBufferSize int

	// TelnetMetricsCollector - collects metrics related with telnet
	TelnetMetricsCollector TelnetMetricsCollector
}

func (tc *TelnetConfiguration) setDefaults() {

	if tc.ReconnectionTimeout == 0 {
		tc.ReconnectionTimeout = time.Second
	}

	if tc.ReadBufferSize < 8192 { // less than 8kb of read buffer is bad
		tc.ReadBufferSize = 8192
	}

	if tc.HostConnectionTimeout == 0 {
		tc.HostConnectionTimeout = 5 * time.Second
	}
}

// CompressionConfiguration - compression configuration
type CompressionConfiguration struct {
	CompressionType  CompressionType
	CompressionLevel int
}

type GetNodeList func() ([]Node, error)

// Configuration - has the main configuration
type Configuration struct {
	// Nodes - the default memcached nodes
	Nodes []Node

	// NumConnectionsPerNode - the number of connections for each memcached node
	NumConnectionsPerNode int

	// CommandExecutionBufferSize - the number of command execution jobs buffered
	CommandExecutionBufferSize uint32

	// NumNodeListRetries - the number of node listing retry after an error
	NumNodeListRetries int

	// RebalanceOnDisconnection - always rebalance aafter some disconnection
	RebalanceOnDisconnection bool

	// ZencachedMetricsCollector - a metrics interface to implement metric extraction
	ZencachedMetricsCollector ZencachedMetricsCollector

	// NodeListFunction - a custom node listing function that can be customizable
	NodeListFunction GetNodeList

	// NodeListRetryTimeout - time to wait for node listing retry after an error
	NodeListRetryTimeout time.Duration

	// TimedMetricsPeriod - send metrics after some period (if metrics are enabled)
	TimedMetricsPeriod time.Duration

	// DisableTimedMetrics - disables the automatic sending of metrics (if metrics are enabled)
	DisableTimedMetrics bool

	TelnetConfiguration

	CompressionConfiguration
}

func (c *Configuration) setDefaults() {

	if c.Nodes == nil {
		c.Nodes = []Node{}
	}

	if c.NumConnectionsPerNode == 0 {
		c.NumConnectionsPerNode = 1
	}

	if c.CommandExecutionBufferSize == 0 {
		c.CommandExecutionBufferSize = 100
	}

	if c.NumNodeListRetries == 0 {
		c.NumNodeListRetries = 1
	}

	if c.NodeListRetryTimeout == 0 {
		c.NodeListRetryTimeout = time.Second
	}

	if c.TimedMetricsPeriod == 0 {
		c.TimedMetricsPeriod = time.Minute
	}

	if !c.CompressionType.IsACompressionType() {
		c.CompressionType = CompressionTypeNone
	}

	if c.CompressionLevel < 0 || c.CompressionLevel > 10 {
		c.CompressionLevel = 5
	}
}
