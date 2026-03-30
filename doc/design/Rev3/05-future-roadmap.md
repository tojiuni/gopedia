# 05. Future Roadmap (Post-Rev3)

Rev3 introduces agent-facing payload controls. The next phase should focus on performance guarantees and richer contract ergonomics.

## 1. Search Contract Evolution
* **Contract versioning**: add optional API version marker for long-lived agent clients.
* **Preset telemetry**: track `summary/standard/full` usage and latency to tune defaults.
* **Fieldset profiles**: define reusable named profiles for common agent tasks (citation, grounding, answer drafting).

## 2. Retrieval Quality
* **Adaptive rerank policy**: dynamic rerank enablement based on hit confidence spread.
* **Project-specific retrieval policy**: expose stronger per-project controls for context depth and rerank model.
* **Deterministic grounding mode**: force evidence-first answer templates for regulated workflows.

## 3. Operational Hardening
* **Compose/build reliability**: standardize BuildKit/buildx and credential helper strategy across local setups.
* **Search SLO reporting**: publish p95 latency by `detail` preset.
* **Error taxonomy expansion**: broaden structured `failure.code` coverage for client retry automation.
