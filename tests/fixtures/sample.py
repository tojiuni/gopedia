import os


def _pg_connect():
    import psycopg

    return psycopg.connect(
        f"host={os.environ.get('POSTGRES_HOST', '')} "
        f"port={os.environ.get('POSTGRES_PORT', '5432')} "
        f"dbname={os.environ.get('POSTGRES_DB', 'gopedia')} "
        f"user={os.environ.get('POSTGRES_USER', '')} "
        f"password={os.environ.get('POSTGRES_PASSWORD', '')}"
    )


class DataStore:
    """Simple data store wrapper."""

    def __init__(self, conn):
        self.conn = conn

    def fetch(self, query: str) -> list:
        with self.conn.cursor() as cur:
            cur.execute(query)
            return cur.fetchall()
