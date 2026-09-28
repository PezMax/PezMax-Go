// 分页拉取工具：后端 datum 列表的单页 pageSize 上限为 100（见 k_admin
// datumPageParams）。需要“全量”列表做本地过滤/分页的视图，按 total 逐页
// 拉取后再合并，单请求体积始终有界（原先一次 pageSize=1000 的拉法已失效）。

// fetchAllPages(fetcher, baseQuery) -> { rows, total }
// fetcher 收到 { ...baseQuery, pageNum, pageSize } 并返回 { rows, total } 形状
// 的响应（RuoYi TableDataInfo 或经 request.js 归一后的 kadmin 分页均可）。
export async function fetchAllPages(fetcher, baseQuery = {}, { pageSize = 100, maxPages = 100 } = {}) {
  const first = await fetcher({ ...baseQuery, pageNum: 1, pageSize })
  const rows = first?.rows ? [...first.rows] : []
  const total = Number(first?.total) || rows.length
  for (let pageNum = 2; pageNum <= maxPages && rows.length < total; pageNum++) {
    const res = await fetcher({ ...baseQuery, pageNum, pageSize })
    const part = res?.rows || []
    if (part.length === 0) break
    rows.push(...part)
  }
  return { rows, total }
}
