# CRDTs — Conflict-Free Replicated Data Types

CRDTs let multiple nodes update a shared value independently and merge the results into the same answer, no matter the order the updates arrive in. They're the math behind the "offline-first" feel in Figma, Linear, Notion, and Yjs-powered editors.

## The core property

A CRDT's merge function must be:

- **Commutative**: `merge(a, b) == merge(b, a)`
- **Associative**: `merge(merge(a, b), c) == merge(a, merge(b, c))`
- **Idempotent**: `merge(a, a) == a`

Together these mean: replay deliveries, reorder deliveries, duplicate deliveries — the final state is always the same. No coordination needed.

## Two flavors

- **State-based (CvRDT)**: nodes periodically exchange their *entire state* and merge. Heavy on bandwidth, light on ordering assumptions.
- **Operation-based (CmRDT)**: nodes broadcast *individual operations*. Lighter on bandwidth but requires a causal broadcast layer so ops aren't duplicated or reordered in an illegal way.

## Useful types

- **G-Counter**: a counter that only grows. Each node keeps its own counter, and merge takes the max per-node. Good for analytics counters.
- **PN-Counter**: increment + decrement, implemented as two G-Counters.
- **OR-Set** (observed-remove set): add and remove elements. Each add gets a unique tag so you can distinguish a removal-that-happened from a concurrent add.
- **LWW-Register**: a value with a timestamp. Last-write-wins on merge. Simple but loses information.
- **RGA / Logoot / Yjs sequence**: ordered sequences for collaborative text editing. Each character gets a unique identifier that determines its position globally.

## Where they shine vs. where they don't

- **Shine**: offline-first apps, collaborative editing, multi-leader replication where conflicts are frequent but mergeable.
- **Don't**: anything that needs a global invariant ("balance can't go negative", "at most one leader", uniqueness constraints). CRDTs can only enforce per-value constraints, not cross-value ones. For those, you need consensus (see Raft).

## Relationship to CAP

CRDTs are the classic **AP** tool — they let an AP system give you a strong eventual consistency guarantee without giving up availability during partitions. But they don't magically give you linearizability; if your domain needs it, no amount of CRDT engineering will close the gap.
