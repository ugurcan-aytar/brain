# Paxos vs Raft in practice

The textbook comparison treats them as equivalent (both give you consensus in a partially synchronous network). In production the differences matter more than the similarities.

## What Raft papers don't tell you

- Raft's strong leader makes it easy to reason about, but it also makes the leader a throughput bottleneck under write-heavy loads. Every commit requires a round-trip from the leader to a majority.
- PreVote and leadership transfer extensions are **not optional** in real deployments — without them, a flaky network partition causes endless term inflation and elections.
- The "committed entries are never lost" guarantee only holds once an entry reaches a majority. Clients with at-least-once semantics see occasional silent drops during leader churn.

## What Paxos gets right that Raft doesn't

- **Multi-Paxos** can have multiple concurrent proposers, which eliminates the single-leader bottleneck in write-heavy workloads. EPaxos pushes this further — any replica can propose, and the protocol resolves conflicts cheaply.
- Paxos doesn't need leader-follower log reconciliation. The "learned values" semantics are cleaner when you care about individual decisions rather than a log.
- In read-mostly workloads with quorum reads, Paxos variants can serve linearizable reads from any node, not just the leader.

## Where Raft wins

- **Understandability.** Engineers can actually reason about Raft's behavior. Paxos is famously easy to get wrong — Lamport's original paper is short but the practical variants (Multi-Paxos, Fast Paxos, Generalized Paxos) accumulate edge cases.
- **Tooling and libraries.** etcd's raft library is battle-tested and widely adopted. There's no equivalent "just use this" Paxos library.
- **Operational simplicity.** Debugging a single leader's decisions is easier than debugging distributed proposer state.

## Practical recommendation

For 95% of use cases, take Raft. You'll get a shipping system faster and the theoretical wins of Paxos variants don't translate into measurable production gains until you're at Spanner-scale. The exceptions are: write-heavy workloads where the leader bottleneck matters, and cross-datacenter deployments where EPaxos's conflict-free fast path wins.
