import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createContext, SourceTextModule, SyntheticModule } from 'node:vm'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'

// Compile the real page scripts and mount their Vue lifecycle. Network and
// Electron boundaries are substituted; the page event handlers remain real.
export async function mountPage(path, imports = {}, window = {}) {
  const context = createContext({ console, window, document: { body: { style: {} }, addEventListener() {}, removeEventListener() {} }, setTimeout, clearTimeout })
  const filename = new URL(`../../src/renderer/${path}`, import.meta.url)
  const { descriptor } = parse(await readFile(filename, 'utf8'), { filename: filename.pathname })
  const { content } = compileScript(descriptor, { id: path })
  const modules = new Map()
  const defaults = {
    vue: Vue,
    '@element-plus/icons-vue': Object.fromEntries(['Refresh', 'Warning', 'CircleClose', 'Close', 'Search', 'Delete', 'Download', 'FolderOpened', 'Plus', 'Link', 'VideoCamera', 'Document', 'Reading', 'Box', 'Film', 'ArrowRight', 'Folder', 'Upload', 'Edit', 'Location', 'Files', 'Collection', 'Picture', 'ArrowUp', 'ArrowDown', 'View', 'Select'].map(name => [name, {}])),
    '@/assets/logo/logo.png': { default: '' },
    '@/utils/auth': { getToken: () => 'test-token' },
    '@/utils/validate': { isEmpty: (value) => !value, isHttp: (value) => /^https?:/.test(value) },
    '@/utils/avatar': { normalizeAvatar: (value) => value },
    '@/utils/clientStorage': { getStorageItem: () => '', setStorageItem() {} },
    ...imports
  }
  const module = new SourceTextModule(content, { context, identifier: path, initializeImportMeta: (meta) => { meta.env = { VITE_APP_BASE_API: '/dev-api' } } })
  await module.link((specifier) => {
    const exports = defaults[specifier]
    assert.ok(exports, `Unexpected import ${specifier}`)
    if (!modules.has(specifier)) modules.set(specifier, new SyntheticModule(Object.keys(exports), function () {
      for (const [key, value] of Object.entries(exports)) this.setExport(key, value)
    }, { context }))
    return modules.get(specifier)
  })
  await module.evaluate()
  const page = module.namespace.default
  let bindings
  const renderer = Vue.createRenderer({
    createComment: () => ({}), createElement: () => ({}), createText: () => ({}),
    insert() {}, remove() {}, setText() {}, setElementText() {}, patchProp() {},
    parentNode: () => null, nextSibling: () => null
  })
  const app = renderer.createApp({
    setup(props, ctx) {
      bindings = page.setup(props, ctx)
      return () => null
    }
  })
  app.mount({})
  const flush = async () => {
    for (let i = 0; i < 8; i++) { await Promise.resolve(); await Vue.nextTick() }
  }
  await flush()
  return { bindings, flush, dispose: () => app.unmount() }
}

export const plain = (value) => JSON.parse(JSON.stringify(value))
export const emptyMessage = Object.fromEntries(['success', 'warning', 'info', 'error'].map((name) => [name, () => {}]))

export async function mountUpload({ code = 0, schools = ['示例大学'], subjects = [{ Subject: '高等数学', Total: 3 }, { Subject: '大学物理', Total: 2 }], exists = false, subject = '高等数学' } = {}) {
  const confirms = []
  const store = Vue.reactive({ uploadStep: 1, uploadKind: 'exam', selectedFile: { name: 'paper.pdf', size: 100 }, uploadProgress: {}, uploadForm: { fileName: 'paper', fileSchool: '示例大学', fileSubject: subject, fileYear: '2026' }, performUpload: async () => ({ success: true }) })
  const page = await mountPage('views/home/components/UploadPanel.vue', {
    'element-plus': { ElMessage: emptyMessage, ElMessageBox: { confirm: async (...args) => { confirms.push(args) } } },
    '@/store/modules/upload': { default: () => store },
    '@/api/datum/file': {
      getSchools: async () => ({ code, data: schools }),
      getSubjects: async () => ({ code, data: subjects }),
      checkSchoolExists: async () => ({ code, data: { exists } })
    }
  }, { electronAPI: { uploadFile() {} } })
  return { ...page, store, confirms }
}
