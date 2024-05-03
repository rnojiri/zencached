package zencached

//
// Functions to distribute a key to all the cluster.
// author: rnojiri
//

// ClusterSet - performs an full set operation operation
func (z *Zencached) ClusterSet(path, key, value []byte, ttl uint64) ([]bool, []error) {

	return z.clusterStore(Set, path, key, value, ttl)
}

// ClusterAdd - performs an full add operation operation
func (z *Zencached) ClusterAdd(path, key, value []byte, ttl uint64) ([]bool, []error) {

	return z.clusterStore(Add, path, key, value, ttl)
}

// clusterStore - performs an full storage operation operation
func (z *Zencached) clusterStore(cmd MemcachedCommand, path, key, value []byte, ttl uint64) ([]bool, []error) {

	stored := make([]bool, z.numNodeTelnetConns)
	errors := make([]error, z.numNodeTelnetConns)

	for i := 0; i < z.numNodeTelnetConns; i++ {

		var telnetConn *Telnet
		telnetConn, errors[i] = z.GetTelnetConnByNodeIndex(i)
		if errors[i] != nil {
			continue
		}

		defer z.ReturnTelnetConnection(telnetConn, i)

		stored[i], errors[i] = z.baseStore(telnetConn, cmd, cmd, path, key, value, ttl)
	}

	return stored, errors
}

// ClusterGet - returns a full replicated key stored in the cluster
func (z *Zencached) ClusterGet(path, key []byte) ([][]byte, []bool, []error) {

	values := make([][]byte, z.numNodeTelnetConns)
	exists := make([]bool, z.numNodeTelnetConns)
	errors := make([]error, z.numNodeTelnetConns)

	for i := 0; i < z.numNodeTelnetConns; i++ {

		var telnetConn *Telnet
		telnetConn, errors[i] = z.GetTelnetConnByNodeIndex(i)
		if errors[i] != nil {
			continue
		}

		defer z.ReturnTelnetConnection(telnetConn, i)

		values[i], exists[i], errors[i] = z.baseGet(telnetConn, path, key)
	}

	return values, exists, errors
}

// ClusterDelete - deletes a key from all cluster nodes
func (z *Zencached) ClusterDelete(path, key []byte) ([]bool, []error) {

	deleted := make([]bool, z.numNodeTelnetConns)
	errors := make([]error, z.numNodeTelnetConns)

	for i := 0; i < z.numNodeTelnetConns; i++ {

		var telnetConn *Telnet
		telnetConn, errors[i] = z.GetTelnetConnByNodeIndex(i)
		if errors[i] != nil {
			continue
		}

		defer z.ReturnTelnetConnection(telnetConn, i)

		deleted[i], errors[i] = z.baseDelete(telnetConn, path, key)
	}

	return deleted, errors
}
