# Raft consensus

Raft keeps a replicated log consistent across a cluster of nodes using a single elected leader.

## Leader election

- Every node starts as a **follower**. If a follower doesn't hear from a leader within its randomized election timeout (150-300ms), it becomes a **candidate** and starts a new term.
- The candidate votes for itself and sends `RequestVote` RPCs to all peers. A node grants its vote if (a) it hasn't voted this term, and (b) the candidate's log is at least as up-to-date as its own.
- A candidate that wins a majority of votes becomes leader. Majority means strictly more than `n/2`, which is why clusters are sized 3, 5, or 7 — even-sized clusters waste a node.
- Split votes trigger a new election with fresh randomized timeouts. The randomization makes repeated splits very unlikely.

## Log replication

- Clients send writes to the leader. The leader appends the entry to its log and sends `AppendEntries` RPCs to followers.
- An entry is **committed** once it's been replicated to a majority. The leader then applies it to its state machine and responds to the client.
- Followers catch up via the leader's periodic heartbeats (also `AppendEntries`, but empty).

## Handling leader failure

- Followers detect the missing heartbeats via the election timeout and start a new election as above.
- Uncommitted entries from the old leader may or may not survive the transition — depends on whether they reached a majority before the leader died.
- Raft guarantees that committed entries are never lost, but an un-acked write to the old leader can be silently dropped. Clients must retry with idempotent requests.

## Things that go wrong

- **Split brain**: can't happen with Raft's majority quorum. A minority partition can't elect a new leader.
- **Slow followers**: the leader tracks a `matchIndex` per follower and keeps retrying until they catch up. A permanently slow follower drags down write latency but doesn't block commits as long as a majority is fast.
- **Network asymmetry**: a follower that can receive but not send will keep triggering elections. The PreVote extension fixes this — a candidate asks "would you vote for me?" before incrementing its term.
