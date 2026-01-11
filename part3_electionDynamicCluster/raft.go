package raft

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"
)

const DebugCM = 1

type LogEntry struct {
	Command any
	Term    int
}

type CMState int

const (
	Follower CMState = iota
	Candidate
	Leader
	Dead
)

func (s CMState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	case Dead:
		return "Dead"
	default:
		panic("unreachable")
	}
}

// ConsensusModule implements leader election + heartbeats.
// (Dynamic membership is supported for experiments via SetPeers, but this is NOT
// full Raft joint-consensus membership change.)
type ConsensusModule struct {
	mu sync.Mutex

	id int

	// Dynamic set of peer IDs (excluding self).
	peers map[int]struct{}

	server *Server

	// Persistent-ish state (not actually persisted to disk in this project).
	currentTerm int
	votedFor    int
	log         []LogEntry

	// Volatile state
	state              CMState
	electionResetEvent time.Time

	// Clean shutdown / leaktest support
	quit chan struct{}
	wg   sync.WaitGroup
}

func NewConsensusModule(id int, peerIds []int, server *Server, ready <-chan any) *ConsensusModule {
	cm := &ConsensusModule{
		id:       id,
		peers:    make(map[int]struct{}, len(peerIds)),
		server:   server,
		state:    Follower,
		votedFor: -1,
		quit:     make(chan struct{}),
	}
	for _, pid := range peerIds {
		if pid == id {
			continue
		}
		cm.peers[pid] = struct{}{}
	}

	// Start once the harness closes `ready`.
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()

		select {
		case <-cm.quit:
			return
		case <-ready:
		}

		cm.mu.Lock()
		cm.electionResetEvent = time.Now()
		cm.mu.Unlock()

		cm.runElectionTimer()
	}()

	return cm
}

// Stop cleanly shuts down the CM and waits for its goroutines to exit.
// This is important to avoid goroutine leaks (e.g., leaktest failures).
func (cm *ConsensusModule) Stop() {
	cm.mu.Lock()
	if cm.state == Dead {
		cm.mu.Unlock()
		return
	}
	cm.state = Dead
	cm.dlog("becomes Dead")
	close(cm.quit)
	cm.mu.Unlock()

	cm.wg.Wait()
}

// SetPeers replaces the current peer set (excluding self). Safe to call while running.
func (cm *ConsensusModule) SetPeers(peerIds []int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	newPeers := make(map[int]struct{}, len(peerIds))
	for _, pid := range peerIds {
		if pid == cm.id {
			continue
		}
		newPeers[pid] = struct{}{}
	}
	cm.peers = newPeers
}

// peerIdSliceLocked returns a snapshot slice of peer IDs.
// Expects cm.mu to be locked.
func (cm *ConsensusModule) peerIdSliceLocked() []int {
	ids := make([]int, 0, len(cm.peers))
	for pid := range cm.peers {
		ids = append(ids, pid)
	}
	return ids
}

func (cm *ConsensusModule) Report() (id int, term int, isLeader bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.id, cm.currentTerm, cm.state == Leader
}

func (cm *ConsensusModule) dlog(format string, args ...any) {
	if DebugCM > 0 {
		format = fmt.Sprintf("[%d] ", cm.id) + format
		log.Printf(format, args...)
	}
}

// ---------------- RPC types ----------------

type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

func (cm *ConsensusModule) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.state == Dead {
		return nil
	}

	cm.dlog("RequestVote: %+v [currentTerm=%d, votedFor=%d]", args, cm.currentTerm, cm.votedFor)

	if args.Term > cm.currentTerm {
		cm.dlog("... term out of date in RequestVote")
		cm.becomeFollowerLocked(args.Term)
	}

	if cm.currentTerm == args.Term &&
		(cm.votedFor == -1 || cm.votedFor == args.CandidateId) {
		reply.VoteGranted = true
		cm.votedFor = args.CandidateId
		cm.electionResetEvent = time.Now()
	} else {
		reply.VoteGranted = false
	}
	reply.Term = cm.currentTerm

	cm.dlog("... RequestVote reply: %+v", reply)
	return nil
}

type AppendEntriesArgs struct {
	Term     int
	LeaderId int

	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

func (cm *ConsensusModule) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.state == Dead {
		return nil
	}

	cm.dlog("AppendEntries: %+v", args)

	if args.Term > cm.currentTerm {
		cm.dlog("... term out of date in AppendEntries")
		cm.becomeFollowerLocked(args.Term)
	}

	reply.Success = false
	if args.Term == cm.currentTerm {
		if cm.state != Follower {
			cm.becomeFollowerLocked(args.Term)
		}
		cm.electionResetEvent = time.Now()
		reply.Success = true
	}
	reply.Term = cm.currentTerm

	cm.dlog("AppendEntries reply: %+v", *reply)
	return nil
}

// ---------------- Election / leader logic ----------------

func (cm *ConsensusModule) electionTimeout() time.Duration {
	if len(os.Getenv("RAFT_FORCE_MORE_REELECTION")) > 0 && rand.Intn(3) == 0 {
		return 150 * time.Millisecond
	}
	return time.Duration(150+rand.Intn(150)) * time.Millisecond
}

func (cm *ConsensusModule) runElectionTimer() {
	timeoutDuration := cm.electionTimeout()

	cm.mu.Lock()
	termStarted := cm.currentTerm
	cm.mu.Unlock()

	cm.dlog("election timer started (%v), term=%d", timeoutDuration, termStarted)

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cm.quit:
			return
		case <-ticker.C:
		}

		cm.mu.Lock()

		if cm.state != Candidate && cm.state != Follower {
			cm.mu.Unlock()
			return
		}
		if termStarted != cm.currentTerm {
			cm.mu.Unlock()
			return
		}

		if time.Since(cm.electionResetEvent) >= timeoutDuration {
			cm.startElectionLocked()
			cm.mu.Unlock()
			return
		}

		cm.mu.Unlock()
	}
}

// startElectionLocked starts a new election with this CM as candidate.
// Expects cm.mu to be locked.
func (cm *ConsensusModule) startElectionLocked() {
	cm.state = Candidate
	cm.currentTerm++
	savedCurrentTerm := cm.currentTerm
	cm.electionResetEvent = time.Now()
	cm.votedFor = cm.id

	cm.dlog("becomes Candidate (currentTerm=%d); log=%v", savedCurrentTerm, cm.log)

	peerIds := cm.peerIdSliceLocked()
	clusterSize := len(peerIds) + 1
	votesReceived := 1

	for _, peerId := range peerIds {
		pid := peerId

		cm.wg.Add(1)
		go func() {
			defer cm.wg.Done()

			select {
			case <-cm.quit:
				return
			default:
			}

			args := RequestVoteArgs{
				Term:        savedCurrentTerm,
				CandidateId: cm.id,
			}
			var reply RequestVoteReply

			cm.dlog("sending RequestVote to %d: %+v", pid, args)
			if err := cm.server.Call(pid, "ConsensusModule.RequestVote", args, &reply); err != nil {
				return
			}

			cm.mu.Lock()
			defer cm.mu.Unlock()

			if cm.state != Candidate {
				return
			}

			cm.dlog("received RequestVoteReply %+v", reply)

			if reply.Term > savedCurrentTerm {
				cm.dlog("term out of date in RequestVoteReply")
				cm.becomeFollowerLocked(reply.Term)
				return
			}

			if reply.Term == savedCurrentTerm && reply.VoteGranted {
				votesReceived++
				if votesReceived*2 > clusterSize {
					cm.dlog("wins election with %d votes", votesReceived)
					cm.startLeaderLocked()
					return
				}
			}
		}()
	}

	// Run another election timer, in case this election isn't successful.
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		cm.runElectionTimer()
	}()
}

// becomeFollowerLocked makes cm a follower and resets its state.
// Expects cm.mu to be locked.
func (cm *ConsensusModule) becomeFollowerLocked(term int) {
	if cm.state == Dead {
		return
	}
	cm.dlog("becomes Follower with term=%d; log=%v", term, cm.log)
	cm.state = Follower
	cm.currentTerm = term
	cm.votedFor = -1
	cm.electionResetEvent = time.Now()

	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		cm.runElectionTimer()
	}()
}

// startLeaderLocked switches cm into leader state and begins heartbeats.
// Expects cm.mu to be locked.
func (cm *ConsensusModule) startLeaderLocked() {
	if cm.state == Dead {
		return
	}
	cm.state = Leader
	cm.dlog("becomes Leader; term=%d, log=%v", cm.currentTerm, cm.log)

	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()

		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			cm.leaderSendHeartbeats()

			select {
			case <-cm.quit:
				return
			case <-ticker.C:
			}

			cm.mu.Lock()
			if cm.state != Leader {
				cm.mu.Unlock()
				return
			}
			cm.mu.Unlock()
		}
	}()
}

func (cm *ConsensusModule) leaderSendHeartbeats() {
	cm.mu.Lock()
	if cm.state != Leader {
		cm.mu.Unlock()
		return
	}
	savedCurrentTerm := cm.currentTerm
	peerIds := cm.peerIdSliceLocked()
	cm.mu.Unlock()

	for _, peerId := range peerIds {
		pid := peerId
		args := AppendEntriesArgs{
			Term:     savedCurrentTerm,
			LeaderId: cm.id,
		}

		cm.wg.Add(1)
		go func() {
			defer cm.wg.Done()

			select {
			case <-cm.quit:
				return
			default:
			}

			cm.dlog("sending AppendEntries to %v: ni=%d, args=%+v", pid, 0, args)
			var reply AppendEntriesReply
			if err := cm.server.Call(pid, "ConsensusModule.AppendEntries", args, &reply); err != nil {
				return
			}

			cm.mu.Lock()
			defer cm.mu.Unlock()

			if cm.state != Leader {
				return
			}
			if reply.Term > savedCurrentTerm {
				cm.dlog("term out of date in heartbeat reply")
				cm.becomeFollowerLocked(reply.Term)
				return
			}
		}()
	}
}
