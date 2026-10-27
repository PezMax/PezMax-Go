import json
import unittest

import httpx

from pezmax_agent.catalog import CatalogSearch
from pezmax_agent.contracts import CatalogConfig
from pezmax_agent.errors import AgentError

CONFIG = CatalogConfig.parse({"baseUrl": "https://catalog.example/api", "token": "platform-secret"})


def row(identifier=1, **extra):
    return {"fileId": identifier, "fileStatus": 1, "fileName": "数学", "fileUrl": "/files/math.pdf", "fileFormat": "pdf",
            "fileSubject": "高等数学", "fileSchool": "示例大学", "fileYear": 2024, "fileType": 1, **extra}


class CatalogTests(unittest.IsolatedAsyncioTestCase):
    async def search(self, handler, arguments):
        async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
            return await CatalogSearch(client, CONFIG).search(arguments)

    async def test_exact_filters_approved_rows_ids_and_private_nodes(self):
        seen = []

        def handler(request):
            seen.append(request)
            return httpx.Response(200, json={"code": 0, "rows": [row(9223372036854775807), row(2, fileStatus=0),
                row(3, fileYear=2020), row(4, fileSchool="别的大学"), row(5, fileUrl="javascript:alert(1)"), row(6, fileSubject="线性代数")], "total": 6})

        result = await self.search(handler, {"kinds": ["file"], "keyword": "数学", "subject": "高等数学", "school": "示例大学",
                                           "yearFrom": 2023, "yearTo": 2024, "fileType": 1})
        self.assertEqual(len(result["resources"]), 1)
        self.assertEqual(result["resources"][0]["node"]["rawId"], "9223372036854775807")
        self.assertEqual(result["resources"][0]["node"]["fileInfo"]["fileId"], "9223372036854775807")
        self.assertNotIn("/files/", json.dumps(result["modelResult"]))
        self.assertNotIn("platform-secret", json.dumps(result))
        request = seen[0]
        self.assertEqual(request.url.path, "/api/datum/file/list")
        self.assertEqual(request.url.params["yearFrom"], "2023")
        self.assertEqual(request.url.params["fileSubject"], "高等数学")
        self.assertEqual(request.headers["Authorization"], "Bearer platform-secret")
        self.assertNotIn("userId", request.url.params)

    async def test_pagination_is_capped_and_partial_report_is_honest(self):
        seen = []

        def handler(request):
            page = int(request.url.params["pageNum"])
            seen.append(page)
            return httpx.Response(200, json={"code": 0, "rows": [row(page * 100 + i) for i in range(50)], "total": 500})

        result = await self.search(handler, {"kinds": ["file"], "keyword": "数学"})
        self.assertEqual(seen, [1, 2, 3])
        self.assertEqual(len(result["resources"]), 5)
        self.assertEqual(result["modelResult"]["reports"][0]["scanned"], 150)
        self.assertTrue(result["modelResult"]["reports"][0]["partial"])

    async def test_later_page_failure_retains_real_results(self):
        def handler(request):
            if request.url.params["pageNum"] == "2":
                return httpx.Response(500, text="platform-secret")
            return httpx.Response(200, json={"code": 0, "rows": [row()], "total": 80})

        result = await self.search(handler, {"kinds": ["file"], "keyword": "数学"})
        self.assertEqual(len(result["resources"]), 1)
        self.assertTrue(result["modelResult"]["reports"][0]["failed"])
        self.assertNotIn("platform-secret", json.dumps(result))

    async def test_ebooks_bookmarks_use_subject_and_skip_unsupported_filters(self):
        seen = []

        def handler(request):
            seen.append(request)
            kind = request.url.path.split("/")[-2]
            item = ({"ebookId": 7, "ebookStatus": 1, "ebookName": "数学教材", "ebookSubject": "数学"}
                    if kind == "ebook" else {"id": 8, "status": 1, "title": "数学网站", "subject": "数学", "url": "https://learn.example"})
            return httpx.Response(200, json={"code": 0, "data": {"items": [item], "total": 1}})

        result = await self.search(handler, {"kinds": ["ebook", "bookmark"], "keyword": "数学", "subject": "数学"})
        self.assertEqual(len(result["resources"]), 2)
        self.assertTrue(all(request.url.params["subject"] == "数学" for request in seen))
        skipped = await self.search(handler, {"kinds": ["ebook", "bookmark"], "keyword": "数学", "yearFrom": 2024})
        self.assertEqual(len(seen), 2)
        self.assertTrue(all(report["skipped"] for report in skipped["modelResult"]["reports"]))

    async def test_authentication_failure_is_not_misreported_as_empty_catalog(self):
        with self.assertRaises(AgentError) as error:
            await self.search(lambda _request: httpx.Response(401, text="platform-secret"), {"kinds": ["file"], "keyword": "数学"})
        self.assertEqual(error.exception.code, "catalog_auth_expired")


if __name__ == "__main__":
    unittest.main()
