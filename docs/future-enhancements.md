# Future Enhancements

Post-MVP features to pursue after the core system is complete. Grouped by category, ordered roughly by impact.

---

## Queue Enhancements

| Feature | Description | Why |
|---|---|---|
| **Job priorities** | Urgent jobs skip ahead in the queue | Prevents high-priority work from waiting behind thousands of low-priority jobs |
| **Job scheduling** | Cron-style recurring jobs or delayed execution ("run at 3am") | Enables periodic audits without external schedulers |
| **Rate limiting** | Built-in throttle (e.g., max 50 registry calls/minute) | More precise downstream protection than pool size alone |
| **Job cancellation** | Cancel pending or running jobs via the API | Lets users abort an audit without restarting the system |
| **Job dependencies** | DAG-based execution — "don't run B until A completes" | Enables multi-step workflows beyond the current single-job model |

---

## Observability

| Feature | Description | Why |
|---|---|---|
| **Dashboard UI** | Live view of queue depth, in-flight jobs, DLQ count, worker status | Makes system health visible at a glance |
| **Metrics export** | Prometheus-compatible metrics — jobs/sec, latency, retry rate, failure rate | Enables alerting and long-term performance tracking |
| **Webhook notifications** | Notify an external service when a job completes or dead-letters | Integrates with Slack, PagerDuty, or CI pipelines |
| **Structured logging** | JSON logs with job ID, attempt number, duration, error context | Makes debugging and log searching practical at scale |

---

## Scalability

| Feature | Description | Why |
|---|---|---|
| **Horizontal workers** | Multiple machines sharing the same Postgres queue | Scale worker capacity across servers without changing the queue |
| **Adaptive pool sizing** | Auto-tune worker count based on queue depth or downstream health | Balances throughput and downstream pressure dynamically |
| **Partitioned queues** | Separate queues per job type | Prevents a flood of one job type from starving another |

---

## Auditor-Specific

| Feature | Description | Why |
|---|---|---|
| **Multiple ecosystems** | Support npm, PyPI, Go modules, Maven — not just one registry | Broadens the auditor from a demo to a real tool |
| **Vulnerability database** | Check packages against CVE databases (e.g., OSV, NVD) | Adds security scanning alongside license and version checks |
| **SBOM export** | Output the dependency graph as SPDX or CycloneDX format | Industry-standard deliverable for compliance and supply-chain security |
| **Diff mode** | Compare two audit runs — "what changed since last time?" | Tracks dependency drift over time |

---

## Recommended Order

1. **Observability** — dashboard + metrics + structured logging. Makes everything else easier to build, debug, and tune.
2. **Rate limiting** — more precise downstream protection than pool size alone.
3. **Horizontal workers** — the natural next step for scaling the system.
4. **Vulnerability database** — highest-impact auditor upgrade.
