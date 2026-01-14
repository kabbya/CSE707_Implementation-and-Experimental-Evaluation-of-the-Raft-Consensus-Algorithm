# Raft Consensus Project
CSE707 Distributed Systems

## Introduction

This project is an implementation of the Raft consensus algorithm developed as part of the CSE707 Distributed Systems course. The goal of the project is to understand how a consensus protocol works.The project focuses on implementing core Raft functionality and running controlled experiments to see how the system reacts to leader failures, network partitions, quorum loss, recovery, and changes in cluster size. The implementation is divided into three parts, with each part extending the system with additional capabilities.

## How This Project Relates to Distributed Systems

This project addresses one of the core problems in distributed systems: achieving agreement among multiple independent nodes that may fail, disconnect, or communicate asynchronously. The Raft protocol coordinates a group of servers to agree on a single leader and a consistent sequence of actions despite these challenges.

The implementation demonstrates fundamental distributed systems concepts such as leader election, quorum-based decision making, fault tolerance, and failure detection using timeouts. It also highlights the distinction between safety and liveness by ensuring that the system never elects multiple leaders when quorum is lost, while still allowing progress when a majority of servers is available.

By simulating node failures, network partitions, recovery, and dynamic changes in cluster membership, the project shows how distributed systems must function correctly without shared memory, global clocks, or reliable failure detection. Overall, the project provides hands-on experience with the practical challenges and design principles behind real-world distributed systems.


## Project Structure and Parts

### Part 1: Leader Election and Failure Handling

This part implements Raft’s leader election mechanism. Servers can elect a leader, detect leader failure using timeouts, and elect a new leader as long as a quorum of servers is available. It also ensures that no leader is elected when quorum is lost and that outdated leaders step down after reconnection.

### Part 2: Log Replication and Commitment

This part extends the system to support log replication. Client commands are accepted only by the leader, recorded in a replicated log, and propagated to follower servers. A command is considered committed only after it has been replicated on a majority of servers. This ensures consistency even when failures or leader changes occur.

### Part 3: Dynamic Cluster Membership

This part adds support for dynamic cluster membership for experimental purposes. New servers can be added to a running cluster, and the system adapts its leader election behavior based on the updated cluster size. This part is used to study how leader election and fault tolerance behave as the cluster grows and changes over time.

## Log Visualization

This project includes a log visualization tool that helps analyze the behavior of the Raft cluster during test execution. The tool converts textual test output into an HTML file that visually shows server states, leader transitions, and event ordering over time. This makes it easier to understand leader elections, failures, and recovery compared to reading raw logs.

## Experiment / Observation 

### Part 1: Leader Election Experiments

- In a healthy three-node cluster, exactly one leader is elected and the system stabilizes after the initial election. Once elected, the leader maintains authority, and followers remain passive as long as they continue to receive messages.

- When the leader is disconnected, the remaining servers stop receiving updates and independently initiate a new election. As long as a quorum of servers remains connected, a new leader is successfully elected. The election term increases after re-election, and the previously isolated leader is ignored by the majority.

- When quorum is lost, no leader is elected even though servers repeatedly attempt elections. Terms continue to increase, but leadership is never established, preventing split-brain behavior and preserving safety.

- After disconnected servers are reconnected, higher-term information propagates through the cluster. Any outdated leader automatically steps down, and the cluster converges to a single leader again.

- A follower that previously voted for a leader may continue increasing its election term while disconnected if it does not receive updates. When this follower is later reconnected and has the highest term, it can participate in a new election and may become the leader. This demonstrates how term progression influences leadership after partitions.

- When a leader is disconnected, it continues sending messages because it has no direct knowledge of the disconnection. Similarly, other servers do not immediately know whether a peer is disconnected. Message sending continues until failures are indirectly inferred through timeouts. This highlights a limitation of timeout-based failure detection and suggests that further work could explore more explicit mechanisms for identifying disconnections.

- The time required to elect a leader depends on the cluster size. Experiments were conducted with cluster sizes of 5, 10, 20, and 50 servers. For each configuration, leader election time was measured over three runs to reduce randomness. Larger clusters required more time to converge due to increased communication and coordination overhead. Experiments with 100 servers could not be performed due to CPU and operating system limitations.

### Part 2: Log Replication and Commitment Experiments

#### Baseline Replication with Quorum
- With all servers connected, the leader appends client commands to its log and replicates them to follower servers.
- Once a majority of servers store the entry, the leader advances its commit index, and followers learn about the commit through subsequent messages.
- **Observation:** All servers apply the same committed entries in the same order.

#### Follower Disconnection and Log Stalling
- When a follower is disconnected, it no longer receives replication messages, so its log and commit index stop progressing.
- Meanwhile, the leader’s log can continue growing because it still receives client requests.
- **Observation:** The disconnected follower becomes stale and its log lags behind the leader.

#### Leader Appending Uncommitted Entries
- If the leader cannot replicate an entry to a majority (for example, when too many followers are disconnected), it can still append entries locally but cannot commit them.
- These entries remain as uncommitted suffix entries in the leader’s log.
- **Observation:** The leader may appear locally up to date, but the update is not durable.

#### Overwriting Uncommitted Entries After Leader Change
- If a leader containing uncommitted entries later loses leadership, a new leader may be elected that does not contain those entries.
- When the new leader replicates its log, any follower with conflicting uncommitted suffix entries deletes that suffix and accepts the new leader’s entries.

#### Why Losing Uncommitted Entries Is Correct
- Raft guarantees that committed values are never lost, but it does not guarantee durability of uncommitted values.
- Allowing uncommitted entries to persist across leader changes could result in different servers applying different histories, violating safety.
- **Observation:** The loss of uncommitted entries is expected behavior and preserves correctness.

#### Client-Visible Consequences
- If a client sends a command to a leader and receives a response before the command is committed, the client may believe the operation succeeded even though it later disappears.
- In correct designs, the leader should respond to the client only after the entry is committed.
- In this implementation, the client is not explicitly notified of such failures, which remains a potential area for further experimentation.

#### Reconnection and Log Catch-Up
- When a stale or disconnected follower reconnects, the leader identifies the last matching log prefix and sends the missing entries.
- If conflicts exist, the follower deletes its conflicting suffix and accepts the leader’s version.
- **Observation:** After reconnection, the follower converges to the leader’s committed log, even if it previously contained extra uncommitted entries.

### Part 3: Dynamic Cluster Membership Experiments

- The cluster continues to elect a single leader after new servers are added.

- When servers are added while the current leader is disconnected, the connected majority elects a new leader.

- After the old leader is reconnected, it observes a higher term, steps down, and the cluster converges to a single leader.

- Repeated server additions leading to larger cluster sizes still result in stable leader election.


## References and Acknowledgements

This project is based on the Raft consensus algorithm as originally described by Diego Ongaro and John Ousterhout. The following resources were used for understanding the protocol and guiding the implementation:

- In Search of an Understandable Consensus Algorithm (Raft), USENIX ATC 2014  
  https://web.stanford.edu/~ouster/cgi-bin/papers/raft-atc14

- Raft Consensus Algorithm website  
  https://raft.github.io/

- Eli Bendersky’s blog (The Green Place)  
  https://eli.thegreenplace.net/

These resources were used for learning and reference purposes.
