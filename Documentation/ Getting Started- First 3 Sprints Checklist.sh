# Sprint 1 Focus: Foundation
☐ Finalize ADRs for: tenant isolation, agent runtime model, memory abstraction
☐ Set up mono-repo with pod-based directory structure
☐ Deploy dev Kubernetes cluster with namespace isolation POC
☐ Integrate first 2 LLM providers in Model Abstraction Layer
☐ Define OpenAPI spec for Tenant Control Plane

# Sprint 2 Focus: Runtime Skeleton
☐ Implement minimal DAG executor (in-memory)
☐ Create agent registration API + metadata schema
☐ Build vector store abstraction with Qdrant POC
☐ Add basic approval webhook for human-in-the-loop

# Sprint 3 Focus: End-to-End Smoke Test
☐ Deploy "Hello Research" workflow: ingest PDF → summarize → draft report → require approval
☐ Emit OpenTelemetry traces for full execution path
☐ Validate tenant A/B isolation (data, execution, logs)
☐ Demo to stakeholders with Arabic text sample