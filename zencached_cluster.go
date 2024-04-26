package zencached

import "math/rand"

//
// Functions to distribute a key to all the cluster.
// author: rnojiri
//

// ClusterStore - performs an full operation operation
func (z *Zencached) ClusterStore(cmd MemcachedCommand, path, key, value []byte, ttl uint64) ([]bool, []error) {

	stored := make([]bool, z.numNodeTelnetConns)
	errors := make([]error, z.numNodeTelnetConns)

	for i := 0; i < z.numNodeTelnetConns; i++ {

		telnetConn := z.GetTelnetConnByNodeIndex(i)
		defer z.ReturnTelnetConnection(telnetConn, i)

		stored[i], errors[i] = z.baseStore(telnetConn, cmd, path, key, value, ttl)
	}

	return stored, errors
}

// ClusterGet - returns a full replicated key stored in the cluster
func (z *Zencached) ClusterGet(path, key []byte) ([]byte, bool, error) {

	index := rand.Intn(z.numNodeTelnetConns)

	telnetConn := z.GetTelnetConnByNodeIndex(index)
	defer z.ReturnTelnetConnection(telnetConn, index)

	return z.baseGet(telnetConn, path, key)
}

// ClusterDelete - deletes a key from all cluster nodes
func (z *Zencached) ClusterDelete(path, key []byte) ([]bool, []error) {

	deleted := make([]bool, z.numNodeTelnetConns)
	errors := make([]error, z.numNodeTelnetConns)

	for i := 0; i < z.numNodeTelnetConns; i++ {

		telnetConn := z.GetTelnetConnByNodeIndex(i)
		defer z.ReturnTelnetConnection(telnetConn, i)

		deleted[i], errors[i] = z.baseDelete(telnetConn, path, key)
	}

	return deleted, errors
}
