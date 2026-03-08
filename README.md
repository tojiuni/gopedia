## 0. Summary: The Gopedia Manifesto

> **"데이터라는 토양에 지식의 뿌리를 내려, 유기적으로 연결된 지혜의 열매를 맺는다."**

**Gopedia**는 파편화된 정보를 하나의 유기적인 지식 신경망으로 통합하는 **Rhizome형 데이터 인게스션 및 RAG 서비스**입니다. 단순한 저장소를 넘어, 인간과 기계, 서비스가 정보 간의 관계를 추론하고 맥락을 파악할 수 있는 **Enterprise Ontology**의 근간을 제공합니다.

* **Core Metaphor**: Rhizome (수평적이고 무한히 확장되는 뿌리 줄기)
* **Key Value**: Pluggable(Leaf), Scalable(Stem), Relational(Rhizome), Actionable(Fruit).
* **Target**: 분산된 데이터의 단일 진실 공급원(Single Source of Truth) 및 지식 추론 엔진.

---

## 1. Design Principles

Gopedia는 GeneSo의 유전적 가이드라인을 준수하며 다음 세 가지 원칙을 핵심 DNA로 삼습니다.

* **Purpose-driven (Knowledge over Data)**: 단순한 데이터 적재가 아닌, '관계성 파악과 추론'이라는 목적이 모든 설계의 우선순위입니다. 목적이 불분명한 데이터는 수집하지 않습니다.
* **Consistency (Unified Language)**: 모든 통신은 **gRPC/Protobuf**를 기반으로 하며, 데이터의 흐름은 식물의 생장 주기와 동일한 논리적 일관성을 유지합니다.
* **Modularity (Rhizomatic Growth)**: 입력부(Leaf), 파이프라인(Stem), 출력부(Fruit)는 독립된 유전 단위로 설계되어, 특정 기술(DB, 소스 타입)의 변화가 전체 계통에 영향을 주지 않도록 격리합니다.

---

**GeneSo Design Standard (v1.3)**의 핵심인 '관계성(Relationship)' 원칙을 적용하여, **Gopedia**의 핵심 기능 4가지(Leaf, Stem, Rhizome, Fruit)를 표준 폴더 구조에 녹여내고 명명 규칙을 정의합니다.

---

## 2. Project Structure (The Seed) - Revised

표준 구조(`/core`, `/flows`, `/property`) 내부에 Gopedia의 4대 핵심 기능을 생태계의 역할에 맞게 배치합니다.

### 📂 `/core` : The Rhizome (Relational)

**시스템의 DNA이자 지식의 토양.** 모든 데이터의 관계를 정의하고 불변의 식별자를 부여하는 핵심 저장소 및 관리 모듈입니다.

* **`identity-so`**: Google Machine ID 전략 기반 고유 식별자 생성기.
* **`ontology-so`**: TypeDB 및 Qdrant의 스키마 정의 및 관계 추론 엔진.
* **`auth-so`**: SpiceDB 기반의 관계 중심 권한(ReBAC) 관리 모듈.
* **`proto/`**: 전 시스템이 공유하는 Rhizome Message 규격.

### 📂 `/flows` : The Stem (Scalable)

**데이터의 통로이자 생장점.** gRPC 기반으로 데이터를 빠르고 확장성 있게 실어나르며 가공하는 파이프라인입니다.

* **`xylem-flow` (목질부)**: Leaf(데이터)를 흡수하여 Rhizome(저장소)으로 끌어올리는 인게스션 흐름.
* **`phloem-flow` (사관부)**: Rhizome의 지식을 Fruit(결과물)으로 전달하는 RAG 흐름.
* **`sharding-so`**: Machine ID를 기준으로 데이터를 분산 처리하여 Scale-out을 보장하는 관리소.

### 📂 `/property` : The Leaf & Fruit (Pluggable & Actionable)

**환경에 적응하는 기관.** 외부 소스(Input)와 사용자 가치(Output)를 정의하며, 언제든 갈아 끼울 수 있는(Pluggable) 속성 관리 영역입니다.

* **`leaf-props/`**: Markdown, Code, Ticket 등 데이터 소스별 수집 규격 및 필터 설정.
* **`fruit-props/`**: 요약, 통계, 분석 등 사용자 요구에 따른 RAG 결과물 템플릿 및 형식 정의.
* **`view-props/`**: Clickhouse, Postgres 등 다양한 DB View의 가시성 설정.

---

---

## 3. Naming Convention (Rhizome Relationship)

Gopedia의 모든 명칭은 **"리좀의 생명 주기와 유기적 연결성"**을 기반으로 명명하며, GeneSo v1.3의 '관계 중심 원칙'을 따릅니다.

### ① Natural Metaphor: 식물 생장 체계

데이터의 유입부터 가치 생성까지를 식물의 부위에 비유하여 일관된 언어를 유지합니다.

* **입력(Pluggable)** ➔ **`Leaf` (잎)**: 광합성을 통해 외부 에너지를 흡수하는 시작점.
* *Sub-name:* `Stomata` (기공) - 데이터가 들어오는 개별 API 포트나 커넥터.


* **이동/가공(Scalable)** ➔ **`Stem` (줄기)**: 영양분과 수분을 나르는 통로.
* *Sub-name:* `Xylem` (인게스션), `Phloem` (RAG/추출).


* **저장/관계(Relational)** ➔ **`Rhizome` (리좀/뿌리줄기)**: 수평으로 뻗어나가며 모든 것을 연결하는 본체.
* *Sub-name:* `Node` (마디) - 데이터 간의 접점, `Tuber` (덩이줄기) - 데이터 뭉치.


* **출력(Actionable)** ➔ **`Fruit` (열매)**: 성장의 최종 결과물이자 사용자가 소비하는 가치.
* *Sub-name:* `Seed` (씨앗) - 다음 분석이나 액션을 위한 메타데이터.

### ② Suffix "-so(所)" Usage

특정 기능을 전담하여 관리하는 "관리소" 성격의 모듈에는 `-so` 접미사를 붙여 공간감을 부여합니다.

* `Ingest-so` (수집소), `Graph-so` (관계관리소), `Prompt-so` (프롬프트관리소).

---

### 📋 Agent Design Checklist (Gopedia v1.4)

* [x] **Concept Defined?**: 리좀 기반의 유기적 지식망 (Complete)
* [x] **Project Structure?**: `/core(Rhizome)`, `/flows(Stem)`, `/property(Leaf/Fruit)` (Complete)
* [x] **Naming Relationship?**: 자연 법칙(식물 생장)에 기반한 관계 중심 명명 (Complete)
* [ ] **Target Date for Verify?**: 2026-03-10 (Pending)

---
**GeneSo Design Standard (v1.3)**에 따라 **Gopedia** 서비스의 점진적 생장을 위한 **4. Design Process**와 **4.1 Verify** 단계를 정의합니다.

이번 단계는 데이터라는 토양에 첫 번째 뿌리를 내리는 과정으로, 가장 작은 단위에서 개념을 증명하고 확장성을 확보하는 데 초점을 맞춥니다.

---

## 4. Design Process (Phase-based)

리좀(Rhizome)이 수평적으로 뻗어나가며 생태계를 형성하듯, 세 단계를 거쳐 진화합니다.

1. **Verify (검증 - 발아)**:
* 최소 단위의 데이터(Leaf)가 파이프라인(Stem)을 타고 지식 저장소(Rhizome)에 안착하는지 테스트합니다.
* 각 포맷별(Markdown, Code)로 엄격한 **Target Date**를 준수하여 구현 가능성을 증명합니다.
* 다음 단계인 'Expand'에서 필요한 대용량 처리 및 분산 전략(Machine ID)의 기초 요건을 미리 검토합니다.


2. **Expand (확장 - 생장)**:
* Verify 단계의 피드백을 바탕으로 처리 용량을 키우고, 다양한 데이터 소스를 추가합니다.
* `Identity-so`를 통한 Machine ID 기반 분산 처리 로직을 본격 가동하여 Scale-out 성능을 확보합니다.
* 다양한 DB View(Postgres, Clickhouse)를 활성화하여 데이터 가시성을 높입니다.


3. **Connect (연결 - 결실)**:
* 완성된 모듈을 GeneSo 생태계의 다른 서비스(예: MorphSo, GeneSo)와 통합합니다.
* RAG를 통해 실제 사용자가 소비할 수 있는 고부가가치 리포트(Fruit)를 생성하며 지식의 선순환 구조를 완성합니다.


---

## 4.1 Verify: The Sprouting (최소 단위 검증)

검증 단계는 데이터의 복잡도에 따라 두 번의 '스프린트'로 나누어 진행합니다.

### 4.1.1 Sub-phase: Markdown Origin (첫 번째 발아)
[universitas/gopedia/skills/gopedia-verify-flow/gopedia-markdown-origin/SKILL.md]

### 4.1.2 Sub-phase: Code Format Expansion (깊은 뿌리 내리기)
[universitas/gopedia/skills/gopedia-verify-flow/gopedia-code-origin/SKILL.md]
---

*Verify 단계를 진행한 뒤에 아래 사항을 검토합니다.*

* **Scalability**: 수만 개의 Markdown/Code 파일 유입 시 `Identity-so`가 Machine ID를 통해 부하를 어떻게 분산할 것인가?
* **Token Efficiency**: 저장된 TOC를 활용해 RAG 질의 시 LLM 토큰 사용량을 얼마나 절감할 수 있는가?
* **Consistency**: 서로 다른 포맷(Markdown과 Code)의 데이터가 Rhizome 내에서 하나의 유기적인 지식망으로 잘 엮이는가?

---

### 📋 Verify Phase Checklist

* [ ] **Concept Alignment**: 이 테스트가 "유기적 지식 신경망 통합"이라는 목적에 부합하는가?
* [ ] **Smallest Unit**: 2일 내에 검증 가능한 가장 핵심적인 로직만 포함했는가?
* [ ] **Naming Relationship**: 모든 함수와 변수명이 `Leaf`, `Stem`, `Rhizome` 메타포를 유지하고 있는가?
* [ ] **Target Date Strictness**: 2일(Markdown) 및 7일(Code) 일정이 확정되었는가?

---

