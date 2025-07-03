package zencached

import (
	"sort"
	"sync/atomic"
	"time"

	"github.com/rnojiri/logh"
)

//
// The memcached client main structure.
// @author rnojiri
//

// Zencached - declares the main structure
type Zencached struct {
	configuration      *Configuration
	logger             *logh.ContextualLogger
	shuttingDown       atomic.Bool
	rebalanceLock      atomic.Bool
	notAvailable       atomic.Bool
	nodeWorkerArray    []*nodeWorkers
	connectedNodes     []Node
	rebalanceChannel   chan struct{}
	metricsEnabled     bool
	compressionEnabled bool
	dataCompressor     DataCompressor
}

// New - creates a new instance
func New(configuration *Configuration) (*Zencached, error) {

	configuration.setDefaults()

	z := &Zencached{
		nodeWorkerArray:  nil,
		configuration:    configuration,
		logger:           logh.CreateContextualLogger("pkg", "zencached"),
		rebalanceChannel: make(chan struct{}, 1),
		metricsEnabled:   !interfaceIsNil(configuration.ZencachedMetricsCollector),
		dataCompressor:   nil,
	}

	if configuration.CompressionConfiguration.CompressionType != CompressionTypeNone {

		var err error

		z.dataCompressor, err = NewDataCompressor(
			configuration.CompressionConfiguration.CompressionType,
			configuration.CompressionConfiguration.CompressionLevel,
		)

		z.compressionEnabled = true

		if err != nil {
			return nil, err
		}
	}

	z.Rebalance()

	go z.rebalanceWorker()

	return z, nil
}

func (z *Zencached) tryListNodes() []Node {

	if z.configuration.NodeListFunction == nil {
		return z.configuration.Nodes
	}

	for i := 0; i < z.configuration.NumNodeListRetries; i++ {

		nodes, err := z.configuration.NodeListFunction()
		if err != nil {

			if z.metricsEnabled {
				z.configuration.ZencachedMetricsCollector.NodeListingError()
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

	if z.rebalanceLock.Load() {
		if logh.WarnEnabled {
			z.logger.Warn().Msg("rebalancing already in progress...")
		}
		return
	}

	z.rebalanceLock.Store(true)
	defer z.rebalanceLock.Store(false)

	var discoveredNodes []Node

	if !z.metricsEnabled {

		discoveredNodes = z.tryListNodes()

	} else {

		start := time.Now()
		discoveredNodes = z.tryListNodes()
		z.configuration.ZencachedMetricsCollector.NodeListingElapsedTime(time.Since(start).Nanoseconds())
		z.configuration.ZencachedMetricsCollector.NodeListingEvent(len(discoveredNodes))
	}

	nodeTelnetConns := make([]*nodeWorkers, 0)
	connectedNodes := []Node{}
	existingNodeWorkerMap := map[string]*nodeWorkers{}
	var err error

	for i, node := range z.connectedNodes {
		existingNodeWorkerMap[node.String()] = z.nodeWorkerArray[i]
	}

	for _, discoveredNode := range discoveredNodes {

		nodeKey := discoveredNode.String()

		nw, exists := existingNodeWorkerMap[nodeKey]
		if exists {

			if nw.connected.Load() {
				z.logger.Info().Msgf("node is connected: %s", nodeKey)
				nodeTelnetConns = append(nodeTelnetConns, nw)
				connectedNodes = append(connectedNodes, discoveredNode)
				continue
			}

			nw.terminate()
		}

		nw, err = z.createNodeWorker(discoveredNode, z.rebalanceChannel)
		if err != nil {
			if logh.ErrorEnabled {
				z.logger.Error().Err(err).Send()
			}
			continue
		}

		nodeTelnetConns = append(nodeTelnetConns, nw)
		connectedNodes = append(connectedNodes, discoveredNode)
		existingNodeWorkerMap[nodeKey] = nw
	}

	z.nodeWorkerArray = nodeTelnetConns
	z.connectedNodes = connectedNodes

	sort.Sort(nodeWorkersByNodeName(z.nodeWorkerArray))
	sort.Sort(nodeByName(z.connectedNodes))

	if logh.InfoEnabled {
		z.logger.Info().Msgf("rebalancing finished with %d connected nodes", len(nodeTelnetConns))
	}

	if len(z.connectedNodes) == 0 {

		if logh.WarnEnabled {
			z.logger.Warn().Msg("no available nodes found to connect")
		}

		z.notAvailable.Store(true)

		<-time.After(z.configuration.NodeListRetryTimeout)

		go func() {
			z.Rebalance()
		}()
	} else {
		z.notAvailable.Store(false)
	}
}

// Rebalance - rebalance all nodes
func (z *Zencached) Rebalance() {

	if !z.metricsEnabled {
		z.rebalance()
		return
	}

	start := time.Now()
	z.rebalance()
	z.configuration.ZencachedMetricsCollector.NodeRebalanceElapsedTime(time.Since(start).Nanoseconds())
	z.configuration.ZencachedMetricsCollector.NodeRebalanceEvent(len(z.connectedNodes))
}

func (z *Zencached) rebalanceWorker() {

	for range z.rebalanceChannel {

		if logh.DebugEnabled {
			z.logger.Debug().Msg("rebalance request received")
		}

		z.Rebalance()
	}

	if logh.DebugEnabled {
		z.logger.Debug().Msg("rebalance channel closed")
	}
}

// Shutdown - closes all connections
func (z *Zencached) Shutdown() {

	if z.shuttingDown.Load() {
		if logh.InfoEnabled {
			z.logger.Info().Msg("already shutting down...")
		}
		return
	}

	if logh.InfoEnabled {
		z.logger.Info().Msg("shutting down...")
	}

	z.shuttingDown.Store(true)

	for ni, nw := range z.nodeWorkerArray {

		if logh.InfoEnabled {
			z.logger.Info().Msgf("closing node connections from index: %d", ni)
		}

		nw.terminate()
	}

	close(z.rebalanceChannel)
}

// GetConnectedNodes - returns the currently connected nodes
func (z *Zencached) GetConnectedNodes() []Node {

	c := make([]Node, len(z.connectedNodes))
	copy(c, z.connectedNodes)

	return c
}
