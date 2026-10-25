export const uploadFields = {
  fileName: '资源名称', fileSchool: '学校', fileSubject: '学科', fileYear: '年份',
  fileType: '资料类型', ebookType: '电子书类型'
}
export const uploadTypes = {
  fileType: { 1: '期末', 2: '期中', 3: '补考', 4: '资料', 5: '其他学校', 6: '保持神秘' },
  ebookType: { 1: '教材', 2: '教辅/参考书', 3: '课外读物', 4: '其他' }
}
export function normalizeUploadInput(input) {
  if (!input || !['exam', 'ebook'].includes(input.kind) || typeof input.folder !== 'boolean' ||
      !Array.isArray(input.names) || !input.names.length || input.names.length > 20 ||
      (!input.folder && input.names.length !== 1) || (input.folder && input.kind !== 'exam')) {
    throw new Error('请选择单个文件或不超过20个文件的文件夹')
  }
  if (input.names.some(name => typeof name !== 'string' || !name.trim() || name.length > 240 ||
      /[/\\\x00-\x1f\x7f]/.test(name) || ['.', '..'].includes(name))) throw new Error('请求中的文件名无效；仅支持文件名，不接受路径')
  return { kind: input.kind, folder: input.folder, names: [...input.names] }
}
export function normalizeUploadSuggestions(items, input) {
  if (!Array.isArray(items)) throw new Error('模型返回的上传建议格式无效，请重试')
  const allowed = input.kind === 'ebook' ? ['fileName', 'fileSubject', 'ebookType'] : ['fileName', 'fileSchool', 'fileSubject', 'fileYear', 'fileType']
  const seen = new Set()
  return items.slice(0, 6).filter(item => {
    if (!item || !allowed.includes(item.field) || seen.has(item.field) || (input.folder && item.field === 'fileName')) return false
    const { field, value, evidence } = item
    if (typeof evidence !== 'string' || !evidence || evidence.length > 240 || !input.names.every(name => name.includes(evidence))) return false
    if (uploadTypes[field]) {
      if (!Number.isInteger(value) || !uploadTypes[field][value]) return false
    } else if (typeof value !== 'string' || !value.trim() || value.length > (field === 'fileName' ? 200 : 100) || /[/\\\x00-\x1f\x7f]/.test(value)) return false
    if (field === 'fileYear' && (!/^\d{4}$/.test(value) || +value < 1900 || +value > new Date().getFullYear() + 1 ||
        !input.names.every(name => { const years = [...new Set(name.match(/(?<!\d)(?:19|20)\d{2}(?!\d)/g) || [])]; return years.length === 1 && years[0] === value }))) return false
    if (field === 'fileSchool' && !input.names.every(name => name.includes(value))) return false
    seen.add(field)
    return true
  }).map(({ field, value, evidence }) => ({ field, value: typeof value === 'string' ? value.trim() : value, evidence }))
}
