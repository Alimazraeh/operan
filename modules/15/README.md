# Module 15 — Knowledge Base Engine

**Knowledge management.**

---

## Purpose

The Knowledge Base Engine provides a comprehensive knowledge management system for the Operan ecosystem. It handles document ingestion, semantic search, natural language Q&A, knowledge graph construction, and vector similarity search with full source attribution and versioning.

## Core Functions

### Collection Management
Organized knowledge repositories:

| Feature               | Description                                    |
|----------------------|------------------------------------------------|
| collections          | Isolated knowledge stores per tenant           |
| embedding models     | Configurable vector models for each collection |
| distance metrics     | Cosine, dot product, or Euclidean similarity   |
| vector indexes       | HNSW, IVF, or flat indexing strategies         |
| access control       | Granular permissions per collection            |
| tagging              | Organize with custom tags and metadata         |
| versioning           | Track collection changes over time             |

### Document Processing
Multi-format document ingestion:

| Document Type | Supported Formats              | Processing Approach            |
|--------------|-------------------------------|--------------------------------|
| text         | plain text, markdown          | Direct chunking                |
| documents    | PDF, DOCX                     | Content extraction + chunking  |
| web          | HTML                          | HTML parsing + text extraction |
| structured   | CSV, JSON                     | Row/record chunking            |

- **Chunking strategies** — Fixed-size, semantic, paragraph, or sentence-based
- **Configurable parameters** — Chunk size and overlap customization
- **Auto-embedding** — Automatic vector generation on ingestion
- **Metadata extraction** — Attach custom metadata to chunks
- **Error handling** — Robust processing with error recovery

### Semantic Search
Vector-based similarity search:

- **Vector similarity** — Cosine, dot product, or Euclidean distance
- **Filtering** — Filter results by metadata
- **Configurable depth** — Adjustable top-k results and threshold
- **Content return** — Optional full chunk content or metadata only
- **Hybrid search** — Combine semantic and keyword search

### Q&A Engine
Natural language question answering:

| Answer Style     | Description                                  | Use Case                    |
|-----------------|----------------------------------------------|-----------------------------|
| concise         | Brief, direct answers                        | Quick lookups               |
| detailed        | Comprehensive explanations                   | Deep understanding          |
| cite_sources    | Answers with source references               | Verified information        |

- **Context-aware** — Retrieves relevant documents before answering
- **Source attribution** — Cites sources used in answers
- **Confidence scoring** — Indicates answer reliability
- **Model selection** — Choose different models for Q&A
- **Token tracking** — Monitor usage and costs

### Knowledge Graph
Structured knowledge extraction:

| Feature         | Description                                    |
|----------------|------------------------------------------------|
| entity types   | Define custom entity categories                |
| relation types | Define relationship patterns                   |
| auto-extraction| Extract entities and relations from documents  |
| graph query    | Query knowledge graph in natural language      |
| visualization  | Graph structure exploration                    |
| depth control  | Adjustable traversal depth for queries         |

- **Entity extraction** — Automatically identify entities in documents
- **Relation extraction** — Discover relationships between entities
- **Graph query** — Ask questions about graph structure
- **Multi-format output** — Graph, triples, or JSON format
- **Source grounding** — Link graph elements to source documents

## Key Capabilities

- **Multi-tenant isolation** — Complete separation of knowledge collections
- **Source attribution** — Full traceability to original documents
- **Version tracking** — Monitor document and collection changes
- **Scalable indexing** — Handle millions of vectors
- **Real-time updates** — Ingest and search documents in real-time
- **Configurable chunking** — Adapt to document types and use cases
- **Metadata filtering** — Filter search results by metadata
- **Batch processing** — Ingest and process documents in bulk

## API Endpoints

| Method   | Endpoint                                             | Description                        |
|----------|------------------------------------------------------|------------------------------------|
| POST     | `/api/v1/knowledge/collections`                      | Create collection                  |
| GET      | `/api/v1/knowledge/collections`                      | List collections                   |
| GET      | `/api/v1/knowledge/collections/{id}`                 | Get collection details             |
| PATCH    | `/api/v1/knowledge/collections/{id}`                 | Update collection                  |
| DELETE   | `/api/v1/knowledge/collections/{id}`                 | Delete collection                  |
| POST     | `/api/v1/knowledge/collections/{id}/documents`       | Upload document                    |
| GET      | `/api/v1/knowledge/collections/{id}/documents`       | List documents                     |
| GET      | `/api/v1/knowledge/collections/{id}/documents/{id}`  | Get document details               |
| DELETE   | `/api/v1/knowledge/collections/{id}/documents/{id}`  | Delete document                    |
| POST     | `/api/v1/knowledge/collections/{id}/search`          | Search collection                  |
| POST     | `/api/v1/knowledge/collections/{id}/qa`              | Ask question                       |
| POST     | `/api/v1/knowledge/graphs`                           | Create knowledge graph             |
| GET      | `/api/v1/knowledge/graphs`                           | List knowledge graphs              |
| GET      | `/api/v1/knowledge/graphs/{id}`                      | Get knowledge graph details        |
| DELETE   | `/api/v1/knowledge/graphs/{id}`                      | Delete knowledge graph             |
| POST     | `/api/v1/knowledge/graphs/{id}/query`                | Query knowledge graph              |
| GET      | `/api/v1/knowledge/indexes`                          | List vector indexes                |
| GET      | `/api/v1/knowledge/indexes/{id}/stats`               | Get index statistics               |
| GET      | `/api/v1/knowledge/health`                           | Health check                       |

## Integration with Modules

Knowledge base powers intelligent operations:

1. **Module 01** — Memory store for agent memories
2. **Module 04** — Agent Engine knowledge retrieval
3. **Module 05** — Task Planning contextual knowledge
4. **Module 06** — Data Ingestion document processing
5. **Module 09** — Supervision analysis knowledge base
6. **Module 10** — Policy & Governance policy documents
7. **Module 14** — Workflow Automation knowledge workflows

## Document Processing Example

```yaml
collection:
  name: "Product Documentation"
  embedding_model: "text-embedding-3-large"
  distance_metric: "cosine"
  index_type: "hnsw"

documents:
  - filename: "api-reference.pdf"
    type: "pdf"
    chunk_strategy: "paragraph"
    chunk_size: 300
    chunk_overlap: 50
    metadata:
      department: "engineering"
      access_level: "public"

  - filename: "internal-policies.md"
    type: "markdown"
    chunk_strategy: "fixed"
    chunk_size: 500
    chunk_overlap: 75
    metadata:
      department: "legal"
      access_level: "restricted"
```

## Search Example

```yaml
query: "How do I reset my password?"
top_k: 5
threshold: 0.75
filter:
  access_level: "public"
return_content: true
return_embeddings: false
```

## Q&A Example

```yaml
question: "What are the latest API changes?"
top_k: 3
model_id: "gpt-4o"
include_sources: true
answer_style: "cite_sources"
```

## Critical Risks

| Risk                        | Mitigation                                  |
|-----------------------------|---------------------------------------------|
| Embedding quality           | Model selection, chunking optimization        |
| Search accuracy             | Threshold tuning, hybrid search               |
| Source hallucination        | Source attribution, confidence scoring        |
| Vector storage scaling      | Index optimization, partitioning              |
| Document processing failures| Error handling, retry logic                   |
| Knowledge drift             | Versioning, periodic refresh                  |

## Module Dependencies

- **Module 12** — ML & Cognitive Services (embedding models)
- **Module 06** — Data Ingestion (document processing)
- **Module 04** — Agent Engine (knowledge retrieval)
- **Module 14** — Workflow Automation (knowledge workflows)
- **Module 10** — Policy & Governance (policy documents)

## Related Artifacts

- `contracts/v1/openapi-15-agent-marketplace.yaml` — OpenAPI specification
- `contracts/v1/schema-15-agent-marketplace.json` — JSON Schema definitions
