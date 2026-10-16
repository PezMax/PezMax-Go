const collator = new Intl.Collator('zh-Hans-CN', { numeric: true, sensitivity: 'base' })

export function sortFileTree(nodes) {
  if (!Array.isArray(nodes)) return []
  return [...nodes]
    .sort((left, right) => {
      const folderOrder = Number(right.type === 'folder') - Number(left.type === 'folder')
      if (folderOrder) return folderOrder
      return collator.compare(String(left.label || ''), String(right.label || ''))
    })
    .map((node) => Array.isArray(node.children)
      ? { ...node, children: sortFileTree(node.children) }
      : node)
}
