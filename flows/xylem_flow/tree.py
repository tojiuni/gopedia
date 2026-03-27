"""Project-scoped knowledge_l1 tree for explorer / viewer navigation."""

from __future__ import annotations

from typing import Any, Dict, List, Optional


PROJECT_L1_NODES_SQL = """
SELECT id::text,
       parent_id::text,
       title,
       source_type,
       document_id::text
FROM knowledge_l1
WHERE project_id = %s
ORDER BY title NULLS LAST, id
"""


def fetch_project_l1_nodes(conn: Any, project_id: int) -> List[Dict[str, Any]]:
    """Return flat rows as dicts (id, parent_id, title, source_type, document_id)."""
    with conn.cursor() as cur:
        cur.execute(PROJECT_L1_NODES_SQL, (project_id,))
        rows = cur.fetchall()
    out: List[Dict[str, Any]] = []
    for r in rows:
        pid = r[1]
        out.append(
            {
                "id": r[0],
                "parent_id": pid if pid else None,
                "title": r[2] or "",
                "source_type": (r[3] or "md").lower(),
                "document_id": r[4] or "",
            }
        )
    return out


def build_project_l1_tree(conn: Any, project_id: int) -> List[Dict[str, Any]]:
    """
    Build a nested tree of knowledge_l1 nodes for one projects.id (BIGINT).

    Each node: id, parent_id, title, source_type, document_id, children[].
    Roots are nodes with no parent or parent not in this project slice.
    """
    flat = fetch_project_l1_nodes(conn, project_id)
    nodes: Dict[str, Dict[str, Any]] = {}
    for n in flat:
        item = {
            "id": n["id"],
            "parent_id": n["parent_id"],
            "title": n["title"],
            "source_type": n["source_type"],
            "document_id": n["document_id"],
            "children": [],
        }
        nodes[item["id"]] = item

    roots: List[Dict[str, Any]] = []
    for nid, node in nodes.items():
        pid: Optional[str] = node.get("parent_id")
        if pid and pid in nodes:
            nodes[pid]["children"].append(node)
        else:
            roots.append(node)
    return roots


def get_project_tree_for_viewer(conn: Any, project_id: int) -> Dict[str, Any]:
    """Wrapper returning project_id and tree for API-friendly JSON."""
    return {
        "project_id": project_id,
        "tree": build_project_l1_tree(conn, project_id),
    }
