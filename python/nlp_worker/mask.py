"""
Mask spans that contain '.' but must not trigger sentence boundaries (links, versions, paths).

Placeholders use U+2060 (WORD JOINER) + U+200C (ZWNJ) so pysbd/kss rarely split inside them.
Keep rules aligned with internal/phloem/sink/splitmask.go.
"""

from __future__ import annotations

import re
from typing import List, Tuple

Replacement = Tuple[str, str]  # (placeholder, original)

# Markdown image or link: [text](url)
_RE_MD_LINK = re.compile(r"!?\[[^\]]*\]\([^)]*\)")
# HTTP(S) URLs (stop before space or closing angle bracket)
_RE_URL = re.compile(r"https?://[^\s\]>]+")
# Semantic versions: v1.3, v2.0.1
_RE_SEMVER = re.compile(r"\bv\d+(?:\.\d+)+")
# Section-style numbers: 4.1.3, 4.1.3~4.1.5 (boundaries validated in _section_ok)
_RE_SECTION_NUM = re.compile(r"\d+\.\d+\.\d+(?:~\d+\.\d+\.\d+)?")
# Common doc/code path endings (path/to/file.md)
_RE_FILE_EXT = re.compile(
    r"(?<![\w/])(?:[\w.-]+/)*[\w.-]+\.(?:md|MD|go|py|txt|yml|yaml|json|proto)\b"
)
# Ordered list markers at the start of a line (e.g., "1.", " 2.")
_RE_LIST_MARKER = re.compile(r"(?m)^\s{0,3}\d+\.")

_PATTERN_ORDER: List[Tuple[str, re.Pattern[str]]] = [
    ("mdlink", _RE_MD_LINK),
    ("url", _RE_URL),
    ("semver", _RE_SEMVER),
    ("section", _RE_SECTION_NUM),
    ("file", _RE_FILE_EXT),
    ("list_marker", _RE_LIST_MARKER),
]


def _section_ok(text: str, start: int, end: int) -> bool:
    if start > 0:
        prev = text[start - 1]
        if prev.isalnum() or prev in "._":
            return False
    if end < len(text):
        nxt = text[end]
        if nxt.isdigit() or nxt == ".":
            return False
    return True


def _file_ok(text: str, start: int, end: int) -> bool:
    if start > 0 and text[start - 1] in "/":
        return True
    if start > 0 and (text[start - 1].isalnum() or text[start - 1] in "._-"):
        return False
    return True


def _list_marker_ok(text: str, start: int, end: int) -> bool:
    if end < len(text):
        nxt = text[end]
        if not nxt.isspace():
            return False
    return True


def _placeholder(i: int) -> str:
    # No '.' or sentence-ending punctuation.
    return f"__GOPEDIA_{i}__"


def _collect_spans(text: str) -> List[Tuple[int, int, str]]:
    raw: List[Tuple[int, int, str]] = []
    seen: set[Tuple[int, int]] = set()
    for name, rx in _PATTERN_ORDER:
        for m in rx.finditer(text):
            s, e = m.start(), m.end()
            if name == "section" and not _section_ok(text, s, e):
                continue
            if name == "file" and not _file_ok(text, s, e):
                continue
            if name == "list_marker" and not _list_marker_ok(text, s, e):
                continue
            key = (s, e)
            if key in seen:
                continue
            seen.add(key)
            raw.append((s, e, m.group(0)))
    # Longest span first, then greedy non-overlapping pack.
    raw.sort(key=lambda x: -(x[1] - x[0]))
    chosen: List[Tuple[int, int, str]] = []
    for s, e, t in raw:
        if any(not (e <= cs or s >= ce) for cs, ce, _ in chosen):
            continue
        chosen.append((s, e, t))
    chosen.sort(key=lambda x: x[0])
    return chosen


def mask_for_sentence_split(text: str) -> Tuple[str, List[Replacement]]:
    """Return masked text and ordered (placeholder, original) pairs."""
    spans = _collect_spans(text)
    if not spans:
        return text, []
    replacements: List[Replacement] = []
    parts: List[str] = []
    last = 0
    for i, (s, e, orig) in enumerate(spans):
        parts.append(text[last:s])
        ph = _placeholder(i)
        replacements.append((ph, orig))
        parts.append(ph)
        last = e
    parts.append(text[last:])
    return "".join(parts), replacements


def unmask_text(s: str, replacements: List[Replacement]) -> str:
    for ph, orig in replacements:
        s = s.replace(ph, orig)
    return s


def unmask_sentences(sents: List[str], replacements: List[Replacement]) -> List[str]:
    return [unmask_text(x, replacements) for x in sents]


def protected_originals(replacements: List[Replacement]) -> List[str]:
    return [orig for _, orig in replacements if orig]
