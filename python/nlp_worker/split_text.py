"""Language-aware sentence splitting on already-masked text."""

from __future__ import annotations

import os


def hangul_ratio(text: str) -> float:
    if not text:
        return 0.0
    n = 0
    for c in text:
        if "\uac00" <= c <= "\ud7a3":
            n += 1
    return n / max(len(text), 1)


def split_sentences_language_aware(masked: str) -> list[str]:
    """
    Split masked text into sentence-like segments.
    Uses kss for Hangul-heavy text, pysbd otherwise.
    """
    text = masked.replace("\r\n", "\n").replace("\r", "\n")
    if not text.strip():
        return []
    thresh = float(os.environ.get("GOPEDIA_NLP_LANG_HANGUL_RATIO", "0.12"))
    if hangul_ratio(text) >= thresh:
        try:
            import kss

            parts = kss.split_sentences(text)
            return [p.strip() for p in parts if p and str(p).strip()]
        except ImportError:
            parts = _split_regex_cjk_fallback(text)
            return [p.strip() for p in parts if p and str(p).strip()]
    from pysbd import Segmenter

    seg = Segmenter(language="en", clean=False)
    parts = seg.segment(text)
    return [p.strip() for p in parts if p and str(p).strip()]


def _split_regex_cjk_fallback(text: str) -> list[str]:
    """If kss is not installed, split on CJK/Latin sentence punctuation (masked text has no false dots)."""
    import re

    parts = re.split(r"(?<=[.!?。！？…])\s+|\n{2,}", text)
    return [p for p in parts if p.strip()]
