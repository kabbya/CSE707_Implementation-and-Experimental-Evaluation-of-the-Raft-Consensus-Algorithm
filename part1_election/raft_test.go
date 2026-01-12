package raft

import (
	"testing"
	"time"
	"github.com/fortytw2/leaktest"
)

const defaultNumServers = 3

// go test -v -run TestElectionBasic
func TestElectionBasic(t *testing.T) {
	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()

	h.CheckSingleLeader()

}

func TestElectionLeaderDisconnect(t *testing.T) {
	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()

	origLeaderId, origTerm := h.CheckSingleLeader()

	h.DisconnectPeer(origLeaderId)
	sleepMs(350)

	newLeaderId, newTerm := h.CheckSingleLeader()
	if newLeaderId == origLeaderId {
		t.Errorf("want new leader to be different from orig leader")
	}
	if newTerm <= origTerm {
		t.Errorf("want newTerm <= origTerm, got %d and %d", newTerm, origTerm)
	}

}

func TestElectionLeaderAndAnotherDisconnect(t *testing.T) {
	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()

	origLeaderId, _ := h.CheckSingleLeader()

	h.DisconnectPeer(origLeaderId)
	otherId := (origLeaderId + 1) % defaultNumServers
	h.DisconnectPeer(otherId)

	// No quorum.
	sleepMs(450)
	h.CheckNoLeader()

	// Reconnect one other server; now we'll have quorum.
	h.ReconnectPeer(otherId)
	h.CheckSingleLeader()

}

func TestDisconnectAllThenRestore(t *testing.T) {
	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()

	sleepMs(100)
	//	Disconnect all servers from the start. There will be no leader.
	for i := 0; i < defaultNumServers; i++ {
		h.DisconnectPeer(i)
	}
	sleepMs(450)
	h.CheckNoLeader()

	// Reconnect all servers. A leader will be found.
	for i := 0; i < defaultNumServers; i++ {
		h.ReconnectPeer(i)
	}
	h.CheckSingleLeader()
}

func TestElectionLeaderDisconnectThenReconnect(t *testing.T) {
	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()
	origLeaderId, _ := h.CheckSingleLeader()

	h.DisconnectPeer(origLeaderId)

	sleepMs(350)
	newLeaderId, newTerm := h.CheckSingleLeader()

	h.ReconnectPeer(origLeaderId)
	sleepMs(150)

	againLeaderId, againTerm := h.CheckSingleLeader()

	if newLeaderId != againLeaderId {
		t.Errorf("again leader id got %d; want %d", againLeaderId, newLeaderId)
	}
	if againTerm != newTerm {
		t.Errorf("again term got %d; want %d", againTerm, newTerm)
	}
}

func TestElectionLeaderDisconnectThenReconnect5(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, 5)
	defer h.Shutdown()

	origLeaderId, _ := h.CheckSingleLeader()

	h.DisconnectPeer(origLeaderId)
	sleepMs(150)
	newLeaderId, newTerm := h.CheckSingleLeader()

	h.ReconnectPeer(origLeaderId)
	sleepMs(150)

	againLeaderId, againTerm := h.CheckSingleLeader()

	if newLeaderId != againLeaderId {
		t.Errorf("again leader id got %d; want %d", againLeaderId, newLeaderId)
	}
	if againTerm != newTerm {
		t.Errorf("again term got %d; want %d", againTerm, newTerm)
	}
}

func TestElectionFollowerComesBack(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()

	origLeaderId, origTerm := h.CheckSingleLeader()

	otherId := (origLeaderId + 1) % defaultNumServers
	h.DisconnectPeer(otherId)
	time.Sleep(650 * time.Millisecond)
	h.ReconnectPeer(otherId)
	sleepMs(150)

	// We can't have an assertion on the new leader id here because it depends
	// on the relative election timeouts. We can assert that the term changed,
	// however, which implies that re-election has occurred.
	_, newTerm := h.CheckSingleLeader()
	if newTerm <= origTerm {
		t.Errorf("newTerm=%d, origTerm=%d", newTerm, origTerm)
	}
}

func TestElectionDisconnectLoop(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()

	for cycle := 0; cycle < 5; cycle++ {
		leaderId, _ := h.CheckSingleLeader()

		h.DisconnectPeer(leaderId)
		otherId := (leaderId + 1) % defaultNumServers
		h.DisconnectPeer(otherId)
		sleepMs(310)
		h.CheckNoLeader()

		// Reconnect both.
		h.ReconnectPeer(otherId)
		h.ReconnectPeer(leaderId)

		// Give it time to settle
		sleepMs(150)
	}
}


/*
func TestPart1FullFlow(t *testing.T) {
	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()

	// 1) basic election
	leader, _ := h.CheckSingleLeader()
	time.Sleep(2 * time.Second)

	// 2) disconnect leader -> new election
	t.Logf("[TEST] Disconnect leader %d", leader)
	h.DisconnectPeer(leader)
	time.Sleep(3 * time.Second)
	newLeader, _ := h.CheckSingleLeader()
	time.Sleep(2 * time.Second)

	// 3) reconnect old leader
	t.Logf("[TEST] Reconnect old leader %d", leader)
	h.ReconnectPeer(leader)
	time.Sleep(2 * time.Second)

	// 4) disconnect new leader too
	t.Logf("[TEST] Disconnect leader %d", newLeader)
	h.DisconnectPeer(newLeader)
	time.Sleep(3 * time.Second)
	h.CheckSingleLeader()
	time.Sleep(2 * time.Second)
}

*/



func TestScenarioPart1(t *testing.T) {
	h := NewHarness(t, defaultNumServers)
	defer h.Shutdown()

	quorum := defaultNumServers/2 + 1
	t.Logf("[TEST] Init: defaultNumServers=%d quorum=%d", defaultNumServers, quorum)

	down := make([]int, 0, defaultNumServers)

	// STEP 1: Initial leader election
	leader, term := h.CheckSingleLeader()
	t.Logf("[TEST] Step 1: leader1=%d term=%d", leader, term)

	// STEP 2A: Kill leader1 -> expect leader2
	t.Logf("[TEST] Step 2A: Disconnect leader1=%d term=%d", leader, term)
	h.DisconnectPeer(leader)
	down = append(down, leader)

	sleepMs(350)

	newLeader, newTerm := h.CheckSingleLeader()
	t.Logf("[TEST] Step 2A: Elected leader2=%d term=%d", newLeader, newTerm)

	if newLeader == leader {
		t.Fatalf("[TEST] Step 2A: expected a different leader after leader1 disconnect; got same leader=%d", newLeader)
	}
	if newTerm <= term {
		t.Fatalf("[TEST] Step 2A: expected term to increase after re-election; oldTerm=%d newTerm=%d", term, newTerm)
	}

	leader, term = newLeader, newTerm

	// STEP 2B: Keep killing leaders while quorum is still possible.
	// Elections are possible as long as connectedServers >= quorum.
	// connectedServers = defaultNumServers - len(down)
	for defaultNumServers-len(down) >= quorum {
		t.Logf("[TEST] Step 2B: Disconnect current leader=%d term=%d (down=%v connected=%d)",
			leader, term, down, defaultNumServers-len(down))

		h.DisconnectPeer(leader)
		down = append(down, leader)

		sleepMs(350)

		// If quorum still exists after this disconnect, a new leader MUST be elected.
		if defaultNumServers-len(down) >= quorum {
			nextLeader, nextTerm := h.CheckSingleLeader()
			t.Logf("[TEST] Step 2B: New leader=%d term=%d (after disconnect, down=%v connected=%d)",
				nextLeader, nextTerm, down, defaultNumServers-len(down))

			if nextLeader == leader {
				t.Fatalf("[TEST] Step 2B: leader did not change after disconnect; leader=%d", nextLeader)
			}
			if nextTerm <= term {
				t.Fatalf("[TEST] Step 2B: expected term to increase; oldTerm=%d newTerm=%d", term, nextTerm)
			}

			leader, term = nextLeader, nextTerm
		}
	}

	// STEP 3: Quorum is now impossible => no leader should be present.
	sleepMs(450)
	t.Logf("[TEST] Step 3: Quorum lost. down=%v connected=%d quorum=%d",
		down, defaultNumServers-len(down), quorum)

	h.CheckNoLeader()
	t.Logf("[TEST] Step 3: Confirmed no leader while quorum lost")

	// STEP 4: Reconnect everyone we disconnected.
	for _, id := range down {
		t.Logf("[TEST] Step 4: Reconnect node=%d", id)
		h.ReconnectPeer(id)
	}
	sleepMs(300)

	// STEP 5: After recovery, a leader must exist again.
	finalLeader, finalTerm := h.CheckSingleLeader()
	t.Logf("[TEST] Step 5: Recovered leader=%d term=%d", finalLeader, finalTerm)
}


// measureLeaderElectionTime starts an N-node cluster and returns how long it
// takes until exactly one leader is detected.
func measureLeaderElectionTime(t *testing.T, n int) time.Duration {
	
	h := NewHarness(t, n)
	defer h.Shutdown()

	start := time.Now()
	h.CheckSingleLeader() // blocks until a leader is found (or fails the test)
	return time.Since(start)
}

func TestLeaderElectionTimeScaling(t *testing.T) {
	t.Log("Here")
	// Optional: run multiple trials to smooth randomness
	trials := 3

	type result struct {
		n    int
		durs []time.Duration
		avg  time.Duration
	}

	run := func(n int) result {
		r := result{n: n}
		var sum time.Duration
		for i := 0; i < trials; i++ {
			d := measureLeaderElectionTime(t, n)
			r.durs = append(r.durs, d)
			sum += d
		}
		r.avg = sum / time.Duration(trials)
		return r
	}

	t.Log("BEFORE run")
	cluster_ := run(5)
	t.Log("AFTER run")
	t.Logf("Output")
	t.Logf("[TIMING] %d nodes: per-trial=%v avg=%v", cluster_.n, cluster_.durs, cluster_.avg)

}
