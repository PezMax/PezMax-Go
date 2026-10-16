import assert from 'node:assert/strict'
import test from 'node:test'
import createDOMPurify from 'dompurify'
import { JSDOM } from 'jsdom'
import { marked } from 'marked'

test('markdown keeps document content without executable HTML', () => {
  const window = new JSDOM('').window
  const sanitizer = createDOMPurify(window)
  const markdown = '# 文档\n\n<img src=x onerror="window.electronAPI.openPath(1)">\n\n[打开](javascript:alert(1))'
  const html = sanitizer.sanitize(marked.parse(markdown))
  const document = new JSDOM(html).window.document
  assert.equal(document.querySelector('h1')?.textContent, '文档')
  assert.equal(document.querySelector('img')?.hasAttribute('onerror'), false)
  assert.equal(document.querySelector('a')?.hasAttribute('href'), false)
  window.close()
})
