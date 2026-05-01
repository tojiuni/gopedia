"""Hierarchical RAG Answer Agent.

LLM이 tool calling으로 gopedia 지식 계층(l3→l2→l1)을 탐색하며
질문에 답할 수 있는 내용을 찾아 합성 답변을 반환한다.

탐색 순서:
  1. search(query) → l3 hits (snippet + score)
  2. LLM 평가 → 부족하면 restore_l2(l2_id)
  3. LLM 평가 → 부족하면 restore_l1(l1_id)
  4. 있으면 answer(content), 없으면 not_found()

환경변수:
  OLLAMA_CHAT_URL   - Ollama base URL (기본: http://localhost:11434)
  OLLAMA_CHAT_MODEL - 모델명 (기본: gemma4:26b)
"""

from __future__ import annotations

import json
import logging
import os
from typing import Any

log = logging.getLogger(__name__)

# ── 설정 ────────────────────────────────────────────────────────────────────
OLLAMA_CHAT_URL = os.environ.get("OLLAMA_CHAT_URL", "http://localhost:11434")
OLLAMA_CHAT_MODEL = os.environ.get("OLLAMA_CHAT_MODEL", "gemma4:26b")
MAX_ITERATIONS = 8  # 무한루프 방지

SYSTEM_PROMPT = """당신은 내부 문서 검색 전문가입니다.
주어진 질문에 답하기 위해 gopedia 지식 베이스를 탐색합니다.

반드시 다음 도구를 순서대로 사용하세요:
1. search 도구로 관련 문서 조각(l3)을 검색하세요.
2. 검색 결과(snippet + context + l2_summary)로 질문의 핵심에 답할 수 있으면 즉시 answer 도구를 호출하세요.
3. 결과가 불충분하면 restore_l2 도구로 관련 섹션 전체를 복원하세요.
4. 여전히 불충분하면 restore_l1 도구로 전체 문서를 복원하세요.
5. 모든 탐색 후에도 관련 정보가 없으면 not_found 도구를 호출하세요.

중요 규칙:
- 반드시 항상 tool을 호출해야 합니다. 직접 텍스트로 답하지 마세요.
- answer 또는 not_found 호출로 반드시 종료해야 합니다.
- search는 최대 3회까지 호출할 수 있습니다. 첫 검색 결과가 불충분하면 다른 키워드로 재검색하세요. 3회 후에도 관련 결과가 없으면 not_found를 호출하세요.
- restore(l2/l1)는 search 재시도보다 나중에 사용하세요. search로 관련 섹션을 먼저 특정한 뒤 restore를 호출하세요.
- snippet + context + l2_summary로 질문의 70% 이상 답할 수 있으면 restore 없이 즉시 answer를 호출하세요.
- restore는 구체적인 명령어·설정값·수치가 snippet에 명시되지 않은 경우에만 사용하세요.
- 답변은 질문자가 이해하기 쉽게 한국어로 작성하세요.
- 출처(문서명, 섹션)를 반드시 포함하세요."""

TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "search",
            "description": "질문과 관련된 문서 조각(l3)을 시맨틱 검색합니다. 항상 첫 번째로 호출하세요.",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "검색할 쿼리 텍스트",
                    },
                    "top_k": {
                        "type": "integer",
                        "description": "반환할 결과 수 (기본: 5)",
                    },
                },
                "required": ["query"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "restore_l2",
            "description": "l2_id로 특정 섹션의 전체 내용을 복원합니다. search 결과가 불충분할 때 사용하세요.",
            "parameters": {
                "type": "object",
                "properties": {
                    "l2_id": {
                        "type": "string",
                        "description": "복원할 섹션의 l2_id UUID (search 결과에서 획득)",
                    },
                },
                "required": ["l2_id"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "restore_l1",
            "description": "l1_id로 문서 전체 내용을 복원합니다. l2 복원으로도 불충분할 때 사용하세요.",
            "parameters": {
                "type": "object",
                "properties": {
                    "l1_id": {
                        "type": "string",
                        "description": "복원할 문서의 l1_id UUID (search 결과에서 획득)",
                    },
                },
                "required": ["l1_id"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "answer",
            "description": "질문에 대한 최종 답변을 제공합니다. 관련 내용을 찾았을 때 호출하세요.",
            "parameters": {
                "type": "object",
                "properties": {
                    "content": {
                        "type": "string",
                        "description": "질문에 대한 답변 텍스트 (한국어, 출처 포함)",
                    },
                    "sources": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "참고한 문서/섹션 목록 (예: ['GeneSo README', 'Gopedia RoadMap > Phase 4.3'])",
                    },
                },
                "required": ["content"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "not_found",
            "description": "모든 탐색 후에도 관련 정보를 찾을 수 없을 때 호출합니다.",
            "parameters": {
                "type": "object",
                "properties": {
                    "reason": {
                        "type": "string",
                        "description": "찾을 수 없는 이유",
                    },
                },
                "required": ["reason"],
            },
        },
    },
]


# ── 도구 실행 ────────────────────────────────────────────────────────────────

def _execute_search(conn: Any, args: dict) -> str:
    from flows.xylem_flow.retriever import retrieve_and_enrich

    query = args.get("query", "")
    top_k = int(args.get("top_k", 5))
    try:
        hits = retrieve_and_enrich(
            query,
            conn,
            final_limit=top_k,
            context_level=1,
            neighbor_window=1000,
        )
    except Exception as e:
        return json.dumps({"error": str(e), "results": []}, ensure_ascii=False)

    if not hits:
        return json.dumps({"results": [], "message": "검색 결과 없음"}, ensure_ascii=False)

    # l2_id 기준으로 중복 제거 (같은 섹션의 여러 청크 → 대표 1개만, 다른 섹션은 각각 포함)
    seen: set[str] = set()
    deduped = []
    for h in hits:
        key = h.get("l2_id") or h.get("l1_id", "")
        if key and key in seen:
            continue
        if key:
            seen.add(key)
        deduped.append(h)

    results = []
    for h in deduped:
        results.append({
            "l1_id": h.get("l1_id", ""),
            "l2_id": h.get("l2_id", ""),
            "l3_id": h.get("matched_l3_id", ""),
            "title": h.get("l1_title", ""),
            "section": h.get("section_heading", ""),
            "l2_summary": (h.get("l2_summary") or "")[:300],
            "score": round(float(h.get("qdrant_score", 0)), 4),
            "snippet": (h.get("matched_content") or "")[:500],
            "context": (h.get("surrounding_context") or "")[:600],
        })

    return json.dumps({"results": results}, ensure_ascii=False)


def _execute_restore_l2(conn: Any, args: dict) -> str:
    from flows.xylem_flow.restorer import restore_code_for_l2

    l2_id = args.get("l2_id", "").strip()
    if not l2_id:
        return json.dumps({"error": "l2_id required"}, ensure_ascii=False)
    try:
        content = restore_code_for_l2(conn, l2_id)
        return json.dumps({"l2_id": l2_id, "content": content[:3000]}, ensure_ascii=False)
    except Exception as e:
        return json.dumps({"error": str(e)}, ensure_ascii=False)


def _execute_restore_l1(conn: Any, args: dict) -> str:
    from flows.xylem_flow.restorer import restore_content_for_l1

    l1_id = args.get("l1_id", "").strip()
    if not l1_id:
        return json.dumps({"error": "l1_id required"}, ensure_ascii=False)
    try:
        restored = restore_content_for_l1(conn, l1_id)
        content = restored.get("content") or ""
        return json.dumps({
            "l1_id": l1_id,
            "content": content[:5000],
        }, ensure_ascii=False)
    except Exception as e:
        return json.dumps({"error": str(e)}, ensure_ascii=False)


def _dispatch_tool(conn: Any, name: str, args: dict) -> str:
    if name == "search":
        return _execute_search(conn, args)
    if name == "restore_l2":
        return _execute_restore_l2(conn, args)
    if name == "restore_l1":
        return _execute_restore_l1(conn, args)
    return json.dumps({"error": f"unknown tool: {name}"}, ensure_ascii=False)


# ── LLM 호출 ────────────────────────────────────────────────────────────────

def _chat(messages: list[dict]) -> dict:
    import urllib.request

    payload = json.dumps({
        "model": OLLAMA_CHAT_MODEL,
        "messages": messages,
        "tools": TOOLS,
        "stream": False,
        "options": {"temperature": 0.1},
    }).encode()

    req = urllib.request.Request(
        f"{OLLAMA_CHAT_URL}/api/chat",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=120) as resp:
        return json.loads(resp.read())


# ── 에이전트 메인 루프 ────────────────────────────────────────────────────────

def run(query: str, conn: Any) -> dict:
    """계층형 검색 에이전트 실행.

    Returns:
        {
            "answer": str,          # 합성 답변 또는 "관련 문서를 찾을 수 없습니다."
            "sources": list[str],   # 참고 출처
            "found": bool,
            "trace": list[str],     # 탐색 경로 (디버그용)
        }
    """
    messages: list[dict] = [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": query},
    ]
    trace: list[str] = []

    for iteration in range(MAX_ITERATIONS):
        try:
            response = _chat(messages)
        except Exception as e:
            log.error("LLM call failed (iter=%d): %s", iteration, e)
            return {
                "answer": f"LLM 호출 중 오류가 발생했습니다: {e}",
                "sources": [],
                "found": False,
                "trace": trace,
            }

        msg = response.get("message", {})
        tool_calls = msg.get("tool_calls") or []

        # tool call 없이 텍스트만 반환한 경우 → 직접 답변으로 처리
        if not tool_calls:
            content = (msg.get("content") or "").strip()
            if content:
                trace.append("direct_text_response")
                return {
                    "answer": content,
                    "sources": [],
                    "found": True,
                    "trace": trace,
                }
            break

        # 메시지 누적
        messages.append({"role": "assistant", "content": msg.get("content") or "", "tool_calls": tool_calls})

        # 각 tool call 처리
        for idx, tc in enumerate(tool_calls):
            fn = tc.get("function", {})
            tool_name = fn.get("name", "")
            tool_args = fn.get("arguments", {})
            tool_id = tc.get("id") or f"call_{iteration}_{idx}"

            trace.append(f"{tool_name}({json.dumps(tool_args, ensure_ascii=False)[:80]})")
            log.info("tool_call iter=%d tool=%s args=%s", iteration, tool_name, tool_args)

            # 종료 도구
            if tool_name == "answer":
                return {
                    "answer": tool_args.get("content", ""),
                    "sources": tool_args.get("sources") or [],
                    "found": True,
                    "trace": trace,
                }
            if tool_name == "not_found":
                reason = tool_args.get("reason", "")
                answer_text = f"관련 문서를 찾을 수 없습니다."
                if reason:
                    answer_text += f" ({reason})"
                return {
                    "answer": answer_text,
                    "sources": [],
                    "found": False,
                    "trace": trace,
                }

            # 검색/복원 도구 실행
            result = _dispatch_tool(conn, tool_name, tool_args)
            messages.append({
                "role": "tool",
                "tool_call_id": tool_id,
                "content": result,
            })

    return {
        "answer": "최대 탐색 횟수를 초과했습니다. 관련 정보를 찾지 못했습니다.",
        "sources": [],
        "found": False,
        "trace": trace,
    }
