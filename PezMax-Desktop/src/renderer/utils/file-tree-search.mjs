// Keep the path to filename matches so users can still see school/subject/year.
// A matching folder keeps its subtree; the cached source tree is never mutated.
// Ancestors kept only for their matches are flagged so the tree can auto-expand them.
export function searchFileTree(nodes, query) {
    if (!Array.isArray(nodes)) return []
    const keyword = String(query || '')
        .trim()
        .toLowerCase()
    if (!keyword) return nodes
    return nodes.flatMap((node) => {
        if (
            String(node.label || '')
                .toLowerCase()
                .includes(keyword)
        )
            return [node]
        const children = searchFileTree(node.children, keyword)
        return children.length ? [{ ...node, children, _searchPath: true }] : []
    })
}
