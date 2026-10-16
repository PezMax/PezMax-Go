"""Convert the ten legacy PezMax MySQL tables to a transactional psql script.

Requires sqlglot: python -m pip install sqlglot
The output contains private user data. Keep it outside the repository and delete
it after the import. Run with psql -X -v ON_ERROR_STOP=1 --single-transaction.
"""

import argparse
from collections import defaultdict
import os
from pathlib import Path
import re
from urllib.parse import urlsplit

from sqlglot import exp, parse_one


TABLES = (
    "ptmj_user",
    "ptmj_security",
    "ptmj_file",
    "ptmj_file_download",
    "ptmj_file_favorite",
    "ptmj_report",
    "ptmj_bookmark",
    "ptmj_bookmark_favorite",
    "ptmj_bookmark_report",
    "ptmj_notification",
)
IDENTITY_COLUMNS = {
    "ptmj_user": "user_id",
    "ptmj_security": "id",
    "ptmj_file": "file_id",
    "ptmj_file_download": "download_id",
    "ptmj_report": "report_id",
    "ptmj_bookmark": "id",
    "ptmj_bookmark_report": "report_id",
    "ptmj_notification": "notify_id",
}
TABLE_START = re.compile(r"^CREATE TABLE `(?P<table>ptmj_[a-z_]+)`")
COLUMN = re.compile(r"^\s*`(?P<column>[a-z_]+)`\s")
INSERT_START = re.compile(r"^INSERT INTO `(?P<table>ptmj_[a-z_]+)`")


def scalar(expression):
    if isinstance(expression, exp.Null):
        return None
    if isinstance(expression, exp.Literal):
        return expression.this
    if isinstance(expression, exp.Neg) and isinstance(expression.this, exp.Literal):
        return "-" + expression.this.this
    raise ValueError(f"Unsupported MySQL value expression: {type(expression).__name__}")


def copy_field(value):
    if value is None:
        return r"\N"
    if "\x00" in value:
        raise ValueError("PostgreSQL cannot store NUL bytes")
    return value.replace("\\", "\\\\").replace("\t", r"\t").replace("\n", r"\n").replace("\r", r"\r")


def read_dump(path):
    columns = {}
    rows = defaultdict(list)
    current_table = None
    with path.open(encoding="utf-8-sig") as source:
        for line_number, line in enumerate(source, 1):
            match = TABLE_START.match(line)
            if match:
                current_table = match.group("table")
                if current_table not in TABLES or current_table in columns:
                    raise ValueError(f"Unexpected or duplicate PezMax table at line {line_number}: {current_table}")
                columns[current_table] = []
                continue
            if current_table is not None:
                if line.startswith(") ENGINE"):
                    current_table = None
                    continue
                match = COLUMN.match(line)
                if match:
                    columns[current_table].append(match.group("column"))
                continue
            match = INSERT_START.match(line)
            if not match:
                continue
            table = match.group("table")
            if table not in TABLES or table not in columns:
                raise ValueError(f"Unexpected PezMax INSERT at line {line_number}: {table}")
            statement = parse_one(line, read="mysql")
            if not isinstance(statement, exp.Insert) or not isinstance(statement.expression, exp.Values):
                raise ValueError(f"Unsupported INSERT syntax at line {line_number}")
            if statement.this.name != table:
                raise ValueError(f"INSERT table mismatch at line {line_number}")
            for tuple_expression in statement.expression.expressions:
                if not isinstance(tuple_expression, exp.Tuple):
                    raise ValueError(f"Unsupported VALUES row at line {line_number}")
                values = [scalar(value) for value in tuple_expression.expressions]
                if len(values) != len(columns[table]):
                    raise ValueError(f"Column count mismatch at line {line_number}: {table}")
                rows[table].append(values)
    if set(columns) != set(TABLES):
        raise ValueError(f"Expected ten PezMax tables, found: {sorted(columns)}")
    return columns, rows


def write_sql(path, columns, rows):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as output:
            output.write("SET client_encoding = 'UTF8';\nSET DateStyle = 'ISO, YMD';\n")
            output.write("DO $check$ BEGIN\n")
            for table in TABLES:
                output.write(
                    f"  IF EXISTS (SELECT 1 FROM public.{table}) THEN "
                    f"RAISE EXCEPTION '{table} is not empty'; END IF;\n"
                )
            output.write("END $check$;\n")
            for table in TABLES:
                quoted_columns = ", ".join(f'"{column}"' for column in columns[table])
                output.write(f"COPY public.{table} ({quoted_columns}) FROM STDIN;\n")
                for row in rows[table]:
                    output.write("\t".join(copy_field(value) for value in row) + "\n")
                output.write("\\.\n")
            output.write("DO $check$ BEGIN\n")
            for table in TABLES:
                output.write(
                    f"  IF (SELECT COUNT(*) FROM public.{table}) <> {len(rows[table])} THEN "
                    f"RAISE EXCEPTION '{table} row count mismatch'; END IF;\n"
                )
            output.write("END $check$;\n")
            for table, column in IDENTITY_COLUMNS.items():
                output.write(
                    f"SELECT setval(pg_get_serial_sequence('public.{table}', '{column}'), "
                    f"(SELECT MAX({column}) FROM public.{table}), true);\n"
                )
    except BaseException:
        path.unlink(missing_ok=True)
        raise


def audit_source(columns, rows):
    def values(table, column):
        index = columns[table].index(column)
        return [row[index] for row in rows[table]]

    user_ids = set(values("ptmj_user", "user_id"))
    file_ids = set(values("ptmj_file", "file_id"))
    bookmark_ids = set(values("ptmj_bookmark", "id"))
    for table, column in (
        ("ptmj_security", "user_id"),
        ("ptmj_file", "user_id"),
        ("ptmj_file_download", "user_id"),
        ("ptmj_file_favorite", "user_id"),
        ("ptmj_report", "user_id"),
        ("ptmj_bookmark", "user_id"),
        ("ptmj_bookmark_favorite", "user_id"),
        ("ptmj_bookmark_report", "user_id"),
    ):
        missing = sum(value not in user_ids for value in values(table, column))
        print(f"{table}.{column}: {missing} missing user references")
    for table, column, valid_ids in (
        ("ptmj_file_download", "file_id", file_ids),
        ("ptmj_file_favorite", "file_id", file_ids),
        ("ptmj_report", "file_id", file_ids),
        ("ptmj_bookmark_favorite", "bookmark_id", bookmark_ids),
        ("ptmj_bookmark_report", "bookmark_id", bookmark_ids),
    ):
        missing = sum(value not in valid_ids for value in values(table, column))
        print(f"{table}.{column}: {missing} missing parent references")
    for table, column in (
        ("ptmj_file", "file_url"),
        ("ptmj_bookmark", "cover_image"),
        ("ptmj_user", "avatar"),
    ):
        hosts = defaultdict(int)
        for value in values(table, column):
            if value:
                hosts[urlsplit(value).netloc or "<relative>"] += 1
        print(f"{table}.{column} URL hosts: {dict(sorted(hosts.items()))}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path, help="Navicat MySQL dump")
    parser.add_argument("output", type=Path, help="private psql import script outside the repository")
    args = parser.parse_args()
    columns, rows = read_dump(args.source)
    write_sql(args.output, columns, rows)
    for table in TABLES:
        print(f"{table}: {len(rows[table])} rows, {len(columns[table])} columns")
    audit_source(columns, rows)


if __name__ == "__main__":
    main()
