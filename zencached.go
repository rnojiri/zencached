package zencached

import (
	"errors"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/rnojiri/logh"
)

//
// The memcached client main structure.
// @author rnojiri
//

type GetNodeList func() ([]Node, error)

var (
	ErrTelnetConnectionIsClosed error = errors.New("telnet connection is closed")
	ErrNoAvailableNodes         error = errors.New("there are no nodes available")
)

// Configuration - has the main configuration
type Configuration struct {
	Nodes                    []Node
	NumConnectionsPerNode    int
	NumCommandRetries        int
	MetricsCollector         MetricsCollector
	RebalanceOnDisconnection bool
	NodeListFunction         GetNodeList
	NumNodeListRetries       int
	NodeListRetryTimeout     time.Duration
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
}

// Zencached - declares the main structure
type Zencached struct {
	numNodeTelnetConns int
	configuration      *Configuration
	logger             *logh.ContextualLogger
	shuttingDown       uint32
	rebalanceLock      uint32
	notAvailable       uint32
	nodeMap            map[string]chan *Telnet
	nodeTelnetConns    []chan *Telnet
	connectedNodes     []Node
}

// New - creates a new instance
func New(configuration *Configuration) (*Zencached, error) {

	configuration.setDefaults()

	z := &Zencached{
		nodeTelnetConns:    nil,
		numNodeTelnetConns: 0,
		configuration:      configuration,
		logger:             logh.CreateContextualLogger("pkg", "zencached"),
		nodeMap:            make(map[string]chan *Telnet),
	}

	err := z.Rebalance()
	if err != nil {
		return nil, err
	}

	return z, nil
}

func (z *Zencached) createNewTelnet(node Node) (*Telnet, error) {

	telnetConn, err := NewTelnet(node, z.configuration.TelnetConfiguration)
	if err != nil {
		z.logger.Error().Str("host", node.Host).Int("port", node.Port).Err(err).Msg("error creating telnet")
		return nil, err
	}

	err = telnetConn.Connect()
	if err != nil {
		z.logger.Error().Str("host", node.Host).Int("port", node.Port).Err(err).Msg("error connecting to host")
		return nil, err
	}

	return telnetConn, nil
}

func (z *Zencached) tryGetNodes() []Node {

	if z.configuration.NodeListFunction == nil {
		return z.configuration.Nodes
	}

	for i := 0; i < z.configuration.NumNodeListRetries; i++ {

		nodes, err := z.configuration.NodeListFunction()
		if err != nil {
			z.logger.Error().Err(err).Msg("error retrieving nodes")
			<-time.After(z.configuration.NodeListRetryTimeout)
			continue
		}

		if len(nodes) == 0 {
			z.logger.Warn().Msg("no available nodes found")
			<-time.After(z.configuration.NodeListRetryTimeout)
			continue
		}

		return nodes
	}

	z.logger.Warn().Msg("node listing failed, falling back to the configured ones")

	return z.configuration.Nodes
}

// Rebalance - rebalance all nodes
func (z *Zencached) Rebalance() error {

	if atomic.LoadUint32(&z.rebalanceLock) == 1 {
		z.logger.Warn().Msg("rebalancing already in progress...")
		return nil
	}

	atomic.StoreUint32(&z.rebalanceLock, 1)
	defer atomic.StoreUint32(&z.rebalanceLock, 0)

	nodes := z.tryGetNodes()

	newNodeMap := make(map[string]chan *Telnet)
	nodeTelnetConns := make([]chan *Telnet, 0)
	connectedNodes := []Node{}

	for _, node := range nodes {

		telnetConn, err := z.createNewTelnet(node)
		if err != nil {
			continue
		}

		telnetConn.Close()

		nodeKey := fmt.Sprintf("%s:%d", node.Host, node.Port)

		if nodeChannelArray, exists := z.nodeMap[nodeKey]; exists {
			newNodeMap[nodeKey] = nodeChannelArray
			nodeTelnetConns = append(nodeTelnetConns, nodeChannelArray)
			connectedNodes = append(connectedNodes, node)
			delete(z.nodeMap, nodeKey)
			continue
		}

		channelArray := make(chan *Telnet, z.configuration.NumConnectionsPerNode)

		for c := 0; c < z.configuration.NumConnectionsPerNode; c++ {

			telnetConn, err = z.createNewTelnet(node)
			if err != nil {
				continue
			}

			channelArray <- telnetConn
		}

		nodeTelnetConns = append(nodeTelnetConns, channelArray)
		connectedNodes = append(connectedNodes, node)
		newNodeMap[nodeKey] = channelArray
	}

	for _, v := range z.nodeMap {
		z.closeNodeTelnetConnections(v)
	}

	z.nodeTelnetConns = nodeTelnetConns
	z.numNodeTelnetConns = len(nodeTelnetConns)
	z.nodeMap = newNodeMap
	z.connectedNodes = connectedNodes

	z.logger.Info().Msg("rebalancing finished")

	if len(z.connectedNodes) == 0 {
		z.logger.Warn().Msg("no available nodes found to connect, rebalancing again...")
		atomic.StoreUint32(&z.notAvailable, 1)
		<-time.After(z.configuration.NodeListRetryTimeout)
		atomic.StoreUint32(&z.rebalanceLock, 0)
		go z.Rebalance()
	} else {
		atomic.StoreUint32(&z.notAvailable, 0)
	}

	return nil
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

	for nodeIndex, nodeConns := range z.nodeTelnetConns {

		if logh.InfoEnabled {
			z.logger.Info().Msgf("closing node connections from index: %d", nodeIndex)
		}

		z.closeNodeTelnetConnections(nodeConns)
	}
}

// closeNodeTelnetConnections - closes all connections
func (z *Zencached) closeNodeTelnetConnections(nodeConns chan *Telnet) {

	for i := 0; i < z.configuration.NumConnectionsPerNode; i++ {

		if logh.DebugEnabled {
			z.logger.Debug().Msg("closing connection...")
		}

		conn := <-nodeConns
		conn.Close()

		if logh.DebugEnabled {
			z.logger.Debug().Msg("connection closed")
		}
	}

	close(nodeConns)
}

func (z Zencached) getAvailableTelnetConnection(index int) (*Telnet, error) {

	telnetConn, open := <-z.nodeTelnetConns[index]
	if !open {
		if logh.DebugEnabled {
			z.logger.Debug().Msg("telnet connection channel is closed")
		}
		return nil, ErrTelnetConnectionIsClosed
	}

	return telnetConn, nil
}

// GetTelnetConnByNodeIndex - returns a telnet connection by node index
func (z *Zencached) GetTelnetConnByNodeIndex(index int) (telnetConn *Telnet, err error) {

	if z.configuration.MetricsCollector == nil {

		telnetConn, err = z.getAvailableTelnetConnection(index)

	} else {

		start := time.Now()

		telnetConn, err = z.getAvailableTelnetConnection(index)

		elapsedTime := time.Since(start)

		z.configuration.MetricsCollector.NodeDistributionEvent(telnetConn.GetHost())
		z.configuration.MetricsCollector.NodeConnectionAvailabilityTime(telnetConn.GetHost(), elapsedTime.Nanoseconds())
	}

	return
}

// GetTelnetConnection - returns an idle telnet connection
func (z *Zencached) GetTelnetConnection(routerHash, path, key []byte) (telnetConn *Telnet, index int, err error) {

	if atomic.LoadUint32(&z.notAvailable) == 1 {
		return nil, 0, ErrNoAvailableNodes
	}

	if routerHash == nil {
		routerHash = append(path, key...)
	}

	if len(routerHash) == 0 {
		index = rand.Intn(z.numNodeTelnetConns)
	} else {
		index = int(routerHash[len(routerHash)-1]) % z.numNodeTelnetConns
	}

	telnetConn, err = z.GetTelnetConnByNodeIndex(index)

	return
}

// ReturnTelnetConnection - returns a telnet connection to the pool
func (z *Zencached) ReturnTelnetConnection(telnetConn *Telnet, index int) {

	z.nodeTelnetConns[index] <- telnetConn
}

// GetNodes - returns the currently connected nodes
func (z *Zencached) GetNodes() []Node {

	c := make([]Node, len(z.connectedNodes))
	copy(c, z.connectedNodes)

	return c
}
