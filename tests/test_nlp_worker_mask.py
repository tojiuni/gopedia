"""Mask + sentence split pipeline (aligns with python/nlp_worker and Phloem L3)."""

from __future__ import annotations

import os
import sys
from pathlib import Path

import pytest

_REPO = Path(__file__).resolve().parents[1]
_NLP = _REPO / "python" / "nlp_worker"
if str(_NLP) not in sys.path:
    sys.path.insert(0, str(_NLP))

from mask import (  # noqa: E402
    mask_for_sentence_split,
    protected_originals,
    unmask_sentences,
)
from split_text import split_sentences_language_aware  # noqa: E402


def _pipeline(text: str) -> list[str]:
    masked, reps = mask_for_sentence_split(text)
    masked_sents = split_sentences_language_aware(masked)
    return unmask_sentences(masked_sents, reps)


def _assert_each_protected_in_single_sentence(sents: list[str], originals: list[str]) -> None:
    for orig in originals:
        if not orig:
            continue
        n = sum(1 for s in sents if orig in s)
        assert n == 1, f"expected {orig!r} in exactly one sentence, got {n} in {sents!r}"


def test_golden_strings_from_plan() -> None:
    cases = [
        "Verify(4.1.1~4.1.5) 일정은 [RoadMap/SKILL.md](RoadMap/SKILL.md) 참조.",
        "(전체 target_day는 [RoadMap/SKILL.md](RoadMap/SKILL.md) 참조)",
        "* **geneso/references/geneso-design-standard.md**: 디자인 (v1.3).",
        "4.1.3~4.1.5(Ticket/Meeting/ERP) 및 Expand·Connect 일정은 [RoadMap/SKILL.md](RoadMap/SKILL.md) 참조.",
        "1. **Verify (발아)**: 최소 단위 Root(소스)가 Stem(phloem, ingestion)을 타고 Rhizome에 안착하는지 검증.",
    ]
    for t in cases:
        _, reps = mask_for_sentence_split(t)
        _assert_each_protected_in_single_sentence(_pipeline(t), protected_originals(reps))


def test_fixture_sample_paragraphs() -> None:
    path = _REPO / "tests" / "fixtures" / "nlp_skill_sample.md"
    text = path.read_text(encoding="utf-8")
    for block in text.split("\n\n"):
        block = block.strip()
        if not block or block.startswith("#"):
            continue
        masked, reps = mask_for_sentence_split(block)
        if not reps:
            continue
        sents = _pipeline(block)
        _assert_each_protected_in_single_sentence(sents, protected_originals(reps))


def test_external_skill_md_if_present() -> None:
    path = os.environ.get(
        "GOPEDIA_NLP_TEST_SKILL_PATH",
        "/morphogen/neunexus/project_skills/wiki/universitas/gopedia/SKILL.md",
    )
    p = Path(path)
    if not p.is_file():
        pytest.skip(f"SKILL.md not at {path}")
    text = p.read_text(encoding="utf-8")
    # Spot-check lines that contain links / versions (body only, skip frontmatter)
    if "---" in text:
        parts = text.split("---", 2)
        if len(parts) >= 3:
            text = parts[2]
    needles = [
        "[RoadMap/SKILL.md](RoadMap/SKILL.md)",
        "4.1.3~4.1.5",
        "v1.3",
        "geneso-design-standard.md",
    ]
    for needle in needles:
        if needle not in text:
            continue
        idx = text.index(needle)
        lo = max(0, idx - 200)
        hi = min(len(text), idx + 200)
        chunk = text[lo:hi]
        masked, reps = mask_for_sentence_split(chunk)
        if not any(needle in o for _, o in reps):
            continue
        sents = _pipeline(chunk)
        for _, o in reps:
            if needle in o:
                _assert_each_protected_in_single_sentence(sents, [o])
