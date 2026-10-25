import test from 'node:test'
import assert from 'node:assert/strict'
import { loadPdfText } from '../src/renderer/utils/pdf-text.mjs'
import { readingPdf } from './fixtures/reading-pdf.mjs'

function fakePdf({ text = 'Synthetic text for reading.', numPages = 3, content } = {}) {
    let destroyed = 0,
        cleaned = 0
    const getDocument = () => ({
        destroy: async () => {
            destroyed++
        },
        promise: Promise.resolve({
            numPages,
            getPage: async () => ({
                getTextContent: content || (async () => ({ items: [{ str: text }] })),
                cleanup() {
                    cleaned++
                }
            })
        })
    })
    return { getDocument, counts: () => ({ destroyed, cleaned }) }
}
const fetcher = async () => new Response(readingPdf())

test('real PDF.js extracts the selected physical page only from a synthetic multi-page PDF', async () => {
    const { getDocument } = await import('pdfjs-dist/legacy/build/pdf.mjs')
    const pdf = await loadPdfText('https://fixture.example/math.pdf', { fetcher, getDocument })
    try {
        assert.equal(pdf.numPages, 3)
        const pages = await pdf.pages(2, 2)
        assert.deepEqual(pages, [{ page: 2, text: 'The derivative measures the rate of change.' }])
    } finally {
        await pdf.destroy()
    }
})

test('PDF loader restricts protocols, credentials, redirects and declared/streamed sizes', async () => {
    for (const url of [
        'file:///private.pdf',
        'javascript:alert(1)',
        'https://user:pass@fixture.example/a.pdf'
    ])
        await assert.rejects(loadPdfText(url, { fetcher }), /地址无效/)
    const fake = fakePdf()
    let request
    const pdf = await loadPdfText('https://fixture.example/a.pdf?token=fixture#page=2', {
        ...fake,
        fetcher: async (url, options) => {
            request = { url, options }
            return fetcher()
        }
    })
    assert.equal(request.url, 'https://fixture.example/a.pdf?token=fixture')
    assert.equal(request.options.redirect, 'error')
    assert.equal(request.options.credentials, 'same-origin')
    await pdf.destroy()
    await pdf.destroy()
    assert.equal(fake.counts().destroyed, 1)
    await assert.rejects(
        loadPdfText('https://fixture/a.pdf', {
            fetcher: async () =>
                new Response('x', { headers: { 'Content-Length': 51 * 1024 * 1024 } })
        }),
        /50MB/
    )
    await assert.rejects(
        loadPdfText('https://fixture/a.pdf', {
            fetcher: async () => new Response(new Uint8Array(50 * 1024 * 1024 + 1))
        }),
        /50MB/
    )
})

test('PDF ranges, empty scans, page/text limits and corrupt PDFs have bounded public errors', async () => {
    const f = fakePdf(),
        pdf = await loadPdfText('blob:fixture', { fetcher, ...f })
    for (const range of [
        [0, 1],
        [2, 1],
        [1, 4],
        [1.5, 2],
        [1, 11]
    ])
        await assert.rejects(pdf.pages(...range), /1到10页/)
    await pdf.destroy()
    for (const [options, pattern] of [
        [{ text: '' }, /OCR/],
        [{ text: 'x'.repeat(20001) }, /超出限制/],
        [{ numPages: 10001 }, /页数/]
    ]) {
        const fake = fakePdf(options)
        let doc
        try {
            doc = await loadPdfText('blob:fixture', { fetcher, ...fake })
            await doc.pages(1, 1)
        } catch (error) {
            assert.match(error.message, pattern)
        } finally {
            await doc?.destroy()
        }
        assert.equal(fake.counts().destroyed, 1)
    }
    await assert.rejects(
        loadPdfText('blob:fixture', {
            fetcher,
            getDocument: () => ({
                promise: Promise.reject(new Error('private-source?token=secret')),
                destroy: async () => {}
            })
        }),
        (error) => !error.message.includes('private-source') && /损坏/.test(error.message)
    )
})

test('abort during parsing or text extraction destroys worker and cleans pages; password protected files fail promptly', async () => {
    for (const phase of ['parsing', 'text']) {
        const controller = new AbortController()
        let destroyed = 0,
            cleaned = 0
        const getDocument = () => ({
            destroy: async () => {
                destroyed++
            },
            promise:
                phase === 'parsing'
                    ? new Promise(() => {})
                    : Promise.resolve({
                          numPages: 1,
                          getPage: async () => ({
                              getTextContent: () => new Promise(() => {}),
                              cleanup: () => {
                                  cleaned++
                              }
                          })
                      })
        })
        const loading = loadPdfText('blob:fixture', {
            fetcher,
            getDocument,
            signal: controller.signal
        })
        const pending = phase === 'text' ? (await loading).pages(1, 1) : loading
        const rejected = assert.rejects(pending, /停止/)
        await new Promise((resolve) => setTimeout(resolve, 15))
        controller.abort()
        await rejected
        assert.equal(destroyed, 1)
        assert.equal(cleaned, phase === 'text' ? 1 : 0)
    }
    let destroyed = 0
    await assert.rejects(
        loadPdfText('blob:fixture', {
            fetcher,
            getDocument: () => {
                const task = {
                    promise: new Promise(() => {}),
                    destroy: async () => {
                        destroyed++
                    }
                }
                queueMicrotask(() => task.onPassword())
                return task
            }
        }),
        /加密/
    )
    assert.equal(destroyed, 1)
})
