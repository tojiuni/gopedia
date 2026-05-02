"""Knowledge tree queries for project-level L1 node exploration.

Provides helpers to fetch and build a nested L1 node tree for a given project,
for use in document viewer UIs and /api/tree endpoints.
"""
from __future__ import annotations
from typing import Any


def fetch_project_l1_nodes(conn: Any, project_id: int) -> list[dict]:
    """Return flat list of L1 nodes for a project, ordered by creation time."""
    rows = conn.execute(
        """
        SELECT id::text, parent_id::text, title, source_type, document_id::text
          FROM knowledge_l1
         WHERE project_id = %s
         ORDER BY created_at
        """,
        (project_id,),
    ).fetchall()
    return [
        {
            "id": r[0],
            "parent_id": r[1],
            "title": r[2],
            "source_type": r[3],
            "document_id": r[4],
        }
        for r in rows
    ]


def build_project_l1_tree(conn: Any, project_id: int) -> list[dict]:
    """Build a nested tree from flat L1 nodes using parent_id relationships.

    Nodes whose parent_id is None or not in the result set become root nodes.
    Each node gets a 'children' list.
    """
    nodes = fetch_project_l1_nodes(conn, project_id)
    by_id: dict[str, dict] = {n["id"]: {**n, "children": []} for n in nodes}
    roots: list[dict] = []
    for node in by_id.values():
        pid = node["parent_id"]
        if pid and pid in by_id:
            by_id[pid]["children"].append(node)
        else:
            roots.append(node)
    return roots


def get_project_tree_for_viewer(conn: Any, project_id: int) -> dict:
    """Return API-ready JSON structure for project tree viewer.

    Response shape:
        { "project_id": int, "tree": [ { "id": ..., "children": [...] }, ... ] }
    """
    return {
        "project_id": project_id,
        "tree": build_project_l1_tree(conn, project_id),
    }
