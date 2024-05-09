package zencached

import (
	"sync/atomic"
	"time"

	"github.com/rnojiri/logh"
)

//
// The memcached client main structure.
// @author rnojiri
//

type GetNodeList func() ([]Node, error)

// Configuration - has the main configuration
type Configuration struct {
	// Nodes - the default memcached nodes
	Nodes []Node

	// NumConnectionsPerNode - the number of connections for each memcached node
	NumConnectionsPerNode int

	// NumNodeListRetries - the number of node listing retry after an error
	NumNodeListRetries int

	// RebalanceOnDisconnection - always rebalance aafter some disconnection
	RebalanceOnDisconnection bool

	// MetricsCollector - a metrics interface to implement metric extraction
	MetricsCollector ZencachedMetricsCollector

	// NodeListFunction - a custom node listing function that can be customizable
	NodeListFunction GetNodeList

	// NodeListRetryTimeout - time to wait for node listing retry after an error
	NodeListRetryTimeout time.Duration

	// MaxWaitForConnection - the max time duration to wait for a free telnet connection
	MaxWaitForConnection time.Duration

	// WaitTimeForConnection - the time duration to check for a free telnet connection
	WaitTimeForConnection time.Duration

	TelnetConfiguration
}

func (c *Configuration) setDefaults() {

	if c.Nodes == nil {
		c.Nodes = []Node{}
	}

	if c.NumConnectionsPerNode == 0 {
		c.NumConnectionsPerNode = 1
	}

	if c.NumNodeListRetries == 0 {
		c.NumNodeListRetries = 1
	}

	if c.NodeListRetryTimeout == 0 {
		c.NodeListRetryTimeout = time.Second
	}

	if c.MaxWaitForConnection == 0 {
		c.MaxWaitForConnection = time.Second
	}

	if c.WaitTimeForConnection == 0 {
		c.WaitTimeForConnection = 10 * time.Millisecond
	}
}

// Zencached - declares the main structure
type Zencached struct {
	numNodeTelnetConns int
	configuration      *Configuration
	logger             *logh.ContextualLogger
	shuttingDown       uint32
	rebalanceLock      uint32
	notAvailable       uint32
	nodeWorkerArray    []nodeWorkers
	connectedNodes     []Node
}

// New - creates a new instance
func New(configuration *Configuration) (*Zencached, error) {

	configuration.setDefaults()

	z := &Zencached{
		nodeWorkerArray:    nil,
		numNodeTelnetConns: 0,
		configuration:      configuration,
		logger:             logh.CreateContextualLogger("pkg", "zencached"),
	}

	z.Rebalance()

	return z, nil
}

func (z *Zencached) createNewTelnet(node Node) (*Telnet, error) {

	telnetConn, err := NewTelnet(node, z.configuration.TelnetConfiguration)
	if err != nil {
		if logh.ErrorEnabled {
			z.logger.Error().Str("host", node.Host).Int("port", node.Port).Err(err).Msg("error creating telnet")
		}

		if z.configuration.MetricsCollector != nil {
			z.configuration.MetricsCollector.NodeRebalanceError()
		}

		return nil, err
	}

	err = telnetConn.Connect()
	if err != nil {
		if logh.ErrorEnabled {
			z.logger.Error().Str("host", node.Host).Int("port", node.Port).Err(err).Msg("error connecting to host")
		}

		if z.configuration.MetricsCollector != nil {
			z.configuration.MetricsCollector.NodeRebalanceError()
		}

		return nil, err
	}

	return telnetConn, nil
}

func (z *Zencached) tryListNodes() []Node {

	if z.configuration.NodeListFunction == nil {
		return z.configuration.Nodes
	}

	for i := 0; i < z.configuration.NumNodeListRetries; i++ {

		nodes, err := z.configuration.NodeListFunction()
		if err != nil {

			if z.configuration.MetricsCollector != nil {
				z.configuration.MetricsCollector.NodeListingError()
			}

			if logh.ErrorEnabled {
				z.logger.Error().Err(err).Msg("error retrieving nodes")
			}

			<-time.After(z.configuration.NodeListRetryTimeout)
			continue
		}

		if len(nodes) == 0 {
			if logh.WarnEnabled {
				z.logger.Warn().Msg("no available nodes found")
			}
			<-time.After(z.configuration.NodeListRetryTimeout)
			continue
		}

		return nodes
	}

	if logh.WarnEnabled {
		z.logger.Warn().Msg("node listing failed, falling back to the configured ones")
	}

	return z.configuration.Nodes
}

// rebalance - rebalance all nodes
func (z *Zencached) rebalance() {

	if atomic.LoadUint32(&z.rebalanceLock) == 1 {
		if logh.WarnEnabled {
			z.logger.Warn().Msg("rebalancing already in progress...")
		}
		return
	}

	atomic.StoreUint32(&z.rebalanceLock, 1)
	defer atomic.StoreUint32(&z.rebalanceLock, 0)

	var nodes []Node

	if z.configuration.MetricsCollector == nil {

		nodes = z.tryListNodes()

	} else {

		start := time.Now()
		nodes = z.tryListNodes()
		z.configuration.MetricsCollector.NodeListingElapsedTime(time.Since(start).Nanoseconds())
		z.configuration.MetricsCollector.NodeListingEvent(len(nodes))
	}

	nodeTelnetConns := make([]nodeWorkers, 0)
	connectedNodes := []Node{}
	connectedNodeWorkerMap := map[string]nodeWorkers{}

	for i, node := range z.connectedNodes {
		connectedNodeWorkerMap[node.String()] = z.nodeWorkerArray[i]
	}

	for _, node := range nodes {

		nodeKey := node.String()

		if nw, exists := connectedNodeWorkerMap[nodeKey]; exists {

			if atomic.LoadUint32(&nw.connected) == 0 {

				telnetConn, err := z.createNewTelnet(node)
				if err != nil {
					continue
				}

				telnetConn.Close()

				atomic.StoreUint32(&nw.connected, 1)
				connectedNodeWorkerMap[nodeKey] = nw

			} else {

				telnetConn, err := z.findAvailableTelnetConnection(nw.channel)
				if err != nil {
					atomic.StoreUint32(&nw.connected, 0)
					connectedNodeWorkerMap[nodeKey] = nw
					continue
				}

				nw.channel <- telnetConn

				telnetConn.Close()
				err = telnetConn.Connect()

				if err != nil {
					atomic.StoreUint32(&nw.connected, 0)
					connectedNodeWorkerMap[nodeKey] = nw
					continue
				}
			}

			nodeTelnetConns = append(nodeTelnetConns, nw)
			connectedNodes = append(connectedNodes, node)

			continue
		}

		telnetChan := make(chan *Telnet, z.configuration.NumConnectionsPerNode)

		for c := 0; c < z.configuration.NumConnectionsPerNode; c++ {

			telnetConn, err := z.createNewTelnet(node)
			if err != nil {
				continue
			}

			telnetChan <- telnetConn
		}

		nw := nodeWorkers{
			channel:   telnetChan,
			connected: 1,
			node:      node,
		}

		nodeTelnetConns = append(nodeTelnetConns, nw)
		connectedNodes = append(connectedNodes, node)
		connectedNodeWorkerMap[nodeKey] = nw
	}

	for _, nodeChannelsValue := range connectedNodeWorkerMap {

		if atomic.LoadUint32(&nodeChannelsValue.connected) == 0 {
			z.closeNodeTelnetConnections(nodeChannelsValue)
		}
	}

	z.nodeWorkerArray = nodeTelnetConns
	z.numNodeTelnetConns = len(nodeTelnetConns)
	z.connectedNodes = connectedNodes

	if logh.InfoEnabled {
		z.logger.Info().Msg("rebalancing finished")
	}

	if len(z.connectedNodes) == 0 {
		if logh.WarnEnabled {
			z.logger.Warn().Msg("no available nodes found to connect")
		}
		atomic.StoreUint32(&z.notAvailable, 1)
	} else {
		atomic.StoreUint32(&z.notAvailable, 0)
	}
}

// Rebalance - rebalance all nodes
func (z *Zencached) Rebalance() {

	if z.configuration.MetricsCollector == nil {
		z.rebalance()
		return
	}

	start := time.Now()
	z.rebalance()
	z.configuration.MetricsCollector.NodeRebalanceElapsedTime(time.Since(start).Nanoseconds())
	z.configuration.MetricsCollector.NodeRebalanceEvent(len(z.connectedNodes))
}

// Shutdown - closes all connections
func (z *Zencached) Shutdown() {

	if atomic.LoadUint32(&z.shuttingDown) == 1 {
		if logh.InfoEnabled {
			z.logger.Info().Msg("already shutting down...")
		}
		return
	}

	if logh.InfoEnabled {
		z.logger.Info().Msg("shutting down...")
	}

	atomic.SwapUint32(&z.shuttingDown, 1)

	for nodeIndex, nodeConns := range z.nodeWorkerArray {

		if logh.InfoEnabled {
			z.logger.Info().Msgf("closing node connections from index: %d", nodeIndex)
		}

		z.closeNodeTelnetConnections(nodeConns)
	}
}

// closeNodeTelnetConnections - closes all connections
func (z *Zencached) closeNodeTelnetConnections(nw nodeWorkers) {

	for i := 0; i < z.configuration.NumConnectionsPerNode; i++ {

		if logh.DebugEnabled {
			z.logger.Debug().Msg("closing connection...")
		}

		conn := <-nw.channel
		conn.Close()

		if logh.DebugEnabled {
			z.logger.Debug().Msg("connection closed")
		}
	}

	close(nw.channel)
}

// GetConnectedNodes - returns the currently connected nodes
func (z *Zencached) GetConnectedNodes() []Node {

	c := make([]Node, len(z.connectedNodes))
	copy(c, z.connectedNodes)

	return c
}

func (z *Zencached) setNodeAsDisconnected(telnetConn *Telnet) {

	for i, nw := range z.nodeWorkerArray {

		if nw.node.String() == telnetConn.node.String() {

			atomic.StoreUint32(&z.nodeWorkerArray[i].connected, 0)

			if logh.WarnEnabled {
				z.logger.Warn().Msgf("node is marked for disconnection: %s", telnetConn.node.String())
			}

			break
		}
	}
}
