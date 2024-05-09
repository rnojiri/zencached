package zencached

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rnojiri/logh"
)

type nodeWorkers struct {
	connected uint32
	node      Node
	channel   chan *Telnet
}

func (z *Zencached) findAvailableTelnetConnection(channel chan *Telnet) (telnetConn *Telnet, err error) {

	wc := sync.WaitGroup{}
	wc.Add(1)

	go func() {

		start := time.Now()
		defer wc.Done()
		var open bool

		for {
			select {
			case telnetConn, open = <-channel:
				if !open {
					if logh.DebugEnabled {
						z.logger.Debug().Msg("telnet connection channel is closed")
					}

					err = ErrTelnetConnectionIsClosed
					telnetConn = nil
				}

				return

			default:
				if time.Since(start) > z.configuration.MaxWaitForConnection {
					telnetConn = nil
					err = ErrNoAvailableConnections
					return
				}

				<-time.After(z.configuration.MaxWaitForConnection)
			}
		}
	}()

	wc.Wait()

	return
}

func (z *Zencached) getAvailableTelnetConnection(index int) (*Telnet, error) {

	if atomic.LoadUint32(&z.nodeWorkerArray[index].connected) == 0 {
		return nil, ErrTelnetConnectionIsClosed
	}

	return z.findAvailableTelnetConnection(z.nodeWorkerArray[index].channel)
}

// GetTelnetConnByNodeIndex - returns a telnet connection by node index
func (z *Zencached) GetTelnetConnByNodeIndex(index int) (telnetConn *Telnet, err error) {

	if z.configuration.MetricsCollector == nil {

		telnetConn, err = z.getAvailableTelnetConnection(index)

	} else {

		start := time.Now()

		telnetConn, err = z.getAvailableTelnetConnection(index)

		elapsedTime := time.Since(start)

		z.configuration.MetricsCollector.NodeDistributionEvent(telnetConn.GetNodeHost())
		z.configuration.MetricsCollector.NodeConnectionAvailabilityTime(telnetConn.GetNodeHost(), elapsedTime.Nanoseconds())
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

	z.nodeWorkerArray[index].channel <- telnetConn
}

// extractValue - extracts a value from the response
func (z *Zencached) extractValue(response []byte) (start, end int, err error) {

	start = -1
	end = -1

	for i := 0; i < len(response); i++ {
		if start == -1 && response[i] == lineBreaksN {
			start = i + 1
		} else if start >= 0 && response[i] == lineBreaksR {
			end = i
			break
		}
	}

	if start == -1 {
		err = fmt.Errorf("no value found")
	}

	if end == -1 {
		end = len(response) - 1
	}

	return
}

func (z *Zencached) sendAndReadResponse(
	telnetConn *Telnet,
	beginResponseSet, endResponseSet memcachedResponseSet,
	renderedCmd []byte,
) (exists bool, response []byte, err error) {

	err = telnetConn.Send(renderedCmd)
	if err == nil {
		exists, response, err = z.checkResponse(telnetConn, beginResponseSet, endResponseSet)
	}

	return
}

func (z *Zencached) sendAndReadResponseWrapper(
	telnetConn *Telnet,
	cmd MemcachedCommand,
	beginResponseSet, endResponseSet memcachedResponseSet,
	renderedCmd, path, key []byte,
	forceCacheMissMetric bool,
) (exists bool, response []byte, err error) {

	if z.configuration.MetricsCollector == nil {

		exists, response, err = z.sendAndReadResponse(telnetConn, beginResponseSet, endResponseSet, renderedCmd)

	} else {

		z.configuration.MetricsCollector.CommandExecution(telnetConn.GetNodeHost(), cmd, path, key)

		start := time.Now()

		exists, response, err = z.sendAndReadResponse(telnetConn, beginResponseSet, endResponseSet, renderedCmd)

		elapsedTime := time.Since(start)

		z.configuration.MetricsCollector.CommandExecutionElapsedTime(
			telnetConn.GetNodeHost(),
			cmd,
			path, key,
			elapsedTime.Nanoseconds(),
		)

		if exists && !forceCacheMissMetric {
			z.configuration.MetricsCollector.CacheHitEvent(telnetConn.GetNodeHost(), cmd, path, key)
		} else {
			z.configuration.MetricsCollector.CacheMissEvent(telnetConn.GetNodeHost(), cmd, path, key)
		}

		if err != nil {

			z.configuration.MetricsCollector.CommandExecutionError(
				telnetConn.GetNodeHost(),
				cmd,
				path, key,
				err,
			)
		}
	}

	if errors.Is(err, ErrNoAvailableConnections) || errors.Is(err, ErrMaxReconnectionsReached) || errors.Is(err, ErrMemcachedNoResponse) {

		if z.configuration.RebalanceOnDisconnection {
			z.setNodeAsDisconnected(telnetConn)
			go z.Rebalance()
		}
	}

	return
}
