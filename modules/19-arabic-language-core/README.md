# Module 19 — Arabic Language Core

Operan's Arabic-native language processing module. Provides text normalization, dialect detection, terminology governance, Arabic embeddings, and Arabic OCR configuration.

## Overview

Arabic is not just a translation away from English. M19 ensures the platform handles Arabic text correctly at every level:

- **Text Normalization**: Convert Arabic text to a consistent canonical form (tashkeel removal, alef/hamza standardization, whitespace normalization)
- **Dialect Detection**: Determine whether text is Modern Standard Arabic (MSA) or which specific dialect
- **Terminology Governance**: Maintain approved government terminology glossaries; flag when unauthorized terms are used
- **Arabic Embeddings**: Delegate to M12 with Arabic-optimized embedding models
- **Arabic Tokenizer**: Word-level and character-level tokenization with RTL support

## Dialect Keywords

The dialect detector uses keyword-frequency-based classification with weighted scoring. Each dialect has distinguishing keywords:

| Dialect | Keywords | Weight Range |
|---------|----------|-------------|
| MSA (الفصحى) | ~30 formal words | 2.0–3.0 |
| Saudi | ~35 Gulf keywords | 2.0–4.0 |
| Emirati | ~30 keywords | 2.0–4.0 |
| Kuwaiti | ~30 keywords | 2.0–3.5 |
| Bahraini | ~30 keywords | 2.0–3.5 |
| Qatari | ~30 keywords | 2.0–3.5 |
| Omani | ~30 keywords | 2.0–3.5 |
| Egyptian | ~25 keywords | 2.0–4.0 |
| Levantine | ~25 keywords | 2.0–4.0 |
| Moroccan | ~25 keywords | 2.0–4.0 |

**Total**: ~290 unique keywords across 10 dialect models.

## API Endpoints

### Public (No Auth Required)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/v1/stats` | Module statistics |
| `POST` | `/v1/normalize` | Normalize Arabic text |
| `POST` | `/v1/detect-dialect` | Detect text dialect |

### Authenticated (Bearer JWT + X-Tenant-ID)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/terminology/check` | Check text against glossary |
| `GET` | `/v1/terminology/glossary` | List glossary entries |
| `POST` | `/v1/terminology/glossary` | Add term to glossary |
| `PATCH` | `/v1/terminology/glossary/{id}` | Update term |
| `DELETE` | `/v1/terminology/glossary/{id}` | Remove term |
| `POST` | `/v1/embeddings` | Get Arabic embeddings (delegates to M12) |

## Configuration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `HTTP_PORT` | Server listen port | `8019` |
| `JWT_SECRET` | JWT signing secret (required) | — |
| `M12_BASE_URL` | Module 12 base URL for embeddings | — |
| `DB_DSN` | PostgreSQL connection string | — |
| `EVENT_BROKER_URL` | Kafka event broker URL | — |

## Database

Run migrations before starting:

```bash
psql "$DB_DSN" -f modules/19-arabic-language-core/migrations/001_create_schema.sql
```

Creates 3 tables:
- `terminology_glossary` — approved government terminology
- `terminology_usage_log` — audit trail for terminology checks
- `arabic_embedding_requests` — M12 embedding call monitoring

## Kafka Events

| Topic | When Published |
|-------|---------------|
| `operan.arabic.text_normalized` | After normalization |
| `operan.arabic.dialect_detected` | After dialect detection |
| `operan.arabic.terminology_check` | After terminology check |
| `operan.arabic.terminology_violation` | When unauthorized term flagged |
| `operan.arabic.embedding_requested` | After embedding delegation |

## Building

```bash
cd modules/19-arabic-language-core
go mod tidy
go build ./...
go test ./...
```

## Running

```bash
HTTP_PORT=8019 \
JWT_SECRET=your-secret \
DB_DSN="postgres://..." \
M12_BASE_URL=http://localhost:8012 \
EVENT_BROKER_URL=kafka://localhost:9092 \
go run .
```

## Docker

```bash
docker build -t operan/arabic-language-core:latest .
docker run -p 8019:8019 operan/arabic-language-core:latest
```

## Helm

```bash
helm install arabic-language-core ./chart/ \
  --set "m12BaseUrl=http://m12-service:8012" \
  --set "image.repository=operan/arabic-language-core" \
  --set "image.tag=latest"
```

## Integration Points

- **M12 Model Abstraction**: Delegates Arabic embeddings to M12's `/v1/models/embeddings`
- **M03 Orchestration**: M03 calls M19 normalize/detect-dialect before processing agent tasks
- **M07 Memory**: M07 calls M19 normalize before storing Arabic text; embeddings for vector generation
- **M05 Template Engine**: M05 uses M19 to set default language settings per department
- **M21 Experience Portal**: M21 uses M19 for RTL rendering hints and dialect-aware UI text

## Tests

```bash
go test ./... -v -count=1
```

Runs 30+ tests covering:
- Normalizer (tashkeel removal, alef normalization, hamza, whitespace, punctuation)
- Dialect detector (MSA, all 6 Gulf dialects, mixed text, confidence)
- Terminology (CRUD, pagination, filtering, deprecated detection, alternatives matching)
- Embeddings (delegation, timeout, 5xx handling, missing M12 URL)
- Middleware (JWT validation, tenant isolation)
- Tokenizer (word and character level)
- Edge cases (empty text, non-Arabic, Unicode)