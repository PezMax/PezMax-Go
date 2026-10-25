import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createContext, SourceTextModule, SyntheticModule } from 'node:vm'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import {
    createUploadAssistantSession,
    createUploadAssistantState
} from '../src/renderer/utils/upload-assistant-session.mjs'
import * as uploadContracts from '../src/shared/upload-assistant.mjs'
import { createReadingAssistantSession } from '../src/renderer/utils/reading-assistant-session.mjs'
import {
    createAssistantSession,
    createAssistantState
} from '../src/renderer/utils/assistant-session.mjs'

// Execute the real SFC event handlers and lifecycle, without model credentials
// or network access. Ant's visual internals are covered by the production build.
async function mountAssistant(name, props = {}, imports = {}) {
    const context = createContext({ console })
    const filename = new URL(`../src/renderer/components/assistant/${name}.vue`, import.meta.url)
    const { descriptor } = parse(await readFile(filename, 'utf8'), { filename: filename.pathname })
    const { content } = compileScript(descriptor, { id: name })
    const antNames = [
        'Alert',
        'Button',
        'Card',
        'Checkbox',
        'Drawer',
        'Form',
        'FormItem',
        'Input',
        'InputPassword',
        'InputNumber',
        'Modal',
        'Spin',
        'Tag',
        'Textarea'
    ]
    const iconNames = [
        'ArrowRightOutlined',
        'ArrowUpOutlined',
        'BookOutlined',
        'DatabaseOutlined',
        'FileSearchOutlined',
        'FileTextOutlined',
        'KeyOutlined',
        'LinkOutlined',
        'LoadingOutlined',
        'LockOutlined',
        'PlusOutlined',
        'ReloadOutlined',
        'RobotOutlined',
        'SafetyCertificateOutlined',
        'SearchOutlined',
        'SettingOutlined',
        'StopOutlined'
    ]
    const exports = {
        vue: Vue,
        'ant-design-vue': Object.fromEntries(antNames.map((name) => [name, {}])),
        '@ant-design/icons-vue': Object.fromEntries(iconNames.map((name) => [name, {}])),
        './AssistantSettings.vue': { default: {} },
        './AssistantTheme.vue': { default: {} },
        '@/utils/assistant-session.mjs': { createAssistantSession, createAssistantState },
        '@/utils/upload-assistant-session.mjs': {
            createUploadAssistantSession,
            createUploadAssistantState
        },
        '../../../shared/upload-assistant.mjs': uploadContracts,
        ...imports
    }
    const module = new SourceTextModule(content, {
        context,
        initializeImportMeta: (meta) => {
            meta.env = { VITE_APP_TARGET_URL: 'https://pezmax.example' }
        }
    })
    await module.link((specifier) => {
        const values = exports[specifier]
        assert.ok(values, `Unexpected import ${specifier}`)
        return new SyntheticModule(
            Object.keys(values),
            function () {
                for (const [key, value] of Object.entries(values)) this.setExport(key, value)
            },
            { context }
        )
    })
    await module.evaluate()
    let bindings
    const emitted = []
    const renderer = Vue.createRenderer({
        createComment: () => ({}),
        insert() {},
        remove() {},
        parentNode: () => null,
        nextSibling: () => null
    })
    const app = renderer.createApp({
        setup(_props, ctx) {
            bindings = module.namespace.default.setup(props, {
                ...ctx,
                emit: (...args) => emitted.push(args)
            })
            return () => null
        }
    })
    app.mount({})
    const flush = async () => {
        for (let index = 0; index < 12; index++) {
            await Promise.resolve()
            await Vue.nextTick()
        }
    }
    await flush()
    return { bindings, emitted, flush, dispose: () => app.unmount() }
}

const config = {
    baseUrl: 'https://model.example/v1',
    model: 'tool-model',
    hasApiKey: false,
    remember: false,
    canRemember: true
}

test('reading UI requires explicit extraction and send, emits verified page jumps, and clears on file/account/close', async () => {
    const calls = [],
        user = Vue.reactive({ token: 'session', id: '7' })
    const props = Vue.reactive({
        documentId: 'pdf-1',
        source: 'blob:fixture-one',
        title: 'Synthetic PDF'
    })
    const pages = [{ page: 2, text: 'The derivative measures the rate of change.' }]
    const page = await mountAssistant('ReadingAssistant', props, {
        '@/store/modules/user': { default: () => user },
        '@/api/assistant': {
            assistantCall: async (method, scope, input) => {
                calls.push({ method, scope, input })
                if (method === 'getConfig') return { ...config, hasApiKey: true }
                if (method === 'run')
                    return {
                        answers: [
                            {
                                text: 'Rate of change.',
                                citations: [{ page: 2, quote: 'rate of change' }]
                            }
                        ],
                        contextPages: [2],
                        partial: false
                    }
            },
            onAssistantProgress: () => () => {}
        },
        '@/utils/reading-assistant-session.mjs': {
            createReadingAssistantState: (
                await import('../src/renderer/utils/reading-assistant-session.mjs')
            ).createReadingAssistantState,
            createReadingAssistantSession: (options) =>
                createReadingAssistantSession({
                    ...options,
                    loadPdf: async () => ({
                        numPages: 3,
                        pages: async () => pages,
                        destroy: async () => {}
                    })
                })
        }
    })
    const ui = page.bindings
    try {
        ui.open.value = true
        await page.flush()
        assert.equal(
            calls.some((call) => call.method === 'run'),
            false
        )
        ui.draft.value = 'What does the derivative measure?'
        await ui.submit()
        assert.equal(
            calls.some((call) => call.method === 'run'),
            false
        )
        ui.session.setRange(2, 2)
        await ui.session.extract()
        await ui.submit()
        ui.jump(9)
        assert.equal(
            page.emitted.some((event) => event[0] === 'jump-page'),
            false
        )
        ui.jump(2)
        const jump = page.emitted.find((event) => event[0] === 'jump-page')
        assert.deepEqual(JSON.parse(JSON.stringify(jump[1])), {
            documentId: 'pdf-1',
            source: 'blob:fixture-one',
            page: 2
        })
        props.source = 'blob:fixture-two'
        await page.flush()
        assert.equal(ui.open.value, false)
        assert.equal(ui.state.result, null)
        assert.equal(ui.state.pages.length, 0)
        assert.equal(ui.draft.value, '')
        ui.open.value = true
        await ui.session.extract()
        ui.close()
        assert.equal(ui.state.pages.length, 0)
        ui.open.value = true
        ui.draft.value = 'private question'
        user.token = ''
        await page.flush()
        assert.equal(ui.open.value, false)
        assert.equal(ui.draft.value, '')
        assert.equal(ui.signedIn.value, false)
    } finally {
        page.dispose()
    }
})

test('upload UI reuses settings and clears suggestions on file changes, upload and logout', async () => {
    const calls = [],
        user = Vue.reactive({ token: 'session', id: '7' })
    const props = Vue.reactive({
        kind: 'exam',
        file: { name: '2026高等数学期末.pdf' },
        form: { fileSubject: '' },
        locked: false
    })
    const page = await mountAssistant('UploadAssistant', props, {
        '@/store/modules/user': { default: () => user },
        '@/api/assistant': {
            assistantCall: async (method, scope, input) => {
                calls.push({ method, scope, input })
                if (method === 'getConfig') return { ...config, hasApiKey: true }
                if (method === 'run')
                    return {
                        suggestions: [
                            { field: 'fileSubject', value: '高等数学', evidence: '高等数学' }
                        ]
                    }
            },
            onAssistantProgress: () => () => {}
        }
    })
    const ui = page.bindings
    try {
        await ui.generate()
        assert.equal(props.form.fileSubject, '')
        ui.state.suggestions[0].selected = true
        ui.session.apply(props.form)
        assert.equal(props.form.fileSubject, '高等数学')
        // A different local file with the same name must invalidate the result.
        props.file = { name: props.file.name }
        await page.flush()
        assert.equal(ui.state.suggestions.length, 0)
        props.file = { name: '图书.pdf', isFolder: true, files: Array(21).fill({ name: 'a.pdf' }) }
        await page.flush()
        assert.match(ui.inputError.value, /20/)
        const previousCalls = calls.length
        await ui.generate()
        assert.equal(calls.length, previousCalls)
        props.file = { name: '2026高等数学期末.pdf' }
        await ui.generate()
        props.locked = true
        await page.flush()
        assert.equal(ui.state.suggestions.length, 0)
        user.token = ''
        await page.flush()
        assert.equal(ui.signedIn.value, false)
        assert.equal(ui.state.config.hasApiKey, false)
        assert.ok(calls.some((item) => item.method === 'release'))
    } finally {
        page.dispose()
    }
})

test('welcome prompts keep the question while opening settings, and IME Enter never sends', async () => {
    const calls = []
    const user = Vue.reactive({ token: 'session', id: '7' })
    const page = await mountAssistant(
        'AssistantWorkspace',
        { active: true },
        {
            '@/store/modules/user': { default: () => user },
            '@/api/assistant': {
                assistantCall: async (method, _scope, input) => {
                    calls.push(method)
                    if (method === 'getConfig') return { ...config }
                    if (method === 'save') return { ...config, hasApiKey: true }
                    if (method === 'run')
                        return { content: input.question, messages: [], resources: [], reports: [] }
                },
                onAssistantProgress: () => () => {}
            }
        }
    )
    const { bindings: ui } = page
    try {
        ui.chooseSuggestion('高等数学试卷')
        assert.equal(ui.draft.value, '高等数学试卷')
        assert.equal(ui.settingsOpen.value, true)
        await ui.submit()
        assert.equal(ui.draft.value, '高等数学试卷')
        assert.equal(calls.includes('run'), false)
        await ui.session.saveSettings({ ...config, apiKey: 'local-fixture' })
        ui.settingsOpen.value = false
        let prevented = false
        ui.handleComposerKey({
            key: 'Enter',
            isComposing: true,
            preventDefault() {
                prevented = true
            }
        })
        await page.flush()
        assert.equal(prevented, false)
        assert.equal(calls.includes('run'), false)
        ui.handleComposerKey({
            key: 'Enter',
            shiftKey: false,
            isComposing: false,
            keyCode: 13,
            preventDefault() {
                prevented = true
            }
        })
        await page.flush()
        assert.equal(prevented, true)
        assert.equal(ui.state.turns[0].content, '高等数学试卷')
        assert.equal(ui.draft.value, '')
        ui.draft.value = '旧账号未发送的内容'
        ui.settingsOpen.value = true
        user.token = ''
        await page.flush()
        assert.equal(ui.draft.value, '')
        assert.equal(ui.settingsOpen.value, false)
        assert.equal(ui.state.turns.length, 0)
    } finally {
        page.dispose()
    }
})

test('settings submit credentials only to the local save boundary and clear the field after saving or closing', async () => {
    const localConfig = Vue.reactive({ ...config })
    const saved = []
    const props = Vue.reactive({
        open: true,
        config: localConfig,
        busy: false,
        saveSettings: async (input) => {
            saved.push({ ...input })
            localConfig.hasApiKey = true
            return '已保存'
        },
        clearKey: async () => {
            localConfig.hasApiKey = false
        }
    })
    const page = await mountAssistant('AssistantSettings', props)
    try {
        page.bindings.form.apiKey = 'fixture-secret'
        await page.bindings.save(false)
        assert.equal(saved[0].apiKey, 'fixture-secret')
        assert.equal(page.bindings.form.apiKey, '')
        assert.equal(
            page.emitted.some(([name, value]) => name === 'update:open' && value === false),
            true
        )
        page.bindings.form.apiKey = 'unsaved-fixture'
        props.open = false
        await page.flush()
        assert.equal(page.bindings.form.apiKey, '')
        props.open = true
        await page.flush()
        await page.bindings.clear()
        assert.equal(localConfig.hasApiKey, false)
        assert.equal(page.bindings.form.remember, false)
    } finally {
        page.dispose()
    }
})
