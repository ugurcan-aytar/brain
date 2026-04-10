# Eventual consistency in production

The theoretical guarantees of eventual consistency are necessary but nowhere near sufficient. What matters operationally is the **bounded staleness window** and the **observability** of when you're in it.

## The real failure mode

Eventually-consistent systems rarely fail by returning wildly wrong data. They fail by returning *slightly* stale data at exactly the wrong moment:

- A user adds an item to their cart, navigates to checkout, and sees an empty cart because the read hit a replica that hasn't caught up yet.
- A permissions change is applied, and for the next 200ms the user can still access resources they were just removed from.
- A price update propagates to the pricing service but not to the UI cache, so a customer sees and clicks on the old price.

None of these are "consistency violations" in the CAP theorem sense. The system is doing exactly what the docs say it does. The user experience is still broken.

## What you actually need

- **Read-your-writes consistency**, at minimum, scoped to a session. If a user wrote it, they should read it. Most "eventual consistency is fine" arguments quietly assume this.
- **Monotonic reads**, so a user doesn't see a value, refresh, and see an older value. This is surprisingly easy to violate with sticky-session load balancers that change stickiness on reconnect.
- **Bounded staleness alerts** at the infrastructure layer. If your replica lag exceeds N seconds, something should page. Azure Cosmos and DynamoDB both expose this metric — use it.

## When pure eventual consistency is actually fine

- **Read-only analytical workloads** that run nightly anyway. No user is waiting.
- **Metrics and logs**, where a few seconds of lag is invisible next to the 5-minute dashboard refresh cadence.
- **Derived caches** that get invalidated on writes — here the "eventual" is measured in tens of milliseconds.

## The takeaway

CAP-theorem framing treats consistency as a binary choice. Production reality is a continuum, and the real engineering work lives in the middle: tunable consistency, session guarantees, bounded lag budgets, and the observability to know when you're outside them.
