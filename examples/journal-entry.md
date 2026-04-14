# Reflections on System Design - April 2026

Today I realized that our caching strategy is fundamentally wrong. We're caching at the API gateway level, but the real bottleneck is the database query in the recommendation engine. Moving the cache closer to the data source (Redis layer between service and DB) would reduce latency by ~60%.

The team pushback was about cache invalidation complexity. Fair point. But with event-driven invalidation via Kafka, we can keep the cache fresh within 500ms of a write. That's acceptable for recommendations.

Next step: build a proof of concept with a single endpoint.
