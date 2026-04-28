# Gopedia — Kubernetes 배포 가이드

## 개요

Gopedia 서비스를 K8s에 배포하는 방법을 설명합니다.  
Postgres, Qdrant는 클러스터에 이미 존재하는 것을 재사용하며 **신규 PVC를 만들 필요 없습니다.**

| 의존성 | 주소 | 필수 여부 |
|--------|------|-----------|
| PostgreSQL | `postgres-gopedia-rw.taxon.svc:5432` | 필수 |
| Qdrant | `qdrant.taxon.svc:6333/6334` | 필수 |
| Ollama (임베딩) | `ollama-embed.ai-assistant.svc:11434` | 필수 |
| TypeDB | `typedb.taxon.svc:1729` | 선택 — `TYPEDB_HOST` 미설정 시 자동 스킵 |
| `/morphogen` 볼륨 | PVC | 선택 — 파일 일괄 ingestion 시에만 필요 |

---

## 1. 사전 조건

- Woodpecker CI가 gopedia 리포지토리를 추적하도록 설정
- Woodpecker Org/Repo Secrets 등록 (섹션 5 참고)
- Vault Agent Injector가 클러스터에 설치되어 있음

---

## 2. Vault 설정

### 2-1. 시크릿 경로 추가

`POSTGRES_PASSWORD`는 기존 `secret/neunexus/postgres` 경로를 재사용합니다.  
`OPENAI_API_KEY`는 지금 placeholder 값을 넣고, 실제 사용 시 교체합니다.

```bash
vault kv put secret/neunexus/gopedia-svc \
  openai-api-key="placeholder"
```

### 2-2. Vault 정책 생성

```bash
vault policy write gopedia-svc - <<'EOF'
path "secret/data/neunexus/postgres" {
  capabilities = ["read"]
}
path "secret/data/neunexus/gopedia-svc" {
  capabilities = ["read"]
}
EOF
```

### 2-3. Kubernetes Auth 역할 생성

```bash
vault write auth/kubernetes/role/gopedia-svc \
  bound_service_account_names=gopedia-svc \
  bound_service_account_namespaces=gopedia-svc \
  policies=gopedia-svc \
  ttl=1h
```

---

## 3. 임베딩: Ollama

gopedia-svc는 OpenAI 클라이언트가 Ollama의 OpenAI 호환 API를 바라보도록 설정되어 있습니다.

| 환경변수 | 값 |
|----------|----|
| `OPENAI_BASE_URL` | `http://ollama-embed.ai-assistant.svc:11434/v1` |
| `OPENAI_EMBEDDING_MODEL` | `nomic-embed-text` |
| `OPENAI_API_KEY` | Vault placeholder (실제 OpenAI 사용 시 교체) |

Ollama에 `nomic-embed-text` 모델이 pull되어 있어야 합니다:

```bash
# ollama-embed pod에서 실행
kubectl exec -n ai-assistant deploy/ollama -- ollama pull nomic-embed-text
```

---

## 4. CI/CD 파이프라인

Woodpecker + Dagger 엔진으로 자동 빌드 및 배포합니다.

### 트리거 조건

| 이벤트 | 동작 |
|--------|------|
| PR 생성/업데이트 | `validate-pr` — 빌드만, push 없음 |
| `main` 브랜치 push (PR merge) | `build-push` → `deploy` 순서 실행 |
| Manual trigger | `build-push` → `deploy` 순서 실행 |

### 파이프라인 흐름

```
PR 이벤트:
  validate-pr  ← Dagger: docker build (no push)

main push / manual:
  build-push   ← Dagger: docker build + push to artifacts.toji.homes
       ↓
  deploy       ← kubectl apply + set image + rollout status
```

---

## 5. Woodpecker Secrets 등록

Woodpecker UI → 리포지토리 Settings → Secrets에서 등록합니다.

| Secret 이름 | 내용 |
|-------------|------|
| `REGISTRY_TOKEN` | `artifacts.toji.homes` woodpecker-ci 유저 API 토큰 |
| `KUBECONFIG_DATA` | base64 인코딩된 kubeconfig |

`KUBECONFIG_DATA` 생성:

```bash
kubectl config view --raw --minify | base64 | tr -d '\n'
```

---

## 6. 수동 배포 (최초 1회 또는 긴급)

Woodpecker가 없는 환경에서 수동으로 배포할 경우:

```bash
# 1. 이미지 빌드 & 푸시
docker build -t artifacts.toji.homes/neunexus/gopedia-svc:latest .
docker push artifacts.toji.homes/neunexus/gopedia-svc:latest

# 2. K8s 적용
kubectl apply -f deploy/k8s/gopedia-svc.yaml

# 3. 상태 확인
kubectl rollout status deployment/gopedia-svc -n gopedia-svc
```

---

## 7. 환경 변수 전체 목록

| 변수 | 값 | 출처 |
|------|----|------|
| `GOPEDIA_HTTP_ADDR` | `0.0.0.0:18787` | manifest |
| `GOPEDIA_PHLOEM_GRPC_ADDR` | `0.0.0.0:50051` | manifest |
| `GOPEDIA_LOG_LEVEL` | `info` | manifest |
| `GOPEDIA_REPO_ROOT` | `/app` | manifest |
| `POSTGRES_HOST` | `postgres-gopedia-rw.taxon.svc` | manifest |
| `POSTGRES_PORT` | `5432` | manifest |
| `POSTGRES_USER` | `app` | manifest |
| `POSTGRES_PASSWORD` | — | Vault: `neunexus/postgres` → `app-password` |
| `POSTGRES_DB` | `app` | manifest |
| `POSTGRES_SSLMODE` | `disable` | manifest |
| `QDRANT_HOST` | `qdrant.taxon.svc` | manifest |
| `QDRANT_PORT` | `6333` | manifest |
| `QDRANT_GRPC_PORT` | `6334` | manifest |
| `QDRANT_COLLECTION` | `gopedia_markdown` | manifest |
| `QDRANT_VECTOR_NAME` | `""` | manifest |
| `OPENAI_BASE_URL` | `http://ollama-embed.ai-assistant.svc:11434/v1` | manifest |
| `OPENAI_EMBEDDING_MODEL` | `nomic-embed-text` | manifest |
| `OPENAI_API_KEY` | placeholder | Vault: `neunexus/gopedia-svc` → `openai-api-key` |
| `TYPEDB_HOST` | 미설정 | 필요 시 manifest 주석 해제 |

---

## 8. 헬스체크

```bash
# HTTP API 상태
kubectl exec -n gopedia-svc deploy/gopedia-svc -- \
  wget -qO- http://localhost:18787/api/health

# 의존성 상세 (Postgres, Qdrant, TypeDB 각각)
kubectl exec -n gopedia-svc deploy/gopedia-svc -- \
  wget -qO- http://localhost:18787/api/health/deps
```

정상 응답 예시:

```json
{
  "status": "ok",
  "deps": {
    "postgres":    {"status": "ok"},
    "qdrant":      {"status": "ok"},
    "typedb":      {"status": "error", "error": "connection refused"},
    "phloem_grpc": {"status": "ok"}
  }
}
```

TypeDB는 선택적 의존성이므로 `error` 상태여도 서비스 정상 운영에 영향 없습니다.

---

## 9. neunexus 봇 생태계 연결

클러스터 내부에서 gopedia-svc 접근 주소:

| 프로토콜 | 주소 |
|----------|------|
| HTTP API | `http://gopedia-svc.gopedia-svc.svc.cluster.local:18787` |
| gRPC (Phloem) | `gopedia-svc.gopedia-svc.svc.cluster.local:50051` |

gopedia-bot이 이 서비스를 프록시로 사용하도록 업데이트하려면,  
gopedia-bot의 `GOPEDIA_SVC_URL` 환경변수를 위 HTTP 주소로 설정하고  
bot-registry ConfigMap에 서비스 엔드포인트를 추가합니다.

---

## 10. /morphogen 볼륨 (선택)

`/morphogen`은 API 서버 실행에 **불필요**합니다.  
마크다운 파일을 파이프라인으로 일괄 ingestion할 때만 필요합니다.

`deploy/k8s/gopedia-svc.yaml`에서 PVC 관련 주석을 해제하고  
적절한 `storageClassName`을 설정한 후 `kubectl apply`를 재실행하세요.

---

## 11. 트러블슈팅

### Vault 시크릿 `<no value>` 오류

Vault 템플릿에서 하이픈이 포함된 키는 `.Data.data.key-name` 형태가 아닌  
`index .Data.data "key-name"` 형태를 사용해야 합니다.

```yaml
# 잘못된 예
export POSTGRES_PASSWORD="{{ .Data.data.app-password }}"

# 올바른 예
export POSTGRES_PASSWORD="{{ index .Data.data "app-password" }}"
```

### Ollama 임베딩 실패

`nomic-embed-text` 모델이 pull되어 있는지 확인합니다:

```bash
kubectl exec -n ai-assistant deploy/ollama -- ollama list
```

모델이 없으면:

```bash
kubectl exec -n ai-assistant deploy/ollama -- ollama pull nomic-embed-text
```

### 배포 후 이전 이미지로 실행됨

`kubectl set image` 이후 `imagePullPolicy: Always`가 필요할 수 있습니다.  
manifest에 `imagePullPolicy: Always`를 추가하거나 SHA 태그로 배포하세요  
(CI 파이프라인의 `deploy` step은 SHA 태그를 사용하므로 자동 처리됩니다).
