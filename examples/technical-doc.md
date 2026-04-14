# Circuit Breaker Pattern

A circuit breaker prevents cascading failures in distributed systems. When a downstream service fails repeatedly, the circuit breaker "opens" and stops sending requests, giving the service time to recover.

## States
1. **Closed** — requests flow normally, failures are counted
2. **Open** — requests are immediately rejected without calling the downstream service
3. **Half-Open** — a limited number of test requests are allowed through

## Configuration
- Failure threshold: 5 consecutive failures to open the circuit
- Recovery timeout: 30 seconds before transitioning to half-open
- Success threshold: 3 successful requests in half-open to close the circuit
