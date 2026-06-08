# Module 12 — ML & Cognitive Services Engine

**AI intelligence layer.**

---

## Purpose

The ML & Cognitive Services Engine is the intelligence backbone of the Operan ecosystem. It provides managed AI model access, prompt engineering infrastructure, vector search capabilities, fine-tuning services, and pre-built cognitive AI functions that all other modules consume.

## Core Functions

### Model Management
Unified interface to AI models across providers:

| Provider   | Models Available                              |
|-----------|----------------------------------------------|
| OpenAI    | GPT-4o, GPT-4, GPT-3.5, embeddings          |
| Anthropic | Claude 3.5, Claude 3, embeddings             |
| Google    | Gemini, PaLM, embeddings                     |
| AWS       | Bedrock models, embeddings                   |
| Azure     | OpenAI models, GPT deployments               |
| Ollama    | Local/open-source models                     |
| Custom    | Self-hosted or custom endpoints              |

- **Model routing** — Automatic failover between providers
- **Cost optimization** — Select cheapest model that meets quality thresholds
- **Performance tracking** — Latency and quality comparison per model
- **Multi-model evaluation** — A/B test models for specific tasks

### Prompt Engineering
Versioned prompt template management:

- **Template variables** — Parametric prompts with injection
- **Category organization** — Chat, extract, classify, summarize, translate, code, reasoning
- **Versioning** — Track prompt changes with rollout control
- **A/B testing** — Compare prompt effectiveness
- **Render API** — Resolve templates with variables at runtime

### Embeddings & Vector Search
Semantic search infrastructure:

- **Embedding generation** — Multi-provider embedding models
- **Vector store management** — Create, update, query vector collections
- **Similarity search** — Cosine, dot product, euclidean distance
- **Metadata filtering** — Combined vector + metadata queries
- **Dimension control** — Configurable embedding dimensions

### Fine-Tuning Services
Custom model training infrastructure:

- **Job management** — Create, monitor, cancel fine-tuning jobs
- **Hyperparameter configuration** — Epochs, learning rate, batch size
- **Dataset management** — Upload and version training data
- **Progress tracking** — Real-time loss and accuracy metrics
- **Output management** — Promote fine-tuned models to active

### AI Agent Cognitive Services
Pre-built AI functions for common tasks:

| Task           | Description                                 | Output                          |
|---------------|---------------------------------------------|---------------------------------|
| Translate     | Multi-language translation                  | Translated text                 |
| Sentiment     | Sentiment analysis with confidence          | Positive/negative/neutral       |
| Keywords      | Keyword/phrase extraction                   | Ranked list of terms            |
| Topics        | Topic classification                        | Category assignment             |
| Tone          | Tone/style analysis                         | Tone profile                    |
| Reasoning     | Chain-of-thought reasoning                  | Reasoned output                 |
| Planning      | Action planning and decomposition           | Structured plan                 |
| Code Generate | Code generation with context                | Executable code                 |
| Code Review   | Code quality assessment                     | Review with suggestions         |
| Data Analyze  | Statistical analysis and insights           | Analysis report                 |
| Knowledge     | Question answering from knowledge base      | Answered response               |

## Key Capabilities

- **Provider abstraction** — Single API for all AI models
- **Token tracking** — Cost and usage attribution per tenant
- **Response caching** — Cache repeated prompts for performance
- **Rate limiting** — Per-tenant and per-model request throttling
- **Quality scores** — Post-hoc quality assessment of model outputs
- **Multi-modal** — Support for text, code, images, speech

## API Endpoints

| Method   | Endpoint                                     | Description                        |
|----------|----------------------------------------------|------------------------------------|
| POST     | `/api/v1/ai/models`                          | Register a model                   |
| GET      | `/api/v1/ai/models`                          | List available models              |
| GET      | `/api/v1/ai/models/{id}`                     | Get model details                  |
| PATCH    | `/api/v1/ai/models/{id}`                     | Update model                       |
| DELETE   | `/api/v1/ai/models/{id}`                     | Remove model                       |
| POST     | `/api/v1/ai/models/{id}/invoke`              | Invoke a model                     |
| POST     | `/api/v1/ai/models/{id}/compare`             | Compare multiple models            |
| POST     | `/api/v1/ai/models/{id}/evaluate`            | Evaluate model performance         |
| POST     | `/api/v1/ai/prompts`                         | Create prompt template             |
| GET      | `/api/v1/ai/prompts`                         | List prompt templates              |
| GET      | `/api/v1/ai/prompts/{id}`                    | Get prompt template                |
| PATCH    | `/api/v1/ai/prompts/{id}`                    | Update prompt template             |
| DELETE   | `/api/v1/ai/prompts/{id}`                    | Delete prompt template             |
| POST     | `/api/v1/ai/prompts/{id}/render`             | Render prompt with variables       |
| POST     | `/api/v1/ai/embeddings`                      | Generate embeddings                |
| POST     | `/api/v1/ai/vector-store/query`              | Search vector store                |
| DELETE   | `/api/v1/ai/vector-store/{store_id}`         | Delete vector store                |
| POST     | `/api/v1/ai/fine-tuning`                     | Create fine-tuning job             |
| GET      | `/api/v1/ai/fine-tuning`                     | List fine-tuning jobs              |
| GET      | `/api/v1/ai/fine-tuning/{job_id}`            | Get fine-tuning job status         |
| DELETE   | `/api/v1/ai/fine-tuning/{job_id}`            | Cancel fine-tuning job             |
| POST     | `/api/v1/ai/agents/invoke`                   | Invoke AI agent for task           |
| POST     | `/api/v1/ai/agents/summarize`                | Summarize content                  |
| POST     | `/api/v1/ai/agents/classify`                 | Classify content                   |
| POST     | `/api/v1/ai/agents/extract`                  | Extract entities                   |

## Integration with Modules

All modules consume AI services:

1. **Module 06** — Data Ingestion uses embeddings for vectorization
2. **Module 07** — Memory uses embeddings for retrieval
3. **Module 08** — Tools use AI for classification and extraction
4. **Module 09** — Supervision uses AI for risk analysis
5. **Module 11** — Observability uses AI for anomaly classification
6. **Module 15** — Marketplace uses AI for recommendation
7. **Module 19** — Integration Hub uses AI for data transformation

## Model Selection Strategy

- **Cost-aware routing** — Select model based on task complexity vs. cost
- **Latency-aware routing** — Use faster models for time-sensitive tasks
- **Quality-aware routing** — Use higher-quality models for critical decisions
- **Provider diversity** — Avoid vendor lock-in with multi-provider strategy

## Critical Risks

| Risk                        | Mitigation                                  |
|-----------------------------|---------------------------------------------|
| Model quality variance      | Benchmarking, fallback models, quality scoring |
| Cost escalation             | Token tracking, budget limits, caching      |
| Prompt injection attacks    | Input sanitization, model guardrails        |
| Provider downtime           | Multi-provider failover, local fallback     |
| Data privacy                | PII filtering, data anonymization           |

## Module Dependencies

- **Module 06** — Data Ingestion (embedding generation)
- **Module 07** — Memory (semantic search)
- **Module 08** — Tools (AI-powered tool orchestration)
- **Module 10** — Policy & Governance (AI safety policies)
- **Module 17** — Cost Governance (model cost attribution)

## Related Artifacts

- `contracts/v1/openapi-12-model-abstraction.yaml` — OpenAPI specification
- `contracts/v1/schema-12-model-abstraction.json` — JSON Schema definitions
