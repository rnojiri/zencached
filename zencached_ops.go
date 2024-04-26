package zencached

import (
	"bytes"
	"fmt"
	"strconv"
	"time"
)

//
// This file has all implemented commands from memcached.
// More information here:
// https://github.com/memcached/memcached/blob/master/doc/protocol.txt
// author: rnojiri
//

// memcached commands and constants
const (
	lineBreaksR byte = '\r'
	lineBreaksN byte = '\n'
	whiteSpace  byte = ' '
	zero        byte = '0'
)

// memcached responses
var (
	doubleBreaks []byte = []byte{lineBreaksR, lineBreaksN}
	// responses
	mcrValue     []byte = []byte("VALUE") // the only prefix
	mcrStored    []byte = []byte("STORED")
	mcrNotStored []byte = []byte("NOT_STORED")
	mcrEnd       []byte = []byte("END")
	mcrNotFound  []byte = []byte("NOT_FOUND")
	mcrDeleted   []byte = []byte("DELETED")

	// response set
	mcrStoredResponseSet      [][]byte = [][]byte{mcrStored, mcrNotStored}
	mcrGetCheckResponseSet    [][]byte = [][]byte{mcrEnd}
	mcrGetCheckEndResponseSet [][]byte = [][]byte{mcrValue, mcrEnd}
	mcrDeletedResponseSet     [][]byte = [][]byte{mcrDeleted, mcrNotFound}
)

// MemcachedCommand type
type MemcachedCommand []byte

var (
	// Add - add some key if it not exists
	Add MemcachedCommand = MemcachedCommand("add")

	// Set - sets a key if it exists or not
	Set MemcachedCommand = MemcachedCommand("set")

	// Get - return a key if it exists or not
	Get MemcachedCommand = MemcachedCommand("get")

	// Delete - return a key if it exists or not
	Delete MemcachedCommand = MemcachedCommand("delete")
)

// executeSend - sends a message to memcached
func (z *Zencached) executeSend(telnetConn *Telnet, operation MemcachedCommand, renderedCmd, path, key []byte) error {

	if !z.enableMetrics {

		err := telnetConn.Send(renderedCmd)
		if err != nil {
			return err
		}

	} else {

		start := time.Now()
		err := telnetConn.Send(renderedCmd)
		if err != nil {
			return err
		}
		elapsedTime := time.Since(start)

		z.configuration.MetricsCollector.CommandExecutionElapsedTime(
			telnetConn.GetHost(),
			operation,
			path, key,
			elapsedTime.Milliseconds(),
		)
	}

	return nil
}

// checkResponse - checks the memcached response
func (z *Zencached) checkResponse(telnetConn *Telnet, checkReadSet, checkResponseSet [][]byte, operation MemcachedCommand, path, key []byte) (bool, []byte, error) {

	response, err := telnetConn.Read(checkReadSet)
	if err != nil {
		return false, nil, err
	}

	if !bytes.HasPrefix(response, checkResponseSet[0]) {
		if !bytes.Contains(response, checkResponseSet[1]) {
			return false, nil, fmt.Errorf("memcached operation error on command:\n%s", operation)
		}

		if z.enableMetrics {
			z.configuration.MetricsCollector.CacheMissEvent(telnetConn.GetHost(), operation, path, key)
		}

		return false, response, nil
	}

	if z.enableMetrics {
		z.configuration.MetricsCollector.CacheHitEvent(telnetConn.GetHost(), operation, path, key)
	}

	return true, response, nil
}

// renderStoreCmd - like Sprintf, but in bytes
func (z *Zencached) renderStoreCmd(cmd MemcachedCommand, path, key, value []byte, ttl uint64) []byte {

	length := strconv.Itoa(len(value))

	ttlStr := strconv.FormatUint(ttl, 10)

	buffer := bytes.Buffer{}
	buffer.Grow(len(cmd) + len(path) + len(key) + len(value) + len(ttlStr) + len(length) + 4 + (len(doubleBreaks) * 2) + 1)
	buffer.Write(cmd)
	buffer.WriteByte(whiteSpace)
	buffer.Write(path)
	buffer.Write(key)
	buffer.WriteByte(whiteSpace)
	buffer.WriteByte(zero)
	buffer.WriteByte(whiteSpace)
	buffer.WriteString(ttlStr)
	buffer.WriteByte(whiteSpace)
	buffer.WriteString(length)
	buffer.Write(doubleBreaks)
	buffer.Write(value)
	buffer.Write(doubleBreaks)

	return buffer.Bytes()
}

// Store - performs an storage operation
func (z *Zencached) Store(cmd MemcachedCommand, routerHash, path, key, value []byte, ttl uint64) (bool, error) {

	telnetConn, index := z.GetTelnetConnection(routerHash, path, key)
	defer z.ReturnTelnetConnection(telnetConn, index)

	return z.baseStore(telnetConn, cmd, path, key, value, ttl)
}

// baseStore - base storage function
func (z *Zencached) baseStore(telnetConn *Telnet, cmd MemcachedCommand, path, key, value []byte, ttl uint64) (bool, error) {

	if z.enableMetrics {
		z.configuration.MetricsCollector.CommandExecution(telnetConn.GetHost(), cmd, path, key)
	}

	err := z.executeSend(telnetConn, cmd, z.renderStoreCmd(cmd, path, key, value, ttl), path, key)
	if err != nil {
		return false, err
	}

	wasStored, _, err := z.checkResponse(telnetConn, mcrStoredResponseSet, mcrStoredResponseSet, cmd, path, key)
	if err != nil {
		return false, err
	}

	return wasStored, nil
}

// renderKeyOnlyCmd - like Sprintf, but in bytes
func (z *Zencached) renderKeyOnlyCmd(cmd MemcachedCommand, path, key []byte) []byte {

	buffer := bytes.Buffer{}
	buffer.Grow(len(cmd) + len(path) + len(key) + 1 + len(doubleBreaks))
	buffer.Write(cmd)
	buffer.WriteByte(whiteSpace)
	buffer.Write(path)
	buffer.Write(key)
	buffer.Write(doubleBreaks)

	return buffer.Bytes()
}

// Get - performs a get operation
func (z *Zencached) Get(routerHash, path, key []byte) ([]byte, bool, error) {

	telnetConn, index := z.GetTelnetConnection(routerHash, path, key)
	defer z.ReturnTelnetConnection(telnetConn, index)

	return z.baseGet(telnetConn, path, key)
}

// baseGet - the base get operation
func (z *Zencached) baseGet(telnetConn *Telnet, path, key []byte) ([]byte, bool, error) {

	if z.enableMetrics {
		z.configuration.MetricsCollector.CommandExecution(telnetConn.GetHost(), Get, path, key)
	}

	err := z.executeSend(telnetConn, Get, z.renderKeyOnlyCmd(Get, path, key), path, key)
	if err != nil {
		return nil, false, err
	}

	exists, response, err := z.checkResponse(telnetConn, mcrGetCheckResponseSet, mcrGetCheckEndResponseSet, Get, path, key)
	if !exists || err != nil {
		return nil, false, err
	}

	start, end, err := z.extractValue([]byte(response))
	if err != nil {
		return nil, false, err
	}

	return response[start:end], true, nil
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

// Delete - performs a delete operation
func (z *Zencached) Delete(routerHash, path, key []byte) (bool, error) {

	telnetConn, index := z.GetTelnetConnection(routerHash, path, key)
	defer z.ReturnTelnetConnection(telnetConn, index)

	return z.baseDelete(telnetConn, path, key)
}

// baseDelete - base delete operation
func (z *Zencached) baseDelete(telnetConn *Telnet, path, key []byte) (bool, error) {

	if z.enableMetrics {
		z.configuration.MetricsCollector.CommandExecution(telnetConn.GetHost(), Delete, path, key)
	}

	err := z.executeSend(telnetConn, Delete, z.renderKeyOnlyCmd(Delete, path, key), path, key)
	if err != nil {
		return false, err
	}

	exists, _, err := z.checkResponse(telnetConn, mcrDeletedResponseSet, mcrDeletedResponseSet, Delete, path, key)
	if err != nil {
		return false, err
	}

	return exists, nil
}
