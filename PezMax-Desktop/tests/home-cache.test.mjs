import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { createContext, SourceTextModule, SyntheticModule } from 'node:vm'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import * as VueRouter from 'vue-router'

// Use the real SFCs, router and Vue lifecycle with a DOM-free renderer. Child
// views and backend/Electron APIs are isolated so this runs without a login.
async function createWorkspace() {
    const calls = { tree: 0, popup: 0, scroll: 0, editorMounts: 0, editorUnmounts: 0 }
    const instances = new Map()
    const timeouts = new Map()
    const intervals = new Map()
    const downloadListeners = new Set()
    const storage = new Map()
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
                'favoriteEbookIds'
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
            getFileTree: async () => {
                calls.tree++
                return { code: 0, data: [] }
            }
        },
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
            listEbookFavorite: emptyRows,
            addEbookFavorite: emptyRows,
            delEbookFavorite: emptyRows
        },
        '@/api/datum/favorite': {
            listFavorite: emptyRows,
            addFavorite: emptyRows,
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
            ['Document', 'Close', 'Minus', 'FullScreen', 'CircleClose'].map((name) => [
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
            if (imports[specifier]) return synthetic(specifier, imports[specifier])
            if (specifier.endsWith('.vue')) {
                const name = specifier.split('/').at(-1).replace('.vue', '')
                return synthetic(specifier, { default: stub(name) })
            }
            if (/\.(png|jpg)$/.test(specifier)) return synthetic(specifier, { default: '' })
            throw new Error(`Unmocked import: ${specifier}`)
        })
        await module.evaluate()
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
    const remove = (target) => {
        if (target.parent) target.parent.children.splice(target.parent.children.indexOf(target), 1)
        target.parent = null
    }
    const renderer = Vue.createRenderer({
        createElement: node,
        createText: (text) => node('text', text),
        createComment: (text) => node('comment', text),
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
    app.mount(node('root'))
    await flush()
    return {
        calls,
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
