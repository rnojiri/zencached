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
	configuration            *Configuration
	logger                   *logh.ContextualLogger
	shuttingDown             atomic.Bool
	rebalanceLock            atomic.Bool
	notAvailable             atomic.Bool
	noResourcesAvailable     atomic.Bool
	nodeWorkerArray          []*nodeWorkers
	connectedNodes           []Node
	rebalanceChannel         chan struct{}
	closeTimedMetricsChannel chan struct{}
	metricsEnabled           bool
	compressionEnabled       bool
	dataCompressor           DataCompressor
}

// New - creates a new instance
func New(configuration *Configuration) (*Zencached, error) {

	configuration.setDefaults()

	z := &Zencached{
		nodeWorkerArray:          nil,
		configuration:            configuration,
		logger:                   logh.CreateContextualLogger("pkg", "zencached"),
		rebalanceChannel:         make(chan struct{}, 1),
		closeTimedMetricsChannel: make(chan struct{}, 1),
		metricsEnabled:           !interfaceIsNil(configuration.ZencachedMetricsCollector),
		dataCompressor:           nil,
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

	if z.metricsEnabled && !z.configuration.DisableTimedMetrics {
		go z.sendTimedMetrics()
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

	var nodes []Node

	if !z.metricsEnabled {

		nodes = z.tryListNodes()

	} else {

		start := time.Now()
		nodes = z.tryListNodes()
		z.configuration.ZencachedMetricsCollector.NodeListingElapsedTime(time.Since(start).Nanoseconds())
		z.configuration.ZencachedMetricsCollector.NodeListingEvent(len(nodes))
	}

	nodeTelnetConns := make([]*nodeWorkers, 0)
	connectedNodes := []Node{}
	connectedNodeWorkerMap := map[string]*nodeWorkers{}

	for i, node := range z.connectedNodes {
		connectedNodeWorkerMap[node.String()] = z.nodeWorkerArray[i]
	}

	for _, node := range nodes {

		nodeKey := node.String()

		if nw, exists := connectedNodeWorkerMap[nodeKey]; exists {

			telnetConn, err := nw.NewTelnetFromNode()

			if !nw.connected.Load() {

				if err != nil {
					continue
				}

				nw.connected.Store(true)

				connectedNodeWorkerMap[nodeKey] = nw

			} else {

				if err != nil {
					nw.connected.Store(false)

					connectedNodeWorkerMap[nodeKey] = nw
					continue
				}
			}

			telnetConn.Close()

			nodeTelnetConns = append(nodeTelnetConns, nw)
			connectedNodes = append(connectedNodes, node)

			continue
		}

		nw, err := z.createNodeWorker(node, z.rebalanceChannel)
		if err != nil {
			continue
		}

		nodeTelnetConns = append(nodeTelnetConns, nw)
		connectedNodes = append(connectedNodes, node)
		connectedNodeWorkerMap[nodeKey] = nw
	}

	for _, nw := range connectedNodeWorkerMap {

		if !nw.connected.Load() {
			nw.terminate()
		}
	}

	z.nodeWorkerArray = nodeTelnetConns
	z.connectedNodes = connectedNodes

	sort.Sort(nodeWorkersByNodeName(z.nodeWorkerArray))
	sort.Sort(nodeByName(z.connectedNodes))

	if logh.InfoEnabled {
		z.logger.Info().Msg("rebalancing finished")
	}

	if len(z.connectedNodes) == 0 {

		if logh.WarnEnabled {
			z.logger.Warn().Msg("no available nodes found to connect")
		}

		z.notAvailable.Store(true)

		go func() {
			<-time.After(z.configuration.NodeListRetryTimeout)
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

	z.closeTimedMetricsChannel <- struct{}{}

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

func (z *Zencached) sendTimedMetrics() {

	ticker := time.NewTicker(z.configuration.TimedMetricsPeriod)

	for {

		select {

		case <-ticker.C:
			z.SendTimedMetrics()

		case <-z.closeTimedMetricsChannel:
			if logh.InfoEnabled {
				z.logger.Info().Msg("terminating timed metrics loop...")
			}
			ticker.Stop()
			return

		default:
			<-time.After(z.configuration.TimedMetricsPeriod / 2)
		}
	}
}

func (z *Zencached) SendTimedMetrics() {

	if len(z.nodeWorkerArray) > 0 {

		if logh.DebugEnabled {
			z.logger.Debug().Msg("sending timed metrics")
		}

		for _, nw := range z.nodeWorkerArray {
			nw.sendNodeTimedMetrics()
		}
	}
}
