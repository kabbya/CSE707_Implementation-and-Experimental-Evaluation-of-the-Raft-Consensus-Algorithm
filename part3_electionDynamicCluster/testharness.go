package raft

import (
	"log"
	"testing"
	"time"
)

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
}

type Harness struct {
	// cluster is a list of all the raft servers participating in a cluster.
	cluster []*Server

	// connected has a bool per server in cluster, specifying whether this server
	// is currently connected to peers (if false, it's partitioned and no messages
	// will pass to or from it).
	connected []bool

	n int
	t *testing.T

	// ready is the shared channel used to start CMs. It is closed once the
	// initial cluster is wired up; servers added later will proceed immediately.
	ready chan any
}

// NewHarness creates a new test Harness, initialized with n servers connected
// to each other.
func NewHarness(t *testing.T, n int) *Harness {

	ns := make([]*Server, n)
	connected := make([]bool, n)
	ready := make(chan any)

	// Create all Servers in this cluster, assign ids and peer ids.
	for i := 0; i < n; i++ {
		peerIds := make([]int, 0)
		for p := 0; p < n; p++ {
			if p != i {
				peerIds = append(peerIds, p)
			}
		}

		ns[i] = NewServer(i, peerIds, ready)
		ns[i].Serve()
	}

	// Connect all peers to each other.
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j {
				ns[i].ConnectToPeer(j, ns[j].GetListenAddr())
			}
		}
		connected[i] = true
	}

	close(ready)

	return &Harness{
		cluster:    ns,
		connected:  connected,
		n:          n,
		t:          t,
		ready:      ready,
	}
}

// AddServer adds one new server to the running cluster, connects it to all
// currently-connected peers, and updates membership on all servers.
// Returns the new server's id.
func (h *Harness) AddServer() int {
	newID := h.n

	// Build initial peer list for the new server (all existing IDs).
	peerIds := make([]int, 0, h.n)
	for i := 0; i < h.n; i++ {
		peerIds = append(peerIds, i)
	}

	s := NewServer(newID, peerIds, h.ready)
	s.Serve()

	// Extend slices.
	h.cluster = append(h.cluster, s)
	h.connected = append(h.connected, true)
	h.n++

	// Wire up RPC connections with peers that are currently connected.
	for i := 0; i < h.n; i++ {
		if i == newID {
			continue
		}
		// Only create live connections to peers that are marked connected.
		if h.connected[i] {
			if err := h.cluster[newID].ConnectToPeer(i, h.cluster[i].GetListenAddr()); err != nil {
				h.t.Fatal(err)
			}
			if err := h.cluster[i].ConnectToPeer(newID, h.cluster[newID].GetListenAddr()); err != nil {
				h.t.Fatal(err)
			}
		}
	}

	// Update membership (peer ID lists) for all servers.
	for i := 0; i < h.n; i++ {
		peers := make([]int, 0, h.n-1)
		for j := 0; j < h.n; j++ {
			if j != i {
				peers = append(peers, j)
			}
		}
		h.cluster[i].SetPeers(peers)
	}

	return newID
}

// AddServers adds k servers and returns their IDs.
func (h *Harness) AddServers(k int) []int {
	ids := make([]int, 0, k)
	for i := 0; i < k; i++ {
		ids = append(ids, h.AddServer())
	}
	return ids
}

// Shutdown shuts down all the servers in the harness and waits for them to
// stop running.
func (h *Harness) Shutdown() {
	for i := 0; i < h.n; i++ {
		h.cluster[i].DisconnectAll()
		h.connected[i] = false
	}
	for i := 0; i < h.n; i++ {
		h.cluster[i].Shutdown()
	}
}

// DisconnectPeer disconnects a server from all other servers in the cluster.
func (h *Harness) DisconnectPeer(id int) {
	tlog("Disconnect %d", id)
	h.cluster[id].DisconnectAll()
	for j := 0; j < h.n; j++ {
		if j != id {
			h.cluster[j].DisconnectPeer(id)
		}
	}
	h.connected[id] = false
}

// ReconnectPeer connects a server to all other servers in the cluster.
func (h *Harness) ReconnectPeer(id int) {
	tlog("Reconnect %d", id)
	for j := 0; j < h.n; j++ {
		if j != id {
			if err := h.cluster[id].ConnectToPeer(j, h.cluster[j].GetListenAddr()); err != nil {
				h.t.Fatal(err)
			}
			if err := h.cluster[j].ConnectToPeer(id, h.cluster[id].GetListenAddr()); err != nil {
				h.t.Fatal(err)
			}
		}
	}
	h.connected[id] = true
}

// CheckSingleLeader checks that only a single server thinks it's the leader.
// Returns the leader's id and term. It retries several times if no leader is
// identified yet.
func (h *Harness) CheckSingleLeader() (int, int) {
	for r := 0; r < 5; r++ {
		leaderId := -1
		leaderTerm := -1
		for i := 0; i < h.n; i++ {
			if h.connected[i] {
				_, term, isLeader := h.cluster[i].cm.Report()
				if isLeader {
					if leaderId < 0 {
						leaderId = i
						leaderTerm = term
					} else {
						h.t.Fatalf("both %d and %d think they're leaders", leaderId, i)
					}
				}
			}
		}
		if leaderId >= 0 {
			return leaderId, leaderTerm
		}
		time.Sleep(150 * time.Millisecond)
	}

	h.t.Fatalf("leader not found")
	return -1, -1
}

// CheckNoLeader checks that no connected server considers itself the leader.
func (h *Harness) CheckNoLeader() {
	for i := 0; i < h.n; i++ {
		if h.connected[i] {
			_, _, isLeader := h.cluster[i].cm.Report()
			if isLeader {
				h.t.Fatalf("server %d leader; want none", i)
			}
		}
	}
}

func tlog(format string, a ...any) {
	format = "[TEST] " + format
	log.Printf(format, a...)
}

func sleepMs(n int) {
	time.Sleep(time.Duration(n) * time.Millisecond)
}
