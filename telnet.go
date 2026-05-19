package zencached

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rnojiri/logh"
)

//
// A persistent telnet connection to the memcached.
// @author rnojiri
//

type operation string

const (
	read  operation = "read"
	write operation = "write"
)

// Node - a memcached node
type Node struct {

	// Host - the server's hostname
	Host string

	// Port - the server's port
	Port int
}

// String - returns a string under the format "host:port"
func (n Node) String() string {

	return fmt.Sprintf("%s:%d", n.Host, n.Port)
}

type nodeByName []Node

func (u nodeByName) Len() int {

	return len(u)
}

func (u nodeByName) Swap(i, j int) {

	u[i], u[j] = u[j], u[i]
}

func (u nodeByName) Less(i, j int) bool {

	return u[i].Host < u[j].Host
}

// Telnet - the telnet structure
type Telnet struct {
	address              *net.TCPAddr
	connection           *net.TCPConn
	logger               *logh.ContextualLogger
	configuration        TelnetConfiguration
	node                 Node
	metricsEnabled       bool
	onFailure            atomic.Bool
	connCheckTicker      *time.Ticker
	connCheckEndChan     chan struct{}
	disconnectionChannel chan<- struct{}
}

func interfaceIsNil(subject interface{}) bool {

	v := reflect.ValueOf(subject)

	return !v.IsValid() || v.IsZero() || v.IsNil()
}

// NewTelnet - creates a new telnet connection
func NewTelnet(node Node, configuration TelnetConfiguration, disconnectionChannel chan<- struct{}) (*Telnet, error) {

	configuration.setDefaults()

	if len(strings.TrimSpace(node.Host)) == 0 {
		return nil, fmt.Errorf("empty server host configured")
	}

	if node.Port <= 0 {
		return nil, fmt.Errorf("invalid server port configured")
	}

	t := &Telnet{
		logger:               logh.CreateContextualLogger("pkg", "zencached/telnet"),
		configuration:        configuration,
		node:                 node,
		metricsEnabled:       !interfaceIsNil(configuration.TelnetMetricsCollector),
		onFailure:            atomic.Bool{},
		connCheckTicker:      time.NewTicker(configuration.ConnectionCheckTimeout),
		connCheckEndChan:     make(chan struct{}),
		disconnectionChannel: disconnectionChannel,
	}

	return t, nil
}

func (t *Telnet) checkConnection() {

	for {
		select {
		case <-t.connCheckEndChan:

			t.connCheckTicker.Stop()

			if logh.InfoEnabled {
				t.logger.Info().Msg("connection check terminated")
			}

			return

		case tickerTime := <-t.connCheckTicker.C:

			if t.configuration.EnableTracerLogs && logh.DebugEnabled {
				t.logger.Debug().Msgf("executing connection check: %s", tickerTime.Format(time.RFC3339))
			}

			if t.onFailure.Load() {
				if logh.WarnEnabled {
					t.logger.Warn().Msg("connection is on failure")
				}
				continue
			}

			_, address, err := t.resolveServerAddress()
			if err != nil {
				t.reportDisconnection()
				if logh.ErrorEnabled {
					t.logger.Error().Msgf("error resolving telnet connection address: %s", t.node.String())
				}
				return
			}

			if !t.address.IP.Equal(address.IP) {
				t.reportDisconnection()
				if logh.ErrorEnabled {
					t.logger.Error().Msgf("telnet connection ip changed: %s -> %s", t.address.IP.String(), address.IP.String())
				}
				return
			}

			connection, err := net.DialTCP("tcp", nil, address)
			if err != nil {
				t.reportDisconnection()
				if logh.ErrorEnabled {
					t.logger.Error().Msgf("error testing telnet connection to ip address: %s", address.IP.String())
				}
				return
			}

			err = connection.Close()
			if err != nil {
				if logh.ErrorEnabled {
					t.logger.Error().Msgf("error closing the test telnet connection to ip address: %s", address.IP.String())
				}
			}

			if t.configuration.EnableTracerLogs && logh.DebugEnabled {
				t.logger.Debug().Msgf("connection tested with success: %s", address.IP.String())
			}

		default:

			<-time.After(t.configuration.ConnectionCheckIdleWait)
		}
	}
}

// resolveServerAddress - configures the server address
func (t *Telnet) resolveServerAddress() (string, *net.TCPAddr, error) {

	hostAndPort := t.node.String()

	if t.configuration.EnableTracerLogs && logh.DebugEnabled {
		t.logger.Debug().Msgf("resolving address: %s", hostAndPort)
	}

	address, err := net.ResolveTCPAddr("tcp", hostAndPort)
	if err != nil {
		if logh.ErrorEnabled {
			t.logger.Error().Err(err).Msgf("error resolving address: %s", hostAndPort)
		}
		return "", nil, err
	}

	return hostAndPort, address, nil
}

// Connect - try to Connect the telnet server
func (t *Telnet) Connect() error {

	return t.connect(true)
}

func (t *Telnet) connect(startCheck bool) error {

	if logh.DebugEnabled {
		t.logger.Debug().Msgf("trying to connect to host: %s:%d", t.node.Host, t.node.Port)
	}

	var hostPort string
	var err error

	if !t.metricsEnabled {

		hostPort, t.address, err = t.resolveServerAddress()

	} else {

		start := time.Now()
		hostPort, t.address, err = t.resolveServerAddress()
		t.configuration.TelnetMetricsCollector.ResolveAddressElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
	}

	if err != nil {
		return err
	}

	if !t.metricsEnabled {

		err = t.dial()

	} else {

		start := time.Now()
		err = t.dial()
		t.configuration.TelnetMetricsCollector.DialElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
	}

	if err != nil {
		return err
	}

	if logh.InfoEnabled {
		t.logger.Info().Msgf("connected to host: %s", hostPort)
	}

	if startCheck {
		go t.checkConnection()
	}

	return nil
}

func (t *Telnet) reconnect() error {

	if logh.InfoEnabled {
		t.logger.Info().Msg("reconnecting...")
	}

	t.close(false)

	err := t.connect(false)
	if err != nil {
		return ErrTelnetConnectionIsClosed
	}

	return nil
}

// dial - connects the telnet client
func (t *Telnet) dial() error {

	var err error
	t.connection, err = net.DialTCP("tcp", nil, t.address)
	if err != nil {
		if logh.ErrorEnabled {
			t.logger.Error().Err(err).Msgf("error connecting to address: %s", t.address.String())
		}
		return err
	}

	return nil
}

// Close - closes the active connection
func (t *Telnet) Close() {

	t.close(true)
}

func (t *Telnet) close(endCheck bool) {

	if t.connection == nil {
		return
	}

	var err error

	if !t.metricsEnabled {

		err = t.connection.Close()

	} else {

		start := time.Now()
		err = t.connection.Close()
		t.configuration.TelnetMetricsCollector.CloseElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
	}

	if err != nil {
		if logh.ErrorEnabled {
			t.logger.Error().Msg(err.Error())
		}
	}

	if logh.InfoEnabled {
		t.logger.Info().Msg("connection closed")
	}

	t.connection = nil

	if endCheck {
		select {
		case t.connCheckEndChan <- struct{}{}:
		default:
		}
	}
}

// send - send some command to the server
func (t *Telnet) send(ctx context.Context, command ...[]byte) error {

	if t.onFailure.Load() {
		return ErrConnectionWrite
	}

	var err error

	for _, c := range command {

		if !t.metricsEnabled {

			err = t.writePayload(ctx, c)

		} else {

			start := time.Now()
			err = t.writePayload(ctx, c)
			t.configuration.TelnetMetricsCollector.WriteElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
			t.configuration.TelnetMetricsCollector.WriteDataSize(t.node.Host, len(c))
		}

		if err != nil {

			if errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			t.reportDisconnection()
			return ErrConnectionWrite
		}
	}

	return err
}

// Send - send some command to the server
func (t *Telnet) Send(ctx context.Context, command ...[]byte) error {

	var err error

	if !t.metricsEnabled {

		err = t.send(ctx, command...)

	} else {

		start := time.Now()
		err = t.send(ctx, command...)
		t.configuration.TelnetMetricsCollector.SendElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
	}

	return err
}

// read - reads the payload from the active connection
func (t *Telnet) read(ctx context.Context, responseSet TelnetResponseSet) ([]byte, ResultType, error) {

	if t.onFailure.Load() {
		return nil, ResultTypeNone, ErrConnectionRead
	}

	var err error
	fullBuffer := bytes.Buffer{}
	buffer := make([]byte, t.configuration.ReadBufferSize)
	var bytesRead, fullBytes int
	resultType := ResultTypeNone

	deadline := time.Now().Add(t.configuration.MaxReadTimeout)

	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}

mainLoop:
	for {

		err = t.connection.SetReadDeadline(deadline)
		if err != nil {
			if logh.ErrorEnabled {
				t.logger.Error().Err(err).Msg("error setting read deadline")
			}

			t.logConnectionError(err, read)

			return nil, ResultTypeError, err
		}

		bytesRead, err = t.connection.Read(buffer)
		if err != nil || bytesRead == 0 {
			break mainLoop
		}

		fullBytes += bytesRead

		bufferPart := buffer[:bytesRead]

		_, err = fullBuffer.Write(bufferPart)
		if err != nil {
			break mainLoop
		}

		for i, ts := range responseSet.ResponseSets {
			if findLastIndexOfByteSlice(bufferPart, ts) != -1 {
				resultType = responseSet.ResultTypes[i]
				break mainLoop
			}
		}
	}

	if err != nil && err != io.EOF {

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, os.ErrDeadlineExceeded) {

			err = t.reconnect()
			if err != nil {
				return nil, ResultTypeError, err
			}

			return nil, ResultTypeContextTimeout, context.DeadlineExceeded
		}

		t.reportDisconnection()
		t.logConnectionError(err, read)

		return nil, ResultTypeError, err
	}

	return fullBuffer.Bytes(), resultType, nil
}

func findLastIndexOfByteSlice(s []byte, sep []byte) int {

	if len(s) == 0 || len(sep) == 0 {
		return -1
	}

	j := -1
	lastIndexSep := len(sep) - 1

	for i := len(s) - 1; i >= 0; i-- {

		if j > -1 {

			if s[i] == sep[j] && j == 0 {
				return i
			}

			if s[i] != sep[j] {
				j = -1
				continue
			}

			j--
			continue
		}

		if s[i] == sep[lastIndexSep] {
			j = lastIndexSep - 1
		}
	}

	return -1
}

// Read - reads the payload from the active connection
func (t *Telnet) Read(ctx context.Context, responseSet TelnetResponseSet) ([]byte, ResultType, error) {

	var data []byte
	var resultType ResultType
	var err error

	if !t.metricsEnabled {

		data, resultType, err = t.read(ctx, responseSet)

	} else {

		start := time.Now()
		data, resultType, err = t.read(ctx, responseSet)
		t.configuration.TelnetMetricsCollector.ReadElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
		t.configuration.TelnetMetricsCollector.ReadDataSize(t.node.Host, len(data))
	}

	return data, resultType, err
}

// writePayload - writes the payload
func (t *Telnet) writePayload(ctx context.Context, payload []byte) error {

	if t.connection == nil {
		return ErrConnectionWrite
	}

	deadline := time.Now().Add(t.configuration.MaxWriteTimeout)

	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}

	err := t.connection.SetWriteDeadline(deadline)
	if err != nil {
		if logh.ErrorEnabled {
			t.logger.Error().Err(err).Msg("error setting write deadline")
		}
		return ErrConnectionWrite
	}

	_, err = t.connection.Write(payload)
	if err != nil {

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, os.ErrDeadlineExceeded) {

			err = t.reconnect()
			if err != nil {
				return err
			}

			return context.DeadlineExceeded
		}

		t.logConnectionError(err, write)
		return ErrConnectionWrite
	}

	return nil
}

// logConnectionError - logs the connection error
func (t *Telnet) logConnectionError(err error, op operation) {

	if err == io.EOF {
		if logh.ErrorEnabled {
			t.logger.Error().Msg(fmt.Sprintf("[%s] connection EOF received, retrying connection...", op))
		}

		return
	}

	if castedErr, ok := err.(net.Error); ok && castedErr.Timeout() {
		if logh.ErrorEnabled {
			t.logger.Error().Msg(fmt.Sprintf("[%s] connection timeout received, retrying connection...", op))
		}

		return
	}

	if logh.ErrorEnabled {
		t.logger.Error().Msg(fmt.Sprintf("[%s] error executing operation on connection: %s", op, err.Error()))
	}
}

// GetNode - returns this node
func (t *Telnet) GetNode() Node {

	return t.node
}

// GetNodeHost - returns this node host
func (t *Telnet) GetNodeHost() string {

	return t.node.Host
}

func (t *Telnet) reportDisconnection() {

	if !t.onFailure.CompareAndSwap(false, true) {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			t.logger.Error().Msgf("panic recovered in reportDisconnection: %v", r)
		}
	}()

	if t.disconnectionChannel != nil {
		t.disconnectionChannel <- struct{}{}
	}

	if logh.InfoEnabled {
		t.logger.Info().Msg("connection failure reported")
	}
}
