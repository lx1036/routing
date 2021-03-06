package main

import (
	"fmt"

	"github.com/tiglabs/raft"
)

// ClusterResolver implement raft Resolver
type nodeResolver struct {
	cfg *Config
}

// NodeAddress get node address
func (r *nodeResolver) NodeAddress(nodeID uint64, stype raft.SocketType) (addr string, err error) {
	node := r.cfg.FindClusterNode(nodeID)
	if node == nil {
		return "", fmt.Errorf("could not find node(%v) in cluster config:\n: %v", nodeID, r.cfg.String())
	}
	switch stype {
	case raft.HeartBeat:
		return fmt.Sprintf("%s:%d", node.Host, node.HeartbeatPort), nil
	case raft.Replicate:
		return fmt.Sprintf("%s:%d", node.Host, node.ReplicatePort), nil
	}
	return "", fmt.Errorf("unknown socket type: %v", stype)
}

func newCluster(cfg *Config) *nodeResolver {
	return &nodeResolver{
		cfg: cfg,
	}
}
