"""
Tree-sitter based source code parser.

Outputs a dict with:
  toc:   list of top-level declarations (L2 candidates)
  lines: list of per-line metadata (L3 candidates)

Each line entry:
  line_num     : int  (1-based)
  content      : str  (exact source line, empty string for blank lines)
  node_type    : str  (tree-sitter node type or "empty_line")
  is_anchor    : bool (True = gets Qdrant embedding)
  is_block_start: bool (True = multiline compound expression head)
  parent_idx   : int  (-1 = no parent in this chunk; else index into lines list)
"""

from __future__ import annotations
import re
from typing import Any

try:
    import tree_sitter_python as tspython
    import tree_sitter_go as tsgo
    from tree_sitter import Language, Parser
    _TS_AVAILABLE = True
except ImportError:
    _TS_AVAILABLE = False


# Node types that should become L3 anchors (get Qdrant embedding)
ANCHOR_NODE_TYPES = {
    # Python
    "function_definition",
    "async_function_definition",
    "class_definition",
    "if_statement",
    "elif_clause",
    "else_clause",
    "for_statement",
    "while_statement",
    "with_statement",
    "try_statement",
    "except_clause",
    "match_statement",
    "decorated_definition",
    # Go
    "function_declaration",
    "method_declaration",
    "type_declaration",
    "const_declaration",
    "var_declaration",
    "if_statement",
    "for_statement",
    "switch_statement",
    "select_statement",
    "go_statement",
    "defer_statement",
}

# Node types that, when spanning multiple lines, mark a block start
BLOCK_START_TYPES = {
    "return_statement",
    "call_expression",
    "expression_statement",
    "assignment",
    "augmented_assignment",
    "short_var_declaration",  # Go :=
    "binary_expression",
    "argument_list",
}


def _get_parser(lang: str) -> "Parser | None":
    if not _TS_AVAILABLE:
        return None
    try:
        if lang == "python":
            language = Language(tspython.language())
        elif lang == "go":
            language = Language(tsgo.language())
        else:
            return None
        parser = Parser(language)
        return parser
    except Exception:
        return None


def _node_covering_line(root, line_0: int):
    """Find the deepest tree-sitter node that starts on line_0 (0-based)."""
    best = None
    def walk(node):
        nonlocal best
        if node.start_point[0] == line_0:
            if best is None or node.start_point[0] >= best.start_point[0]:
                best = node
        for child in node.children:
            if child.start_point[0] <= line_0 <= child.end_point[0]:
                walk(child)
    walk(root)
    return best


def _collect_top_level_nodes(root):
    """Return direct children of module/source_file that are declarations."""
    results = []
    for child in root.children:
        if child.type in (
            "function_definition", "async_function_definition",
            "class_definition", "decorated_definition",
            "function_declaration", "method_declaration",
            "type_declaration", "const_declaration", "var_declaration",
            "import_declaration", "package_clause",
        ):
            results.append(child)
    return results


def _find_anchor_ancestor(node, anchor_types):
    """Walk up from node to find the nearest ancestor that is an anchor."""
    cur = node.parent
    while cur is not None:
        if cur.type in anchor_types:
            return cur
        cur = cur.parent
    return None


def parse_source(source: str, lang: str) -> dict[str, Any]:
    """
    Parse source code and return toc + lines structure.
    Falls back to line-by-line regex if tree-sitter is unavailable.
    """
    parser = _get_parser(lang)
    if parser is not None:
        return _parse_with_treesitter(source, lang, parser)
    return _parse_fallback(source, lang)


def _parse_with_treesitter(source: str, lang: str, parser) -> dict[str, Any]:
    source_bytes = source.encode("utf-8")
    tree = parser.parse(source_bytes)
    root = tree.root_node

    raw_lines = source.splitlines()

    # Build TOC from top-level declarations
    toc = []
    top_nodes = _collect_top_level_nodes(root)
    for node in top_nodes:
        name = _extract_name(node, source_bytes)
        toc.append({
            "text": name,
            "level": 2,
            "node_type": node.type,
            "start_line": node.start_point[0] + 1,
            "end_line": node.end_point[0] + 1,
        })

    # Build line-level metadata
    # Strategy:
    #   For each line, find the shallowest interesting node that starts on that line.
    #   Determine is_anchor, is_block_start.
    #   Determine parent_idx by finding the nearest anchor ancestor's start line.

    # Map: start_line_0 -> anchor node (for parent_idx resolution)
    anchor_start_lines: dict[int, int] = {}  # line_0 -> index in lines list

    lines_out = []
    for i, raw in enumerate(raw_lines):
        line_num = i + 1
        content = raw  # preserve original (including indentation)
        stripped = raw.strip()

        if stripped == "":
            lines_out.append({
                "line_num": line_num,
                "content": content,
                "node_type": "empty_line",
                "is_anchor": False,
                "is_block_start": False,
                "parent_idx": -1,  # filled in second pass
            })
            continue

        node = _node_covering_line(root, i)
        if node is None:
            lines_out.append({
                "line_num": line_num,
                "content": content,
                "node_type": "unknown",
                "is_anchor": False,
                "is_block_start": False,
                "parent_idx": -1,
            })
            continue

        # Walk up to find shallowest anchor-type node starting on this line
        anchor_node = None
        cur = node
        while cur is not None:
            if cur.start_point[0] == i and cur.type in ANCHOR_NODE_TYPES:
                anchor_node = cur
            cur = cur.parent

        is_anchor = anchor_node is not None
        is_block_start = False
        node_type = node.type

        if anchor_node is not None:
            node_type = anchor_node.type
            # Block start: anchor spans multiple lines
            if anchor_node.end_point[0] > anchor_node.start_point[0]:
                is_block_start = True
        else:
            # Check for multi-line non-anchor block start
            cur = node
            while cur is not None:
                if cur.start_point[0] == i and cur.type in BLOCK_START_TYPES:
                    if cur.end_point[0] > cur.start_point[0]:
                        is_block_start = True
                        node_type = cur.type
                    break
                cur = cur.parent

        if is_anchor:
            anchor_start_lines[i] = len(lines_out)

        lines_out.append({
            "line_num": line_num,
            "content": content,
            "node_type": node_type,
            "is_anchor": is_anchor,
            "is_block_start": is_block_start,
            "parent_idx": -1,  # filled in second pass
        })

    # Second pass: assign parent_idx
    # For each non-anchor line, find the nearest anchor ancestor's line index in lines_out
    for i, raw in enumerate(raw_lines):
        if lines_out[i]["is_anchor"]:
            continue

        stripped = raw.strip()
        if stripped == "":
            # Blank lines: parent = most recent anchor above
            for j in range(i - 1, -1, -1):
                if lines_out[j]["is_anchor"]:
                    lines_out[i]["parent_idx"] = j
                    break
            continue

        node = _node_covering_line(root, i)
        if node is None:
            continue

        # Find anchor ancestor node
        anc = _find_anchor_ancestor(node, ANCHOR_NODE_TYPES)
        while anc is not None:
            anc_line_0 = anc.start_point[0]
            if anc_line_0 in anchor_start_lines:
                lines_out[i]["parent_idx"] = anchor_start_lines[anc_line_0]
                break
            anc = _find_anchor_ancestor(anc, ANCHOR_NODE_TYPES)

        # For is_block_start lines that are children of another anchor line
        if lines_out[i]["is_block_start"] and lines_out[i]["parent_idx"] == -1:
            for j in range(i - 1, -1, -1):
                if lines_out[j]["is_anchor"]:
                    lines_out[i]["parent_idx"] = j
                    break

    return {"toc": toc, "lines": lines_out}


def _extract_name(node, source_bytes: bytes) -> str:
    """Extract identifier name from a declaration node."""
    for child in node.children:
        if child.type == "identifier":
            return source_bytes[child.start_byte:child.end_byte].decode("utf-8")
        if child.type == "name":
            return source_bytes[child.start_byte:child.end_byte].decode("utf-8")
    # fallback: first line of node
    first_line = source_bytes[node.start_byte:node.end_byte].decode("utf-8", errors="replace").split("\n")[0]
    return first_line[:80]


def _parse_fallback(source: str, lang: str) -> dict[str, Any]:
    """
    Regex-based fallback when tree-sitter is not installed.
    Detects function/class definitions as anchors.
    """
    raw_lines = source.splitlines()
    toc = []
    lines_out = []

    py_def_re = re.compile(r'^(\s*)(async\s+)?def\s+(\w+)\s*\(')
    py_class_re = re.compile(r'^(\s*)class\s+(\w+)')
    go_func_re = re.compile(r'^func\s+')

    last_anchor_idx = -1

    for i, raw in enumerate(raw_lines):
        line_num = i + 1
        stripped = raw.strip()

        if stripped == "":
            lines_out.append({
                "line_num": line_num, "content": raw,
                "node_type": "empty_line", "is_anchor": False,
                "is_block_start": False,
                "parent_idx": last_anchor_idx if last_anchor_idx >= 0 else -1,
            })
            continue

        is_anchor = False
        node_type = "expression_statement"
        m = py_def_re.match(raw)
        if m:
            indent = len(m.group(1))
            if indent == 0:
                is_anchor = True
                node_type = "function_definition"
                toc.append({"text": m.group(3), "level": 2, "node_type": node_type,
                             "start_line": line_num, "end_line": line_num})
        if not is_anchor:
            m2 = py_class_re.match(raw)
            if m2 and len(m2.group(1)) == 0:
                is_anchor = True
                node_type = "class_definition"
                toc.append({"text": m2.group(2), "level": 2, "node_type": node_type,
                             "start_line": line_num, "end_line": line_num})
        if not is_anchor and go_func_re.match(raw):
            is_anchor = True
            node_type = "function_declaration"

        cur_idx = len(lines_out)
        if is_anchor:
            last_anchor_idx = cur_idx
            parent_idx = -1
        else:
            parent_idx = last_anchor_idx if last_anchor_idx >= 0 else -1

        lines_out.append({
            "line_num": line_num, "content": raw,
            "node_type": node_type, "is_anchor": is_anchor,
            "is_block_start": False, "parent_idx": parent_idx,
        })

    return {"toc": toc, "lines": lines_out}
