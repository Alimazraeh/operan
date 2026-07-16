# Module 06 — Knowledge Ingestion Pipeline

Document ingestion pipeline for the Operan platform. Ingests enterprise documents (PDFs, Word, Excel, text, HTML), extracts and normalizes text, segments into semantic chunks, generates embeddings, and stores them in the M07 vector store.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Document    │────>│  Text        │────>│  Arabic      │────>│  Adaptive    │────>│  M12         │
│  Source      │     │  Extraction  │     │  Normalizer  │     │  Chunker     │     │  Embedder    │
│  (file/URL)  │     │  (PDF/DOCX/  │     │  (M19)       │     │  (sections)  │     │  (embeddings)│
│              │     │   XLSX/TXT)  │     │              │     │              │     │              │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘     └──────┬───────┘
                                                                                           │
                                                                                           v
┌──────────────┐     ┌──────────────┐     ┌──────────────┐                            ┌──────────────┐
│  Results     │<────│  M07         │     │  Dedup       │                            │  Kafka       │
│  (DB)        │     │  Vector      │     │  (SHA-256)   │                            │  Events      │
│  chunk meta  │     │  Store (M07) │     │  hash check  │                            │  pub/sub     │
└──────────────┘     └──────────────┘     └──────────────┘                            └──────────────┘
```

## Pipeline Flow

1. **Source Management**: Register document sources (file upload, SharePoint, email, web crawl, S3, direct URL)
2. **Ingestion Trigger**: POST /v1/ingest creates an async job for a registered source
3. **Extraction**: PDF (pdfcpu), DOCX (go-docx), XLSX (excelize), TXT (raw), HTML (goquery)
4. **Arabic Normalization** (optional): Calls M19 /v1/normalize for text normalization
5. **Chunking**: Adaptive, fixed, by-heading, or by-paragraph strategies
6. **Deduplication**: SHA-256 hash-based chunk deduplication
7. **Embedding**: Calls M12 /v1/models/embeddings for each chunk
8. **Vector Storage**: Writes vectors + metadata to M07 vector store
9. **Result Recording**: Stores chunk metadata in local DB (vectors go to M07)
10. **Event Publishing**: Kafka events at each pipeline stage

## Features

- **Multi-format support**: PDF, DOCX, XLSX, TXT, HTML
- **Intelligent chunking**: 4 strategies — adaptive (default), fixed, by-heading, by-paragraph
- **Arabic normalization**: Optional M19 integration for Arabic text
- **Content deduplication**: SHA-256 chunk-level deduplication
- **Async processing**: Background worker with progress tracking
- **Job recovery**: Resumes pending/failing jobs on startup
- **Tenant isolation**: JWT-based auth with tenant header validation
- **Kafka events**: Full pipeline observability via event stream

## Database Schema

Three tables managed by migration `migrations/001_create_schema.sql`:

| Table | Purpose |
|-------|---------|
| `ingestion_sources` | Document source registry with chunking config |
| `ingestion_jobs` | Async ingestion job tracking with progress |
| `ingestion_results` | Per-chunk metadata (text, hash, status) |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check (unauthenticated) |
| GET | `/v1/sources` | List sources (paginated) |
| POST | `/v1/sources` | Register new source |
| GET | `/v1/sources/{id}` | Source detail |
| PATCH | `/v1/sources/{id}` | Update source config |
| DELETE | `/v1/sources/{id}` | Delete source |
| POST | `/v1/ingest` | Trigger ingestion for a source |
| GET | `/v1/jobs` | List ingestion jobs |
| GET | `/v1/jobs/{id}` | Job status with progress |
| POST | `/v1/jobs/{id}/cancel` | Cancel running job |

Full OpenAPI contract: `contracts/openapi-06-knowledge-ingestion.yaml`

## Configuration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `IAM_TOKEN_SECRET` | JWT signing secret (required) | — |
| `HTTP_PORT` | Server listen port | `8006` |
| `DB_DSN` | PostgreSQL connection string | — |
| `EVENT_BROKER_URL` | Kafka broker URL | — |
| `M12_BASE_URL` | Model Abstraction Layer (embeddings) | — |
| `M07_BASE_URL` | Memory Fabric (vector store) | — |
| `M19_BASE_URL` | Arabic Language Core (normalization) | — |
| `PROVIDER_API_KEYS` | JSON array of provider API keys | `[]` |

## Chunking Strategies

| Strategy | Description | Best For |
|----------|-------------|----------|
| `adaptive` (default) | Detects natural boundaries (headings, sections), fills chunks with overlap | Mixed documents |
| `fixed` | Equal-size chunks with overlap | Technical docs, code |
| `by_heading` | Splits at ## and ### headings, keeps full sections | Markdown, articles |
| `by_paragraph` | Splits at double newlines, merges short paragraphs | General text |

## Building

```bash
cd modules/06-knowledge-ingestion
go build ./...
go test ./...
```

## Running

```bash
# 1. Run migrations
psql "$DB_DSN" -f migrations/001_create_schema.sql

# 2. Start the service
IAM_TOKEN_SECRET=your-secret \
DB_DSN="postgres://user:pass@localhost:5432/operan" \
M12_BASE_URL="http://localhost:8012" \
M07_BASE_URL="http://localhost:8007" \
HTTP_PORT=8006 \
go run .
```

## Deploying with Helm

```bash
helm install operan-m06 ./chart/ \
  --set env.DB_DSN="postgres://..." \
  --set env.IAM_TOKEN_SECRET="your-secret" \
  --set env.M12_BASE_URL="http://operan-m12:8012" \
  --set env.M07_BASE_URL="http://operan-m07:8007"
```

## Kafka Events

| Topic | Triggered When |
|-------|---------------|
| `operan.knowledge.ingestion_started` | Job created |
| `operan.knowledge.chunk_created` | Each chunk extracted |
| `operan.knowledge.embedding_stored` | Each chunk embedded |
| `operan.knowledge.ingestion_completed` | Job finishes successfully |
| `operan.knowledge.ingestion_failed` | Job fails |

## Integration Points

| Module | Integration |
|--------|------------|
| M12 (Model Abstraction) | Embedding generation via `/v1/models/embeddings` |
| M19 (Arabic Language Core) | Arabic text normalization via `/v1/normalize` |
| M07 (Memory Fabric) | Vector storage via `/v1/vectors` |
| M03 (Orchestration) | Searches M07 vectors for document-aware responses |
| M21 (Experience Portal) | Displays ingestion progress to users |

## Testing

```bash
go test -v ./...
```

Tests cover: extraction engines, chunking strategies, worker pipeline, source/job CRUD, middleware auth, client integrations, and event publishing.