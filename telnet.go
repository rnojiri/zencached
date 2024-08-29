package zencached

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
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

// TelnetConfiguration - contains the telnet connection configuration
type TelnetConfiguration struct {

	// ReconnectionTimeout - the time duration between connection retries
	ReconnectionTimeout time.Duration

	// MaxWriteTimeout - the max time duration to wait a write operation
	MaxWriteTimeout time.Duration

	// MaxReadTimeout - the max time duration to wait a read operation
	MaxReadTimeout time.Duration

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

	if tc.MaxWriteTimeout == 0 {
		tc.MaxWriteTimeout = time.Second
	}

	if tc.MaxReadTimeout == 0 {
		tc.MaxReadTimeout = time.Second
	}

	if tc.ReadBufferSize == 0 {
		tc.ReadBufferSize = 64
	}

	if tc.HostConnectionTimeout == 0 {
		tc.HostConnectionTimeout = 5 * time.Second
	}
}

// Telnet - the telnet structure
type Telnet struct {
	address        *net.TCPAddr
	connection     *net.TCPConn
	logger         *logh.ContextualLogger
	configuration  TelnetConfiguration
	node           Node
	metricsEnabled bool
}

func interfaceIsNil(subject interface{}) bool {

	v := reflect.ValueOf(subject)

	return !v.IsValid() || v.IsZero() || v.IsNil()
}

// NewTelnet - creates a new telnet connection
func NewTelnet(node Node, configuration TelnetConfiguration) (*Telnet, error) {

	configuration.setDefaults()

	if len(strings.TrimSpace(node.Host)) == 0 {
		return nil, fmt.Errorf("empty server host configured")
	}

	if node.Port <= 0 {
		return nil, fmt.Errorf("invalid server port configured")
	}

	t := &Telnet{
		logger:         logh.CreateContextualLogger("pkg", "zencached/telnet"),
		configuration:  configuration,
		node:           node,
		metricsEnabled: !interfaceIsNil(configuration.TelnetMetricsCollector),
	}

	return t, nil
}

// resolveServerAddress - configures the server address
func (t *Telnet) resolveServerAddress() (string, error) {

	hostPort := t.node.String()

	if logh.DebugEnabled {
		t.logger.Debug().Msgf("resolving address: %s", hostPort)
	}

	var err error
	t.address, err = net.ResolveTCPAddr("tcp", hostPort)
	if err != nil {
		if logh.ErrorEnabled {
			t.logger.Error().Err(err).Msgf("error resolving address: %s", hostPort)
		}
		return "", err
	}

	return hostPort, nil
}

// Connect - try to Connect the telnet server
func (t *Telnet) Connect() error {

	if logh.DebugEnabled {
		t.logger.Debug().Msgf("trying to connect to host: %s:%d", t.node.Host, t.node.Port)
	}

	var hostPort string
	var err error

	if !t.metricsEnabled {

		hostPort, err = t.resolveServerAddress()

	} else {

		start := time.Now()
		hostPort, err = t.resolveServerAddress()
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

	err = t.connection.SetDeadline(time.Now().Add(t.configuration.ReconnectionTimeout))
	if err != nil {
		if logh.ErrorEnabled {
			t.logger.Error().Err(err).Msg("error setting connection's deadline")
		}
		return err
	}

	return nil
}

// Close - closes the active connection
func (t *Telnet) Close() {

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
}

// send - send some command to the server
func (t *Telnet) send(command ...[]byte) error {

	var err error
	var wrote bool

	for _, c := range command {
	innerLoop:
		for i := 0; i < t.configuration.MaxWriteRetries; i++ {

			if !t.metricsEnabled {

				wrote = t.writePayload(c)

			} else {

				start := time.Now()
				wrote = t.writePayload(c)
				t.configuration.TelnetMetricsCollector.WriteElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
			}

			if !wrote {

				t.Close()

				err = t.Connect()
				if err != nil {
					if i+1 == t.configuration.MaxWriteRetries {
						if logh.DebugEnabled {
							t.logger.Debug().Err(err).Msg("maximum number of connections retries reached")
						}

						return fmt.Errorf("%w: %s", ErrMaxReconnectionsReached, err)
					}

					<-time.After(t.configuration.ReconnectionTimeout)
					continue
				}
			} else {
				break innerLoop
			}
		}
	}

	return err
}

// Send - send some command to the server
func (t *Telnet) Send(command ...[]byte) error {

	var err error

	if !t.metricsEnabled {

		err = t.send(command...)

	} else {

		start := time.Now()
		err = t.send(command...)
		t.configuration.TelnetMetricsCollector.SendElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
	}

	return err
}

// read - reads the payload from the active connection
func (t *Telnet) read(trs TelnetResponseSet) ([]byte, ResultType, error) {

	err := t.connection.SetReadDeadline(time.Now().Add(t.configuration.MaxReadTimeout))
	if err != nil {
		if logh.ErrorEnabled {
			t.logger.Error().Msg(fmt.Sprintf("error setting read deadline: %s", err.Error()))
		}
		return nil, ResultTypeNone, err
	}

	fullBuffer := bytes.Buffer{}
	fullBuffer.Grow(t.configuration.ReadBufferSize)

	buffer := make([]byte, t.configuration.ReadBufferSize)
	var bytesRead, fullBytes int
	growSize := 1

	responseSetIndex := 0

mainLoop:
	for {
		bytesRead, err = t.connection.Read(buffer)
		if bytesRead == 0 || err != nil {
			break mainLoop
		}

		fullBytes += bytesRead
		if fullBytes > t.configuration.ReadBufferSize*growSize {
			fullBuffer.Grow(t.configuration.ReadBufferSize)
		}

		fullBuffer.Write((buffer[0:bytesRead]))

		for j := 0; j < len(trs.ResponseSets); j++ {

			responseSetIndex = j

			if bytes.LastIndex(buffer[0:bytesRead], trs.ResponseSets[j]) != -1 {
				break mainLoop
			}
		}
	}

	if err != nil && err != io.EOF {
		t.logConnectionError(err, read)
		return nil, ResultTypeNone, err
	}

	return fullBuffer.Bytes(), trs.ResultTypes[responseSetIndex], nil
}

// Read - reads the payload from the active connection
func (t *Telnet) Read(trs TelnetResponseSet) ([]byte, ResultType, error) {

	var data []byte
	var rt ResultType
	var err error

	if !t.metricsEnabled {

		data, rt, err = t.read(trs)

	} else {

		start := time.Now()
		data, rt, err = t.read(trs)
		t.configuration.TelnetMetricsCollector.ReadElapsedTime(t.node.Host, time.Since(start).Nanoseconds())
	}

	return data, rt, err
}

// writePayload - writes the payload
func (t *Telnet) writePayload(payload []byte) bool {

	if t.connection == nil {
		return false
	}

	err := t.connection.SetWriteDeadline(time.Now().Add(t.configuration.MaxWriteTimeout))
	if err != nil {
		if logh.ErrorEnabled {
			t.logger.Error().Err(err).Msg("error setting write deadline")
		}
		return false
	}

	_, err = t.connection.Write([]byte(payload))
	if err != nil {
		t.logConnectionError(err, write)
		return false
	}

	return true
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
