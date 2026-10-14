import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { createContext, SourceTextModule, SyntheticModule } from 'node:vm'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import * as VueRouter from 'vue-router'

// Use the real SFCs, router and Vue lifecycle with a DOM-free renderer. Child
// views and backend/Electron APIs are isolated so this runs without a login.
async function createWorkspace({ treeResponses = [], initialTreeCache, realTreeUi = false, getFileDetail } = {}) {
    const calls = { tree: 0, treeQueries: [], detail: [], favorite: [], popup: 0, scroll: 0, editorMounts: 0, editorUnmounts: 0 }
    const instances = new Map()
    const timeouts = new Map()
    const intervals = new Map()
    const downloadListeners = new Set()
    const storage = new Map()
    if (initialTreeCache) storage.set('ptmj_file_tree_cache', JSON.stringify(initialTreeCache))
    let timerId = 0
    const window = new EventTarget()
    window.electronAPI = {
        getSettings: async () => ({ shortcuts: { closeTab: 'CommandOrControl+W' } }),
        onDownloadProgress: (callback) => {
            downloadListeners.add(callback)
            return () => downloadListeners.delete(callback)
        },
        onWindowMaximized: () => {},
        onSessionExpired: () => () => {}
    }
    const document = {
        body: { style: {} },
        documentElement: { classList: { contains: () => false } },
        addEventListener() {},
        removeEventListener() {}
    }
    const sessionStorage = {
        getItem: (key) => storage.get(key) ?? null,
        setItem: (key, value) => storage.set(key, value),
        removeItem: (key) => storage.delete(key)
    }
    const userStore = Vue.reactive({ token: 'session-1', id: 'user-1', avatar: '' })
    userStore.getInfo = async () => ({ user: { userId: userStore.id } })
    const router = VueRouter.createRouter({
        history: VueRouter.createMemoryHistory(),
        routes: []
    })
    const context = createContext({
        console: { ...console, log() {} },
        window,
        document,
        navigator: { userAgent: 'Windows' },
        localStorage: sessionStorage,
        sessionStorage,
        URL,
        Blob,
        CustomEvent: class extends Event {
            constructor(type, options = {}) {
                super(type, options)
                this.detail = options.detail
            }
        },
        useRoute: VueRouter.useRoute,
        useRouter: VueRouter.useRouter,
        setTimeout: (callback) => {
            timeouts.set(++timerId, callback)
            return timerId
        },
        clearTimeout: (id) => timeouts.delete(id),
        setInterval: (callback) => {
            intervals.set(++timerId, callback)
            return timerId
        },
        clearInterval: (id) => intervals.delete(id)
    })
    const modules = new Map()
    const stub = (name) =>
        Vue.defineComponent({
            name,
            props: [
                'openTabs',
                'activeTab',
                'activeView',
                'width',
                'visible',
                'text',
                'favoriteFileIds',
                'favoriteBookmarkIds',
                'favoriteEbookIds',
                'fileTreeData',
                'fileInfo',
                'modelValue'
            ],
            setup(props, { emit }) {
                const instance = { props, emit }
                Vue.onMounted(() => {
                    instances.set(name, instance)
                    if (name === 'MainEditor') calls.editorMounts++
                })
                Vue.onUnmounted(() => {
                    if (name === 'MainEditor') calls.editorUnmounts++
                })
                return () => Vue.h('stub', { name })
            }
        })
    function synthetic(name, exports) {
        if (!modules.has(name)) {
            modules.set(
                name,
                new SyntheticModule(
                    Object.keys(exports),
                    function () {
                        for (const [key, value] of Object.entries(exports))
                            this.setExport(key, value)
                    },
                    { context }
                )
            )
        }
        return modules.get(name)
    }
    const emptyRows = async () => ({ code: 0, rows: [], data: [] })
    const imports = {
        vue: { ...Vue, Transition: Vue.BaseTransition },
        'vue-router': VueRouter,
        'element-plus': {
            ElMessage: Object.assign(() => {}, { success() {}, warning() {}, error() {} }),
            ElMessageBox: {}
        },
        '@/store/modules/user': { default: () => userStore },
        '@/store/modules/upload': { default: () => ({ isUploading: false, uploadProgress: {} }) },
        '@/api/datum/file': {
            getFileTree: async (query) => {
                calls.tree++
                calls.treeQueries.push(query)
                return treeResponses.shift() || { code: 0, data: [] }
            },
            getFile: async (id) => {
                calls.detail.push(id)
                return getFileDetail ? getFileDetail(id) : { code: 0, data: null }
            }
        },
        '@/api/datum/user': { getUser: emptyRows },
        '@/api/datum/bookmark': { listBookmark: emptyRows, addBookmark: emptyRows, updateBookmark: emptyRows },
        axios: { default: {} },
        '@/api/datum/notification': {
            getUserPopupNotifications: async () => {
                calls.popup++
                return { code: 0, data: [] }
            },
            getUserScrollNotifications: async () => {
                calls.scroll++
                return { code: 0, data: [], hash: 'notice-1' }
            }
        },
        '@/api/datum/ebook': {
            ebookDownloadUrl: () => '',
            ebookPreviewUrl: (id) => `/ebook/${id}`,
            listEbook: emptyRows,
            listEbookFavorite: emptyRows,
            addEbookFavorite: emptyRows,
            delEbookFavorite: emptyRows
        },
        '@/api/datum/favorite': {
            listFavorite: emptyRows,
            addFavorite: async (data) => {
                calls.favorite.push(data)
                return emptyRows()
            },
            delFavorite: emptyRows
        },
        '@/api/datum/bookmarkFavorite': {
            listBookmarkFavorite: emptyRows,
            addBookmarkFavorite: emptyRows,
            delBookmarkFavorite: emptyRows
        },
        '@/utils/auth': { getToken: () => userStore.token },
        '@/utils/clientStorage': {
            getStorageItem: sessionStorage.getItem,
            setStorageItem: sessionStorage.setItem
        },
        '@/utils/url': { normalizeFileUrl: (url) => url },
        '@/utils/avatar': { normalizeAvatar: (avatar) => avatar },
        '@/utils/pagination': { fetchAllPages: (api) => api() },
        '@/utils/request': { handleSessionExpired: async () => {} },
        '@/utils/ideAppearance': {
            applyIdeAppearanceFromSettings: async () => {},
            applyIdeThemeState() {},
            teardownIdeAppearanceMediaListener() {}
        },
        '@/constants/ptmjAuth': {
            isPtmjAuthRoute: (path) => ['/login', '/register', '/forgotPassword'].includes(path)
        },
        '@element-plus/icons-vue': Object.fromEntries(
            ['Document', 'Close', 'Minus', 'FullScreen', 'CircleClose', 'CopyDocument', 'Download',
                'DocumentCopy', 'Warning', 'Select', 'Star', 'StarFilled', 'Refresh', 'Plus', 'Link',
                'Search', 'Delete', 'VideoCamera', 'Reading', 'Box', 'Film', 'ArrowRight', 'Folder',
                'Upload', 'Edit', 'Location', 'Files', 'Collection', 'Picture', 'ArrowUp', 'ArrowDown',
                'View'].map((name) => [
                name,
                stub(name)
            ])
        )
    }
    async function loadSfc(relativePath) {
        const filename = new URL(`../src/renderer/${relativePath}`, import.meta.url)
        const { descriptor } = parse(await readFile(filename, 'utf8'), {
            filename: filename.pathname
        })
        const { content } = compileScript(descriptor, { id: relativePath, inlineTemplate: true })
        const module = new SourceTextModule(content, {
            context,
            identifier: relativePath,
            initializeImportMeta: (meta) => {
                meta.env = { VITE_APP_BASE_API: '/dev-api' }
            }
        })
        await module.link(async (specifier) => {
            if (specifier === '@/components/TitleHeader/index.vue') {
                return loadSfc('components/TitleHeader/index.vue')
            }
            if (realTreeUi && ['./components/SidePanel.vue', './components/FileInfoDrawer.vue'].includes(specifier)) {
                return loadSfc(`views/home/${specifier.slice(2)}`)
            }
            if (imports[specifier]) return synthetic(specifier, imports[specifier])
            if (specifier.endsWith('.vue')) {
                const name = specifier.split('/').at(-1).replace('.vue', '')
                return synthetic(specifier, { default: stub(name) })
            }
            if (/\.(png|jpg)$/.test(specifier)) return synthetic(specifier, { default: '' })
            throw new Error(`Unmocked import: ${specifier}`)
        })
        await module.evaluate()
        if (realTreeUi && /\/(SidePanel|FileInfoDrawer)\.vue$/.test(relativePath)) {
            const name = relativePath.split('/').at(-1).replace('.vue', '')
            const component = module.namespace.default
            const setup = component.setup
            component.setup = (props, context) => {
                instances.set(name, { props, emit: context.emit })
                return setup(props, context)
            }
        }
        return module
    }
    const [appModule, homeModule] = await Promise.all([
        loadSfc('App.vue'),
        loadSfc('views/home/index.vue')
    ])
    router.addRoute({ path: '/index', name: 'Index', component: homeModule.namespace.default })
    router.addRoute({ path: '/datum/ptmj-user', component: stub('UserCenter') })
    router.addRoute({ path: '/datum/favorite', component: stub('Favorite') })
    router.addRoute({ path: '/login', component: stub('Login') })
    const node = (type, text = '') => ({ type, text, children: [], parent: null, style: {} })
    const teleportRoot = node('body')
    const remove = (target) => {
        if (target.parent) target.parent.children.splice(target.parent.children.indexOf(target), 1)
        target.parent = null
    }
    const renderer = Vue.createRenderer({
        createElement: node,
        createText: (text) => node('text', text),
        createComment: (text) => node('comment', text),
        querySelector: () => teleportRoot,
        setText: (target, text) => {
            target.text = text
        },
        setElementText: (target, text) => {
            target.text = text
        },
        parentNode: (target) => target.parent,
        nextSibling: (target) =>
            target.parent?.children[target.parent.children.indexOf(target) + 1] ?? null,
        insert(target, parent, anchor = null) {
            if (target.parent) remove(target)
            const index = anchor ? parent.children.indexOf(anchor) : -1
            parent.children.splice(index < 0 ? parent.children.length : index, 0, target)
            target.parent = parent
        },
        remove,
        patchProp: (target, key, _previous, value) => {
            target[key] = value
        }
    })
    const app = renderer.createApp(appModule.namespace.default)
    // Render the real tree rows and drawer content while isolating Element Plus DOM internals.
    if (realTreeUi) {
        app.component('ElTree', Vue.defineComponent({
            props: ['data'],
            setup(props, { slots }) {
                const rows = (nodes) => nodes.flatMap((data) => [
                    Vue.h('tree-row', {}, slots.default?.({ node: { label: data.label }, data })),
                    ...rows(data.children || [])
                ])
                return () => Vue.h('tree', {}, rows(props.data || []))
            }
        }))
        app.component('ElDrawer', Vue.defineComponent({
            props: ['modelValue'],
            setup: (props, { slots }) => () => props.modelValue ? Vue.h('drawer', {}, slots.default?.()) : null
        }))
        for (const name of ['ElIcon', 'ElAvatar']) {
            app.component(name, Vue.defineComponent({ setup: (_props, { slots }) => () => Vue.h('slot', {}, slots.default?.()) }))
        }
        app.directive('loading', {})
    }
    app.use(router)
    app.config.warnHandler = (message) => {
        if (!message.startsWith('Failed to resolve component:')) throw new Error(message)
    }
    async function flush() {
        for (let i = 0; i < 12; i++) {
            await Vue.nextTick()
            for (const [id, callback] of [...timeouts]) {
                timeouts.delete(id)
                callback()
            }
        }
    }
    await router.push('/index')
    await router.isReady()
    const root = node('root')
    app.mount(root)
    await flush()
    return {
        calls,
        root,
        instances,
        router,
        userStore,
        window,
        sessionStorage,
        intervals,
        downloadListeners,
        flush,
        dispose: () => app.unmount(),
        async navigate(path) {
            await router.push(path)
            await flush()
        },
        async openPaper(id) {
            instances
                .get('SidePanel')
                .emit('node-click', { id, type: 'file', label: `${id}.pdf`, url: `/${id}.pdf` })
            await flush()
        }
    }
}

test('home retains both paper tabs and does not initialize again after repeated user-center visits', async (t) => {
    const workspace = await createWorkspace()
    t.after(workspace.dispose)
    await workspace.openPaper('paper-1')
    await workspace.openPaper('paper-2')
    const editor = workspace.instances.get('MainEditor')
    const initialCalls = { ...workspace.calls }
    for (let i = 0; i < 3; i++) {
        await workspace.navigate('/datum/ptmj-user')
        // A cached home must not react to shortcuts on another page.
        const event = new Event('keydown', { cancelable: true })
        Object.assign(event, { key: 'w', ctrlKey: true })
        workspace.window.dispatchEvent(event)
        await workspace.flush()
        assert.equal(editor.props.openTabs.length, 2)
        await workspace.navigate('/index')
        assert.equal(workspace.instances.get('MainEditor'), editor)
        assert.deepEqual(
            Array.from(editor.props.openTabs, (tab) => tab.id),
            ['paper-1', 'paper-2']
        )
        assert.equal(editor.props.activeTab, 'paper-2')
        assert.equal(workspace.instances.get('GlobalLoader').props.visible, false)
    }
    assert.deepEqual(workspace.calls, initialCalls)
    assert.equal(workspace.downloadListeners.size, 1)
    // Explicit refresh and the existing notification poll still work.
    workspace.instances.get('SidePanel').emit('refresh')
    for (const callback of workspace.intervals.values()) await callback()
    await workspace.flush()
    assert.equal(workspace.calls.tree, initialCalls.tree + 1)
    assert.equal(workspace.calls.scroll, initialCalls.scroll + 1)
    assert.equal(editor.props.openTabs.length, 2)
})

test('a cached home consumes a favorite preview once and ignores other routes query changes', async (t) => {
    const workspace = await createWorkspace()
    t.after(workspace.dispose)
    await workspace.openPaper('paper-1')
    const editor = workspace.instances.get('MainEditor')
    await workspace.navigate('/index?view=ebook')
    await workspace.navigate('/datum/favorite')
    for (const detail of [
        { fileId: 11, favorited: true },
        { bookmarkId: 22, type: 'bookmark', favorited: true },
        { ebookId: 33, type: 'ebook', favorited: true }
    ]) {
        const event = new Event('favorite-updated')
        Object.assign(event, { detail })
        workspace.window.dispatchEvent(event)
    }
    await workspace.flush()
    assert.equal(workspace.instances.get('SidePanel').props.activeView, 'ebook')
    assert.equal(workspace.calls.tree, 1)
    workspace.sessionStorage.setItem(
        'pendingOpenFile',
        JSON.stringify({
            id: 'paper-2',
            type: 'file',
            label: 'paper-2.pdf',
            url: '/paper-2.pdf'
        })
    )
    await workspace.navigate('/index?view=ebook')
    assert.equal(workspace.instances.get('MainEditor'), editor)
    assert.deepEqual(
        Array.from(editor.props.openTabs, (tab) => tab.id),
        ['paper-1', 'paper-2']
    )
    assert.equal(editor.props.activeTab, 'paper-2')
    assert.equal(workspace.sessionStorage.getItem('pendingOpenFile'), null)
    assert.equal(workspace.calls.tree, 1)
    await workspace.navigate('/datum/ptmj-user')
    await workspace.navigate('/index')
    assert.equal(workspace.instances.get('SidePanel').props.activeView, 'ebook')
    assert.deepEqual(Array.from(editor.props.favoriteFileIds), ['11'])
    assert.deepEqual(Array.from(editor.props.favoriteBookmarkIds), ['22'])
    assert.deepEqual(Array.from(editor.props.favoriteEbookIds), ['33'])
    assert.equal(editor.props.openTabs.length, 2)
    assert.equal(workspace.calls.editorMounts, 1)
})

test('logout discards the cached home so the next session starts with fresh tabs and data', async (t) => {
    const workspace = await createWorkspace()
    t.after(workspace.dispose)
    await workspace.openPaper('paper-1')
    const previousEditor = workspace.instances.get('MainEditor')
    await workspace.navigate('/datum/ptmj-user')
    workspace.userStore.token = ''
    await workspace.navigate('/login')
    assert.equal(workspace.calls.editorUnmounts, 1)
    assert.equal(workspace.downloadListeners.size, 0)
    workspace.userStore.token = 'session-2'
    workspace.userStore.id = 'user-2'
    await workspace.navigate('/index')
    const nextEditor = workspace.instances.get('MainEditor')
    assert.notEqual(nextEditor, previousEditor)
    assert.equal(nextEditor.props.openTabs.length, 0)
    assert.equal(workspace.calls.editorMounts, 2)
    assert.equal(workspace.calls.tree, 2)
    assert.equal(workspace.calls.popup, 2)
    assert.equal(workspace.downloadListeners.size, 1)
    // Only the new home's shortcut handler should close the current paper.
    await workspace.openPaper('paper-2')
    const event = new Event('keydown', { cancelable: true })
    Object.assign(event, { key: 'w', ctrlKey: true })
    workspace.window.dispatchEvent(event)
    await workspace.flush()
    assert.equal(nextEditor.props.openTabs.length, 0)
})

test('clearing the session on home unmounts its cache without initializing an anonymous workspace', async (t) => {
    const workspace = await createWorkspace()
    t.after(workspace.dispose)
    await workspace.openPaper('paper-1')
    workspace.userStore.token = ''
    // Session-expiry cleanup can happen before the login navigation finishes.
    await workspace.flush()
    assert.equal(workspace.calls.editorUnmounts, 1)
    assert.equal(workspace.calls.editorMounts, 1)
    assert.equal(workspace.calls.tree, 1)
    assert.equal(workspace.calls.popup, 1)
    assert.equal(workspace.downloadListeners.size, 0)
    await workspace.navigate('/login')
    workspace.userStore.token = 'session-2'
    await workspace.navigate('/index')
    assert.equal(workspace.calls.editorMounts, 2)
    assert.equal(workspace.calls.tree, 2)
    assert.equal(workspace.instances.get('MainEditor').props.openTabs.length, 0)
})

const paperInfo = {
    fileId: 42,
    fileName: '高等数学试卷.pdf',
    fileUrl: '/papers/42.pdf',
    fileFormat: 'pdf',
    fileStatus: '1',
    fileSchool: '示例大学',
    fileSubject: '高等数学',
    fileType: 1,
    fileYear: 2025,
    fileSize: 2048,
    createTime: '2025-06-20 10:00:00'
}

const findNodes = (root, predicate) => [
    ...(predicate(root) ? [root] : []),
    ...root.children.flatMap((child) => findNodes(child, predicate))
]
const renderedText = (root) => findNodes(root, (node) => Boolean(node.text)).map((node) => node.text).join(' ')
const plain = (value) => JSON.parse(JSON.stringify(value))

test('tree file entities keep their badges, drawer metadata, preview URLs and database IDs', async (t) => {
    for (const representation of ['flat', 'fileInfo', 'ptmjFile']) {
        await t.test(representation, async (t) => {
            const entity = { ...paperInfo }
            const file = representation === 'flat'
                ? { ...entity, id: 'file-42', type: 'file' }
                : { id: 'file-42', label: entity.fileName, [representation]: entity }
            const workspace = await createWorkspace({
                realTreeUi: true,
                treeResponses: [{ code: 0, hash: 'tree-1', data: [{
                    id: 'subject-math', label: '高等数学', type: 'folder', children: [
                        file,
                        { ...paperInfo, fileId: 43, fileStatus: 0 },
                        { ...paperInfo, fileId: 44, fileStatus: '2' }
                    ]
                }] }]
            })
            t.after(workspace.dispose)
            const sidePanel = workspace.instances.get('SidePanel')
            const folder = sidePanel.props.fileTreeData[0]
            assert.equal(folder.type, 'folder')
            assert.equal(folder.fileInfo, undefined)
            assert.equal(folder.children.length, 1)
            const node = folder.children[0]
            assert.equal(node.id, 'file-42')
            assert.equal(node.fileInfo.fileId, 42)
            assert.equal(node.fileInfo.fileStatus, '1')
            assert.equal(node.children, null)
            assert.equal(node.url, 'http://127.0.0.1:9033/papers/42.pdf')
            const badge = findNodes(workspace.root, (item) => item.class?.includes('tree-status-badge'))[0]
            assert.equal(badge.text, '通过')
            assert.match(badge.class, /status-approved/)
            // Click the actual tree details action and inspect the rendered drawer.
            const infoAction = findNodes(workspace.root, (item) => item.title === '文件详情')[0]
            infoAction.onClick(new Event('click'))
            await workspace.flush()
            assert.deepEqual(workspace.calls.detail, [42])
            const drawer = findNodes(workspace.root, (item) => item.type === 'drawer')[0]
            for (const value of ['高等数学试卷.pdf', '示例大学', '高等数学', '期末', '2025', '2.00 KB', paperInfo.createTime]) {
                assert.ok(renderedText(drawer).includes(value), `drawer is missing ${value}`)
            }
            sidePanel.emit('node-click', node)
            sidePanel.emit('toggle-favorite', node)
            await workspace.flush()
            const tab = workspace.instances.get('MainEditor').props.openTabs[0]
            assert.equal(tab.id, 'file-42')
            assert.equal(tab.originalData.fileId, 42)
            assert.equal(tab.url, node.url)
            assert.equal(workspace.calls.favorite[0].fileId, '42')
            const cache = JSON.parse(workspace.sessionStorage.getItem('ptmj_file_tree_cache'))
            assert.equal(cache.version, 2)
            assert.deepEqual(cache.tree[0].children[0].fileInfo, plain(node.fileInfo))
        })
    }
})

test('old formatted tree caches are replaced without sending their hash', async (t) => {
    const workspace = await createWorkspace({
        initialTreeCache: { hash: 'same-backend-hash', tree: [{ id: 'file-42', label: paperInfo.fileName, type: 'file' }] },
        treeResponses: [{ code: 0, hash: 'same-backend-hash', data: [paperInfo] }, { code: 0, unchanged: true }]
    })
    t.after(workspace.dispose)
    assert.equal(workspace.calls.treeQueries[0], undefined)
    const panel = workspace.instances.get('SidePanel')
    assert.equal(panel.props.fileTreeData[0].fileInfo.fileId, 42)
    assert.equal(JSON.parse(workspace.sessionStorage.getItem('ptmj_file_tree_cache')).version, 2)
    panel.emit('refresh')
    await workspace.flush()
    assert.equal(workspace.calls.treeQueries[1].hash, 'same-backend-hash')
    assert.equal(panel.props.fileTreeData[0].fileInfo.fileName, paperInfo.fileName)
})

test('drawer metadata uses fetched details for both response codes and retains tree fallback fields', async (t) => {
    for (const code of [0, 200]) {
        await t.test(`code=${code}`, async (t) => {
            let resolveDetail
            const pendingDetail = new Promise((resolve) => { resolveDetail = resolve })
            const workspace = await createWorkspace({
                realTreeUi: true,
                treeResponses: [{ code: 0, data: [paperInfo] }],
                getFileDetail: () => pendingDetail
            })
            t.after(workspace.dispose)
            const panel = workspace.instances.get('SidePanel')
            const file = panel.props.fileTreeData[0]
            panel.emit('open-info', {}, file)
            await workspace.flush()
            assert.ok(renderedText(workspace.root).includes('示例大学'))
            resolveDetail({ code, data: {
                fileId: 42,
                fileName: '补充详情.pdf',
                fileSchool: '详情大学',
                fileSubject: '大学物理',
                fileType: 2,
                fileYear: 2026,
                fileSize: 4096,
                createTime: '2026-06-01 12:00:00'
            } })
            await workspace.flush()
            const drawer = findNodes(workspace.root, (item) => item.type === 'drawer')[0]
            for (const value of ['补充详情.pdf', '详情大学', '大学物理', '期中', '2026', '4.00 KB', '2026-06-01 12:00:00', '审核通过']) {
                assert.ok(renderedText(drawer).includes(value), `drawer is missing ${value}`)
            }
            assert.ok(!renderedText(drawer).includes('示例大学'))
        })
    }
})

test('switching files in an open drawer ignores the earlier pending detail response', async (t) => {
    let resolveFirst
    const workspace = await createWorkspace({
        realTreeUi: true,
        treeResponses: [{ code: 0, data: [paperInfo, { ...paperInfo, fileId: 43, fileName: '当前文件.pdf' }] }],
        getFileDetail: (id) => id === 42
            ? new Promise((resolve) => { resolveFirst = resolve })
            : Promise.resolve({ code: 0, data: { fileId: 43, fileSchool: '当前大学' } })
    })
    t.after(workspace.dispose)
    const panel = workspace.instances.get('SidePanel')
    panel.emit('open-info', {}, panel.props.fileTreeData[0])
    await workspace.flush()
    panel.emit('open-info', {}, panel.props.fileTreeData[1])
    await workspace.flush()
    resolveFirst({ code: 0, data: { fileId: 42, fileName: '过时详情.pdf', fileSchool: '过时大学' } })
    await workspace.flush()
    const drawer = findNodes(workspace.root, (item) => item.type === 'drawer')[0]
    assert.ok(renderedText(drawer).includes('当前文件.pdf'))
    assert.ok(renderedText(drawer).includes('当前大学'))
    assert.ok(!renderedText(drawer).includes('过时'))
    assert.deepEqual(workspace.calls.detail, [42, 43])
})
