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

	numNodes := len(z.nodeWorkerArray)

	stored := make([]bool, numNodes)
	errors := make([]error, numNodes)

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		stored[i], errors[i] = z.baseStore(nw, cmd, path, key, value, ttl)
	}

	return stored, errors
}

// ClusterGet - returns a full replicated key stored in the cluster
func (z *Zencached) ClusterGet(path, key []byte) ([][]byte, []bool, []error) {

	numNodes := len(z.nodeWorkerArray)

	values := make([][]byte, numNodes)
	exists := make([]bool, numNodes)
	errors := make([]error, numNodes)

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		values[i], exists[i], errors[i] = z.baseGet(nw, path, key)
	}

	return values, exists, errors
}

// ClusterDelete - deletes a key from all cluster nodes
func (z *Zencached) ClusterDelete(path, key []byte) ([]bool, []error) {

	numNodes := len(z.nodeWorkerArray)

	deleted := make([]bool, numNodes)
	errors := make([]error, numNodes)

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		deleted[i], errors[i] = z.baseDelete(nw, path, key)
	}

	return deleted, errors
}
