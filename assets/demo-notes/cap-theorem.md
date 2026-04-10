# CAP theorem

During a network partition you can have either consistency or availability, not both. The "CA" corner is a trick answer — if there's no partition there's nothing to choose between.

## The three properties

- **Consistency**: every read sees the most recent successful write (linearizability in the strict reading).
- **Availability**: every non-failing node responds to every request.
- **Partition tolerance**: the system keeps working when the network drops messages between nodes.

Partitions are a fact of life in any multi-node system, so in practice you're choosing **CP** or **AP**.

## CP systems (sacrifice availability during partitions)

- Raft-based stores like **etcd**, **Consul**, **Zookeeper**, **CockroachDB**, **TiKV**.
- Reads on the minority side of a partition block or fail. Writes require a majority quorum, so they also block.
- Good fit for coordination data (leader election, config, locks) where a stale read is worse than no read.

## AP systems (sacrifice consistency during partitions)

- **Cassandra**, **DynamoDB** (tunable), **Riak**, **CouchDB**.
- Every replica answers independently. Conflicting writes are reconciled later via last-write-wins timestamps, vector clocks, or CRDTs.
- Good fit for high-throughput user data where the app can tolerate a short window of staleness.

## PACELC — the honest version

CAP only talks about partitions. PACELC adds: **else** (no partition), you still trade **Latency** for **Consistency**. DynamoDB is PA/EL (AP during partition, low-latency during normal ops). Spanner is PC/EC (CP during partition, strong consistency even in normal ops at the cost of cross-region latency via TrueTime).

## Common confusions

- "CA" databases (single-node Postgres, etc.) aren't really CA — they're CP with a partition tolerance of zero, because any partition between client and server kills availability.
- "Eventually consistent" isn't the opposite of consistent — it's a specific model where all non-conflicting writes eventually reach all replicas. You can layer stronger guarantees (causal consistency, session consistency) on top.
