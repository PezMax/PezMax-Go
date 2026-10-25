import test from 'node:test'
import assert from 'node:assert/strict'
import JSZip from 'jszip'
import epubModule from 'epubjs'
import { JSDOM } from 'jsdom'
import { DOMParser } from '@xmldom/xmldom'
import { createEpubTextReader, epubMarkupText } from '../src/renderer/utils/epub-text.mjs'
import { readingEpub } from './fixtures/reading-epub.mjs'

async function fixture() {
    const zip = await JSZip.loadAsync(await readingEpub())
    const spineItems = ['one', 'two', 'three'].map((name, index) => ({
        index,
        href: `${name}.xhtml`,
        url: `/OPS/${name}.xhtml`,
        load() {
            throw new Error('Rendition sections must not be loaded or changed')
        }
    }))
    const book = {
        archived: true,
        opened: Promise.resolve(),
        archive: { zip },
        spine: { spineItems, get: (href) => spineItems.find((item) => item.href === href) },
        destroy() {
            throw new Error('The shared reader book must not be destroyed')
        }
    }
    return book
}

test(
    'EPUB extraction interoperates with a real epub.js book and preserves its spine',
    { timeout: 10000 },
    async () => {
        const bytes = await readingEpub()
        const dom = new JSDOM('', { url: 'https://reader.invalid' })
        const previous = new Map(
            ['window', 'document', 'DOMParser', 'Node'].map((key) => [key, globalThis[key]])
        )
        for (const key of previous.keys())
            globalThis[key] = key === 'window' ? dom.window : dom.window[key]
        const ePub = epubModule.default || epubModule
        const book = ePub()
        try {
            await book.open(
                bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength)
            )
            await book.opened
            const navigation = await book.loaded.navigation
            const reader = await createEpubTextReader(book, {
                Parser: DOMParser,
                toc: navigation.toc
            })
            assert.equal(reader.numChapters, 3)
            assert.match(reader.units[1].label, /Derivatives/)
            assert.deepEqual(await reader.chapters([2]), [
                { chapter: 2, text: 'Section 2 The derivative measures the rate of change.' }
            ])
            await reader.destroy()
            assert.equal(book.spine.get(1).href, 'two.xhtml')
        } finally {
            try {
                book.destroy()
            } finally {
                dom.window.close()
                for (const [key, value] of previous) {
                    if (value === undefined) delete globalThis[key]
                    else globalThis[key] = value
                }
            }
        }
    }
)

test('EPUB extraction reads only selected archive sections, retaining original spine indexes and local labels', async () => {
    const book = await fixture()
    const reader = await createEpubTextReader(book, {
        Parser: DOMParser,
        toc: [{ href: 'two.xhtml', label: 'Derivatives' }]
    })
    assert.equal(reader.numChapters, 3)
    assert.equal(reader.units[1].label, '第 2 节 · Derivatives')
    assert.deepEqual(await reader.chapters([2]), [
        { chapter: 2, text: 'Section 2 The derivative measures the rate of change.' }
    ])
    await reader.destroy()
    assert.equal(book.spine.spineItems.length, 3)
    await assert.rejects(reader.chapters([2]), /停止/)
})

test('EPUB text omits executable, hidden and URL attributes while preserving visible block text', () => {
    const markup =
        '<html xmlns="http://www.w3.org/1999/xhtml"><head><title>hidden title</title></head><body><h1>Visible title</h1><p>Public <a href="https://private-source/?token=secret">link label</a></p><script>script-secret</script><style>style-secret</style><iframe src="https://private-source"/><span hidden="hidden">hidden-secret</span><div style="display:none">css-secret</div><object>object-secret</object><p>Last paragraph.</p></body></html>'
    assert.equal(
        epubMarkupText(markup, DOMParser),
        'Visible title Public link label Last paragraph.'
    )
    assert.throws(
        () => epubMarkupText('<!DOCTYPE html [<!ENTITY ex "private">]><html/>', DOMParser),
        /格式不支持/
    )
})

test('EPUB limits, invalid sections and aborts fail locally without raw paths or tokens', async () => {
    const book = await fixture(),
        controller = new AbortController()
    const reader = await createEpubTextReader(book, {
        Parser: DOMParser,
        signal: controller.signal
    })
    for (const numbers of [[], [0], [4], [1, 1], [true], [1, 2, 3, 4, 5, 6]])
        await assert.rejects(reader.chapters(numbers), /1到5节/)
    book.archive.zip.file(
        'OPS/two.xhtml',
        '<html xmlns="http://www.w3.org/1999/xhtml"><body>' + 'x'.repeat(40001) + '</body></html>'
    )
    await assert.rejects(reader.chapters([2]), /超出限制/)
    book.spine.spineItems[1].url = 'https://private-source/?token=secret'
    await assert.rejects(
        reader.chapters([2]),
        (error) => /地址无效/.test(error.message) && !error.message.includes('secret')
    )
    controller.abort()
    await assert.rejects(reader.chapters([1]), /停止/)
    await reader.destroy()
    await assert.rejects(
        createEpubTextReader({ opened: Promise.reject(new Error('private-token')) }),
        (error) => !error.message.includes('private-token')
    )
})

test('EPUB parsing is abortable while the existing book is opening', async () => {
    const controller = new AbortController()
    const loading = createEpubTextReader(
        { opened: new Promise(() => {}) },
        { signal: controller.signal }
    )
    const stopped = assert.rejects(loading, /停止/)
    controller.abort()
    await stopped
})
