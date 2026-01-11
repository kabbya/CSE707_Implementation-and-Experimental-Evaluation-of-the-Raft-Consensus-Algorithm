package raft

import (
	"testing"
	"time"
)

// Helper: wait until the harness reports a single leader (or fail).
func requireSingleLeader(t *testing.T, h *Harness) (leaderID int, leaderTerm int) {
	t.Helper()
	return h.CheckSingleLeader()
}

// Helper: small sleep to allow heartbeats/elections to settle.
func settle() {
	time.Sleep(400 * time.Millisecond)
}

// Basic existing behaviors
func TestInitialElection_SingleLeader(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	_, _ = requireSingleLeader(t, h)
}

func TestReElectionAfterLeaderDisconnect(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	leader1, term1 := requireSingleLeader(t, h)

	h.DisconnectPeer(leader1)
	settle()

	leader2, term2 := requireSingleLeader(t, h)
	if leader2 == leader1 {
		t.Fatalf("expected a new leader after disconnecting old leader %d", leader1)
	}
	if term2 <= term1 {
		t.Fatalf("expected term to increase after re-election, got old=%d new=%d", term1, term2)
	}
}


// Dynamic membership: adding new servers

func TestAddServers_LeaderElection(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	leader1, term1 := requireSingleLeader(t, h)

	dumpClusterState(t,h,"AFTER INITIAL ELECTION (3-node cluster)",)

	t.Logf("Initial leader elected: leader=%d term=%d clusterSize=%d",leader1,term1,h.n,)

	// Add 2 servers dynamically (cluster becomes 5).
	h.AddServers(2)
	settle()

	leader2, term2 := requireSingleLeader(t, h)

	dumpClusterState(t,h,"AFTER ADDING 2 SERVERS AND RE-ELECTION (5-node cluster)",)

	t.Logf("Post-add leader elected: leader=%d term=%d clusterSize=%d",leader2, term2, h.n,)

	// Term must not go backwards
	if term2 < term1 {
		t.Fatalf("term went backwards after adding servers: old=%d new=%d (leader1=%d leader2=%d)",term1, term2, leader1, leader2,)
	}
}

func TestAddServerManyTimes_StillConvergesToSingleLeader(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	_, _ = requireSingleLeader(t, h)

	// Repeatedly grow the cluster.
	for i := 0; i < 7; i++ { // 3 -> 10 servers
		h.AddServer()
		// shorter settle between adds is fine; we do a longer one at the end.
		time.Sleep(120 * time.Millisecond)
	}
	settle()

	_, _ = requireSingleLeader(t, h)
}


func TestAddServerWhileCurrentLeaderDisconnected_ClusterConverges(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	leader1, term1 := requireSingleLeader(t, h)

	dumpClusterState(t, h, "AFTER INITIAL ELECTION (3-node cluster)")
	t.Logf("Initial leader elected: leader=%d term=%d clusterSize=%d", leader1, term1, h.n)

	// Disconnect current leader from the cluster.
	h.DisconnectPeer(leader1)

	dumpClusterState(t, h, "AFTER DISCONNECTING INITIAL LEADER (still 3 nodes)")
	t.Logf("Disconnected leader: leader=%d (clusterSize=%d)", leader1, h.n)

	// While leader is disconnected, add 2 new servers.
	h.AddServers(2) // cluster size becomes 5

	dumpClusterState(t, h, "AFTER ADDING 2 SERVERS (leader still disconnected; 5-node cluster)")
	t.Logf("Added 2 servers; clusterSize=%d (old leader %d still disconnected)", h.n, leader1)

	settle()

	leader2, term2 := requireSingleLeader(t, h)

	dumpClusterState(t, h, "AFTER RE-ELECTION WITH OLD LEADER DISCONNECTED (5-node cluster)")
	t.Logf("New leader elected (with old leader disconnected): leader=%d term=%d clusterSize=%d", leader2, term2, h.n)

	if leader2 == leader1 {
		t.Fatalf("disconnected leader %d should not still be leader among connected nodes", leader1)
	}
	if term2 <= term1 {
		t.Fatalf("expected term to increase after re-election with leader disconnected, old=%d new=%d", term1, term2)
	}

	// Reconnect the old leader; it should step down if outdated and cluster should still converge.
	h.ReconnectPeer(leader1)

	dumpClusterState(t, h, "AFTER RECONNECTING OLD LEADER (before settle; 5-node cluster)")
	t.Logf("Reconnected old leader=%d; clusterSize=%d", leader1, h.n)

	settle()

	leader3, term3 := requireSingleLeader(t, h)

	dumpClusterState(t, h, "AFTER FINAL CONVERGENCE (old leader reconnected; 5-node cluster)")
	t.Logf("Final leader elected after reconnection: leader=%d term=%d clusterSize=%d", leader3, term3, h.n)
}


func dumpClusterState(t *testing.T, h *Harness, phase string) {
	t.Helper()

	t.Logf("==== %s ====", phase)
	t.Logf("Cluster size: %d", h.n)

	for i := 0; i < h.n; i++ {
		srv := h.cluster[i]
		id, term, isLeader := srv.cm.Report()

		role := "Follower"
		if isLeader {
			role = "Leader"
		}

		conn := "DISCONNECTED"
		if h.connected[i] {
			conn = "CONNECTED"
		}

		t.Logf(
			"Server %d | term=%d | role=%s | %s | peers=%v",
			id,
			term,
			role,
			conn,
			srv.peerIds,
		)
	}

	t.Logf("====================")
}


// func TestAddServerThenPartitionMinority_NoLeaderInMinority(t *testing.T) {
// 	h := NewHarness(t, 5)
// 	defer h.Shutdown()

// 	leader, _ := requireSingleLeader(t, h)

// 	// Add 2 servers -> 7 total
// 	h.AddServers(2)
// 	settle()

// 	_, _ = requireSingleLeader(t, h)

// 	// Create a minority partition by disconnecting 3 servers (keep 4 connected = majority).
// 	// We'll disconnect the leader + two others to make it interesting.
// 	h.DisconnectPeer(leader)
// 	h.DisconnectPeer(0)
// 	if leader == 0 {
// 		h.DisconnectPeer(1)
// 	} else {
// 		h.DisconnectPeer(1)
// 	}

// 	settle()

// 	// In the remaining connected majority, there should be a leader.
// 	_, _ = requireSingleLeader(t, h)
// }

// func TestAddServers_LargeClusterElectsLeader(t *testing.T) {
// 	// This is heavier than the others. You can skip with: go test -short
// 	if testing.Short() {
// 		t.Skip("skipping large dynamic membership test in -short mode")
// 	}

// 	h := NewHarness(t, 5)
// 	defer h.Shutdown()

// 	_, _ = requireSingleLeader(t, h)

// 	// Grow to 50 servers.
// 	for i := 0; i < 45; i++ {
// 		h.AddServer()
// 	}
// 	// Give the bigger cluster a bit more time.
// 	time.Sleep(1200 * time.Millisecond)

// 	_, _ = requireSingleLeader(t, h)
// }


// func TestAddServerWhileCurrentLeaderDisconnected_ClusterConverges(t *testing.T) {
// 	h := NewHarness(t, 3)
// 	defer h.Shutdown()

// 	leader1, term1 := requireSingleLeader(t, h)

// 	// Disconnect current leader from the cluster.
// 	h.DisconnectPeer(leader1)

// 	// While leader is disconnected, add 2 new servers.
// 	h.AddServers(2) // cluster size becomes 5
// 	settle()

// 	leader2, term2 := requireSingleLeader(t, h)
// 	if leader2 == leader1 {
// 		t.Fatalf("disconnected leader %d should not still be leader among connected nodes", leader1)
// 	}
// 	if term2 <= term1 {
// 		t.Fatalf("expected term to increase after re-election with leader disconnected, old=%d new=%d", term1, term2)
// 	}

// 	// Reconnect the old leader; it should step down if outdated and cluster should still converge.
// 	h.ReconnectPeer(leader1)
// 	settle()

// 	_, _ = requireSingleLeader(t, h)
// }

// func pickFollowerID(h *Harness, leaderID int) int {
// 	for i := 0; i < h.n; i++ {
// 		if i != leaderID {
// 			return i
// 		}
// 	}
// 	return -1
// }

