import re
from urllib.parse import urlsplit

import httpx

from .contracts import CatalogConfig
from .errors import AgentError
from .http import read_json
from .tools import search_arguments

LABELS = {"file": "资料", "ebook": "电子书", "bookmark": "书签"}
# Only fields consumed by the existing reader/details are sent back to Electron.
# The model receives a separate metadata projection, never these preview nodes.
PREVIEW_FIELDS = {
    "file": "fileId userId fileName fileUrl fileSize fileFormat fileYear fileType fileSchool fileSubject fileStatus createBy createTime remark",
    "ebook": "ebookId userId ebookName ebookUrl ebookSize ebookFormat ebookSubject ebookType ebookStatus author publisher coverUrl createBy createTime remark",
    "bookmark": "id userId url title description coverImage subject resourceType collection status createBy createTime remark",
}


def text(value, size=160):
    return str(value)[:size] if value is not None else ""


def number(value):
    try:
        return int(value) if not isinstance(value, bool) else 0
    except (ValueError, TypeError, OverflowError):
        return 0


def http_url(value):
    try:
        url = urlsplit(value)
        return url.scheme in ("https", "http") and bool(url.hostname) and not url.username and not url.password
    except (ValueError, TypeError):
        return False


def resource(kind, row):
    if not isinstance(row, dict):
        return None
    raw_id = row.get({"file": "fileId", "ebook": "ebookId", "bookmark": "id"}[kind])
    status = row.get({"file": "fileStatus", "ebook": "ebookStatus", "bookmark": "status"}[kind])
    if not re.fullmatch(r"[1-9]\d{0,18}", str(raw_id)) or number(status) != 1:
        return None
    # Preserve 64-bit Go IDs through JavaScript without precision loss.
    raw_id = str(raw_id)
    title = text(row.get({"file": "fileName", "ebook": "ebookName", "bookmark": "title"}[kind]))
    if not title:
        return None
    if kind == "bookmark" and not http_url(row.get("url")):
        return None
    if kind == "file":
        url = row.get("fileUrl")
        if not isinstance(url, str) or not (http_url(url) or re.match(r"^/(?!/)", url)):
            return None
    item = {"id": f"{kind}-{raw_id}", "kind": kind, "title": title,
            "subject": text(row.get("fileSubject") or row.get("ebookSubject") or row.get("subject"), 80)}
    if kind == "file":
        item.update(school=text(row.get("fileSchool"), 80), year=number(row.get("fileYear")) or None,
                    fileType=number(row.get("fileType")), format=text(row.get("fileFormat"), 12))
    elif kind == "ebook":
        item.update(author=text(row.get("author"), 80), publisher=text(row.get("publisher"), 80),
                    format=text(row.get("ebookFormat"), 12))
    else:
        item.update(description=text(row.get("description"), 200), collection=text(row.get("collection"), 80))
    preview = {key: text(row[key], 8192) if isinstance(row[key], str) else row[key]
               for key in PREVIEW_FIELDS[kind].split() if key in row and isinstance(row[key], (str, int, float, bool))}
    preview[{"file": "fileId", "ebook": "ebookId", "bookmark": "id"}[kind]] = raw_id
    node = {"type": kind, "id": item["id"], "rawId": raw_id, "label": title}
    if kind == "file":
        node.update(url=preview["fileUrl"], fileExt=preview.get("fileFormat", ""), fileInfo=preview)
    elif kind == "ebook":
        node.update(fileExt=preview.get("ebookFormat", ""), originalData=preview)
    else:
        node = {**preview, **node}
    return {**item, "node": node}


def catalog_status(status):
    if status == 401:
        return AgentError("平台登录已失效，请重新登录后重试", "catalog_auth_expired")
    return AgentError("资源目录暂不可用，请稍后重试")


class CatalogSearch:
    def __init__(self, client: httpx.AsyncClient, config: CatalogConfig):
        self.client, self.config = client, config

    async def search(self, value):
        args = search_arguments(value)
        resources, reports = [], []
        for kind in args["kinds"]:
            if kind != "file" and any(args.get(key) for key in ("school", "yearFrom", "yearTo", "fileType")):
                reports.append({"kind": kind, "skipped": True, "note": f"{LABELS[kind]}不支持学校、年份或试卷类型筛选，本次未查询"})
                continue
            params = {"keyword": args["keyword"], "pageSize": 50}
            if kind == "file":
                mapping = {"subject": "fileSubject", "school": "fileSchool", "fileType": "fileType",
                           "yearFrom": "yearFrom", "yearTo": "yearTo"}
                params.update({target: args[key] for key, target in mapping.items() if args.get(key)})
            elif args["subject"]:
                params["subject"] = args["subject"]
            found, scanned, total, invalid = {}, 0, 0, 0
            try:
                for page_number in range(1, 4):
                    # Fixed endpoints and query allowlist: model cannot choose a
                    # host, arbitrary route, owner, status, or HTTP method.
                    body = await read_json(
                        self.client, "GET", f"{self.config.base_url}/datum/{kind}/list",
                        params={**params, "pageNum": page_number}, timeout=10, limit=2 * 1024 * 1024,
                        headers={"Authorization": f"Bearer {self.config.token}"}, status_error=catalog_status,
                    )
                    page = body.get("data") if isinstance(body.get("data"), dict) else body
                    rows = page.get("rows", page.get("items"))
                    total = page.get("total")
                    if (body.get("code") not in (0, 200) or not isinstance(rows, list) or len(rows) > 50
                            or type(total) is not int or total < 0):
                        raise AgentError("资源目录响应格式不正确")
                    scanned += len(rows)
                    for row in rows:
                        item = resource(kind, row)
                        if item is None:
                            invalid += 1
                            continue
                        if args["subject"] and item["subject"] != args["subject"]:
                            continue
                        if kind == "file" and (
                            (args["school"] and text(row.get("fileSchool"), 8192) != args["school"])
                            or (args.get("fileType") and item["fileType"] != args["fileType"])
                            or (args.get("yearFrom") and (item["year"] or 0) < args["yearFrom"])
                            or (args.get("yearTo") and not 1900 <= (item["year"] or 0) <= args["yearTo"])
                        ):
                            continue
                        found[item["id"]] = item
                    if scanned >= total or not rows:
                        break
                resources.extend(list(found.values())[:5])
                reports.append({"kind": kind, "scanned": scanned, "catalogTotal": total, "matchedInScanned": len(found),
                                "shown": min(len(found), 5), "partial": scanned < total or len(found) > 5, "omittedInvalidRows": invalid})
            except (AgentError, httpx.HTTPError) as error:
                if isinstance(error, AgentError) and error.code == "catalog_auth_expired":
                    raise
                resources.extend(list(found.values())[:5])
                reports.append({"kind": kind, "failed": True, "partial": True, "shown": min(len(found), 5),
                                "note": f"{LABELS[kind]}检索未完成，请稍后重试；不能据此判断没有匹配资源"})
        return {"resources": resources, "modelResult": {
            "filters": args, "resources": [{key: value for key, value in item.items() if key != "node"} for item in resources],
            "reports": reports, "note": "仅返回真实目录元数据，未读取文件正文。每类最多检查150条并展示5项；partial表示部分结果。",
        }}
