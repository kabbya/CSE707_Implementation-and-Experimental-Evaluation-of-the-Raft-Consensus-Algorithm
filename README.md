# Raft Consensus Project
CSE707 Distributed Systems

## Introduction

This project is an implementation of the Raft consensus algorithm developed as part of the CSE707 Distributed Systems course. The goal of the project is to understand how a consensus protocol works in practice by building it step by step and observing its behavior under failures and cluster changes.

The project focuses on implementing core Raft functionality and running controlled experiments to see how the system reacts to leader failures, network partitions, quorum loss, recovery, and changes in cluster size. The implementation is divided into three parts, with each part extending the system with additional capabilities.

## Project Structure and Parts

### Part 1: Leader Election and Failure Handling

This part implements Raft’s leader election mechanism. Servers can elect a leader, detect leader failure using timeouts, and elect a new leader as long as a quorum of servers is available. It also ensures that no leader is elected when quorum is lost and that outdated leaders step down after reconnection.

### Part 2: Log Replication and Commitment

This part extends the system to support log replication. Client commands are accepted only by the leader, recorded in a replicated log, and propagated to follower servers. A command is considered committed only after it has been replicated on a majority of servers. This ensures consistency even when failures or leader changes occur.

### Part 3: Dynamic Cluster Membership

This part adds support for dynamic cluster membership for experimental purposes. New servers can be added to a running cluster, and the system adapts its leader election behavior based on the updated cluster size. This part is used to study how leader election and fault tolerance behave as the cluster grows and changes over time.

## References and Acknowledgements

This project is based on the Raft consensus algorithm as originally described by Diego Ongaro and John Ousterhout. The following resources were used for understanding the protocol and guiding the implementation:

- In Search of an Understandable Consensus Algorithm (Raft), USENIX ATC 2014  
  https://web.stanford.edu/~ouster/cgi-bin/papers/raft-atc14

- Raft Consensus Algorithm website  
  https://raft.github.io/

- Eli Bendersky’s blog (The Green Place)  
  https://eli.thegreenplace.net/

These resources were used for learning and reference purposes.
