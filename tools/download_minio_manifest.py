"""Resume a public MinIO bucket into a private staging directory.

Create the manifest with `mc ls --recursive --json alias/bucket > manifest.jsonl`.
Only complete, size-checked objects are exposed at their final paths. For
single-part ETags, downloaded bytes are also checked against the source MD5.
"""

import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
import hashlib
from http.client import IncompleteRead
import json
import os
from pathlib import Path, PurePosixPath
import re
import time
from urllib.error import URLError
from urllib.parse import quote
from urllib.request import ProxyHandler, Request, build_opener


SINGLE_PART_ETAG = re.compile(r"[0-9a-f]{32}", re.IGNORECASE)
PART_SUFFIX = ".pezmax-download-part"


def manifest_entries(path):
    seen = set()
    with path.open(encoding="utf-8") as manifest:
        for line in manifest:
            item = json.loads(line)
            if item.get("type") != "file":
                continue
            key = item["key"]
            parts = PurePosixPath(key).parts
            if not parts or key.startswith("/") or ".." in parts or "\x00" in key:
                raise ValueError("Manifest contains an unsafe object key")
            if key in seen:
                raise ValueError("Manifest contains duplicate object keys")
            seen.add(key)
            yield key, int(item["size"]), item.get("etag", "").strip('"'), item.get("lastModified")


def verified(path, size, etag):
    if not path.is_file() or path.stat().st_size != size:
        return False
    if not SINGLE_PART_ETAG.fullmatch(etag):
        return True
    with path.open("rb") as source:
        return hashlib.file_digest(source, "md5").hexdigest().lower() == etag.lower()


def fetch(entry, destination, source_base, attempts):
    key, size, etag, modified = entry
    target = destination.joinpath(*PurePosixPath(key).parts)
    if verified(target, size, etag):
        return True
    target.parent.mkdir(parents=True, exist_ok=True)
    partial = target.with_name(target.name + PART_SUFFIX)
    if size == 0:
        partial.write_bytes(b"")
    url = source_base.rstrip("/") + "/" + quote(key, safe="/")
    for attempt in range(attempts):
        offset = partial.stat().st_size if partial.exists() else 0
        if offset > size:
            partial.unlink()
            offset = 0
        if offset < size:
            headers = {"Range": f"bytes={offset}-"} if offset else {}
            request = Request(url, headers=headers)
            try:
                with build_opener(ProxyHandler({})).open(request, timeout=90) as response:
                    if offset and (response.status != 206 or not response.headers.get("Content-Range", "").startswith(f"bytes {offset}-")):
                        partial.unlink(missing_ok=True)
                        continue
                    with partial.open("ab" if offset else "wb") as output:
                        while chunk := response.read(256 * 1024):
                            output.write(chunk)
            except (OSError, URLError, IncompleteRead):
                pass
        if verified(partial, size, etag):
            os.replace(partial, target)
            if modified:
                timestamp = datetime.fromisoformat(modified.replace("Z", "+00:00")).timestamp()
                os.utime(target, (timestamp, timestamp))
            return True
        if partial.exists() and partial.stat().st_size == size:
            partial.unlink()
        time.sleep(min(2 ** attempt, 15))
    return False


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("destination", type=Path)
    parser.add_argument("source_base", help="Public bucket URL, e.g. http://host:9000/bucket")
    parser.add_argument("--workers", type=int, default=8)
    parser.add_argument("--attempts", type=int, default=20)
    args = parser.parse_args()
    if args.workers < 1 or args.attempts < 1:
        parser.error("workers and attempts must be positive")
    entries = list(manifest_entries(args.manifest))
    pending = [entry for entry in entries if not verified(args.destination.joinpath(*PurePosixPath(entry[0]).parts), entry[1], entry[2])]
    print(f"manifest={len(entries)} already_verified={len(entries) - len(pending)} pending={len(pending)}", flush=True)
    failed = 0
    with ThreadPoolExecutor(max_workers=args.workers) as pool:
        futures = [pool.submit(fetch, entry, args.destination, args.source_base, args.attempts) for entry in pending]
        for completed, future in enumerate(as_completed(futures), 1):
            try:
                if not future.result():
                    failed += 1
            except Exception:
                failed += 1
            if completed % 100 == 0 or completed == len(pending):
                print(f"processed={completed}/{len(pending)} failed={failed}", flush=True)
    print(f"verified={len(entries) - failed}/{len(entries)}", flush=True)
    raise SystemExit(1 if failed else 0)


if __name__ == "__main__":
    main()
