# Module 07 — Memory Fabric

**Persistent organizational cognition layer.**

---

## Purpose

The Memory Fabric is Operan's institutional memory backbone. It transforms transient AI interactions into persistent, searchable, provable organizational knowledge. Agents don't just work — they learn and remember.

## Memory Types

### Semantic Memory
- **Facts and embeddings** derived from ingested documents
- Vector similarity search across enterprise knowledge
- Metadata-rich entries with confidence scores
- Automatic expiration and refresh cycles

### Episodic Memory
- **Historical executions** — every agent action recorded
- Event-driven capture from workflow runs
- Outcome tracking with evidence attachments
- Time-serializable for audit and replay

### Procedural Memory
- **Operational workflows** captured as reusable procedures
- Role-based step definitions with tool requirements
- Condition-gated execution paths
- Version-controlled procedure evolution

### Graph Memory
- **Relationship structures** connecting entities
- Multi-hop traversal for institutional context
- Weighted edges for relationship strength
- Dynamic graph construction from episodic data

### Institutional Memory
- **Long-term organizational intelligence** synthesized from all other layers
- Auto-generated summaries from episodic compression
- Domain-organized knowledge bases
- Historical trend analysis

## Key Capabilities

- **Multi-tenant isolation** — memory never crosses tenant boundaries
- **Lineage tracking** — every memory entry traces back to its source
- **Provenance** — full attribution chain from ingestion to query
- **Memory compression** — episodic entries distilled into summaries over time
- **Expiration policies** — automatic cleanup of stale memory
- **Confidence scoring** — every retrieval includes reliability metrics

## Suggested Stack

| Component          | Technology |
|-------------------|------------|
| Vector Storage    | Qdrant     |
| Graph Database    | Neo4j      |
| Relational Store  | PostgreSQL |
| Cache             | Redis      |

## API Endpoints

| Method   | Endpoint                          | Description                  |
|----------|-----------------------------------|------------------------------|
| POST     | `/api/v1/memory/semantic`         | Store semantic memory entry  |
| GET      | `/api/v1/memory/semantic`         | Query semantic memory        |
| POST     | `/api/v1/memory/episodic`         | Store an episode             |
| GET      | `/api/v1/memory/episodic`         | Query episodic memory        |
| POST     | `/api/v1/memory/procedural`       | Register a procedure         |
| GET      | `/api/v1/memory/procedural`       | Query procedural memory      |
| POST     | `/api/v1/memory/graph`            | Create graph memory edges    |
| GET      | `/api/v1/memory/graph`            | Query graph memory           |
| GET      | `/api/v1/memory/institutional`    | Get institutional summaries  |
| GET      | `/api/v1/memory/knowledge/{id}/lineage` | Get memory lineage    |
| GET      | `/api/v1/memory/knowledge/{id}/provenance` | Get memory provenance |
| DELETE   | `/api/v1/memory/flush`            | Flush expired entries        |
| POST     | `/api/v1/memory/compress`         | Compress episodic memory     |
| GET      | `/api/v1/memory/health`           | Memory fabric health         |

## Governance & Safety

- Memory poisoning detection and quarantine
- Hallucination correlation across memory entries
- Source trust scoring affects memory weight
- Policy-gated memory access (RBAC + ABAC)
- Immutable memory audit trails

## Critical Risks

| Risk                  | Mitigation                                  |
|-----------------------|---------------------------------------------|
| Memory poisoning      | Source validation, trust scoring, quarantine |
| Cross-tenant bleed    | Cryptographic tenant isolation              |
| Stale knowledge       | Expiration policies, refresh triggers        |
| Vector drift          | Periodic re-embedding, drift detection       |
| Lineage breakage      | Chain-of-custody tracking at every stage     |

## Module Dependencies

- **Module 06** — Knowledge Ingestion Pipeline (feeds semantic/episodic memory)
- **Module 03** — Agent Orchestration Engine (consumes procedural memory)
- **Module 10** — Policy & Governance Engine (access controls)
- **Module 11** — Observability & Telemetry (query metrics)

## Related Artifacts

- `contracts/v1/openapi-07-memory-fabric.yaml` — OpenAPI specification
- `contracts/v1/schema-07-memory-fabric.json` — JSON Schema definitions
