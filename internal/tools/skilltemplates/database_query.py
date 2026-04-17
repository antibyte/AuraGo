import sys
import json
import os
import sqlite3

def {{.FunctionName}}(query, db_type="sqlite", connection="", params=None, limit=100):
    """{{.Description}}"""
    if not query:
        return {"status": "error", "message": "SQL query is required"}
    if not connection:
        return {"status": "error", "message": "Database connection (file path or connection string) is required"}

    limit = int(limit)
    query_upper = query.strip().upper()

    try:
        if db_type == "sqlite":
            if not os.path.isabs(connection):
                connection = os.path.abspath(connection)
            if not os.path.exists(connection):
                return {"status": "error", "message": f"Database file not found: {connection}"}

            conn = sqlite3.connect(connection)
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            if params and isinstance(params, list):
                cursor.execute(query, params)
            else:
                cursor.execute(query)

            if query_upper.startswith("SELECT"):
                rows = cursor.fetchmany(limit)
                columns = [desc[0] for desc in cursor.description] if cursor.description else []
                results = [dict(zip(columns, row)) for row in rows]
                conn.close()
                return {"status": "success", "result": {"rows": results, "count": len(results), "columns": columns}}
            else:
                affected = cursor.rowcount
                conn.commit()
                conn.close()
                return {"status": "success", "result": {"affected_rows": affected}}

        elif db_type == "postgresql":
            try:
                import psycopg2
            except ImportError:
                return {"status": "error", "message": "psycopg2 not installed. Add 'psycopg2-binary' to dependencies."}
            conn = psycopg2.connect(connection)
            cursor = conn.cursor()
            if params and isinstance(params, list):
                cursor.execute(query, params)
            else:
                cursor.execute(query)
            if query_upper.startswith("SELECT"):
                rows = cursor.fetchmany(limit)
                columns = [desc[0] for desc in cursor.description] if cursor.description else []
                results = [dict(zip(columns, row)) for row in rows]
                conn.close()
                return {"status": "success", "result": {"rows": results, "count": len(results), "columns": columns}}
            else:
                affected = cursor.rowcount
                conn.commit()
                conn.close()
                return {"status": "success", "result": {"affected_rows": affected}}

        elif db_type == "mysql":
            try:
                import pymysql
            except ImportError:
                return {"status": "error", "message": "pymysql not installed. Add 'pymysql' to dependencies."}
            conn = pymysql.connect(connection if "://" in connection else connection)
            cursor = conn.cursor()
            if params and isinstance(params, list):
                cursor.execute(query, params)
            else:
                cursor.execute(query)
            if query_upper.startswith("SELECT"):
                rows = cursor.fetchmany(limit)
                columns = [desc[0] for desc in cursor.description] if cursor.description else []
                results = [dict(zip(columns, row)) for row in rows]
                conn.close()
                return {"status": "success", "result": {"rows": results, "count": len(results), "columns": columns}}
            else:
                affected = cursor.rowcount
                conn.commit()
                conn.close()
                return {"status": "success", "result": {"affected_rows": affected}}

        else:
            return {"status": "error", "message": f"Unsupported database type: {db_type}. Use: sqlite, postgresql, mysql"}

    except Exception as e:
        return {"status": "error", "message": str(e)}
