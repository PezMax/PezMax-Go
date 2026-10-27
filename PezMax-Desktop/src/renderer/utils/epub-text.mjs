// Read only entries from the already-open EPUB archive. No rendition hooks,
// scripts, image requests, extra book download or filesystem access are used.
class EpubReadError extends Error {}
const maxMarkup = 2 * 1024 * 1024
const stopped = () => new EpubReadError('已停止读取EPUB')
function waitFor(promise, signal) {
    return new Promise((resolve, reject) => {
        const abort = () => reject(stopped())
        if (signal?.aborted) {
            Promise.resolve(promise).catch(() => {})
            reject(stopped())
            return
        }
        signal?.addEventListener('abort', abort, { once: true })
        Promise.resolve(promise)
            .then(resolve, reject)
            .finally(() => signal?.removeEventListener('abort', abort))
    })
}
function entryText(entry, signal) {
    if (!entry || entry.dir) throw new EpubReadError('所选EPUB分节不存在或无法读取')
    if (entry._data?.uncompressedSize > maxMarkup)
        throw new EpubReadError('EPUB分节过大，请选择较短分节')
    return new Promise((resolve, reject) => {
        const stream = entry.internalStream('string')
        let text = '',
            size = 0,
            settled = false
        const finish = (error) => {
            if (settled) return
            settled = true
            signal?.removeEventListener('abort', abort)
            if (error) {
                stream.pause()
                text = ''
                reject(error)
            } else resolve(text)
        }
        const abort = () => finish(stopped())
        if (signal?.aborted) {
            abort()
            return
        }
        signal?.addEventListener('abort', abort, { once: true })
        stream
            .on('data', (chunk) => {
                if (settled) return
                size += new TextEncoder().encode(chunk).byteLength
                if (size > maxMarkup) {
                    finish(new EpubReadError('EPUB分节过大，请选择较短分节'))
                    return
                }
                text += chunk
            })
            .on('error', () => finish(new EpubReadError('EPUB分节解压失败，可能已加密或损坏')))
            .on('end', () => finish())
            .resume()
    })
}
export function epubMarkupText(markup, Parser = globalThis.DOMParser) {
    if (
        typeof markup !== 'string' ||
        markup.length > maxMarkup ||
        /<!ENTITY|<!DOCTYPE[^>]*\[/i.test(markup)
    )
        throw new EpubReadError('EPUB分节格式不支持或内容过大')
    const document = new Parser().parseFromString(markup, 'application/xhtml+xml')
    if (document.getElementsByTagName('parsererror').length)
        throw new EpubReadError('EPUB分节格式无效')
    const body = document.getElementsByTagNameNS('*', 'body')[0]
    if (!body) throw new EpubReadError('EPUB分节没有可读取的正文')
    const skip = new Set(['script', 'style', 'noscript', 'iframe', 'object', 'embed', 'head'])
    const block = new Set([
        'p',
        'div',
        'section',
        'article',
        'h1',
        'h2',
        'h3',
        'h4',
        'h5',
        'h6',
        'li',
        'tr',
        'br',
        'blockquote',
        'td',
        'th'
    ])
    let result = ''
    function visit(node) {
        if (node.nodeType === 3 || node.nodeType === 4) {
            result += node.nodeValue || ''
            return
        }
        if (node.nodeType !== 1) return
        const name = (node.localName || node.nodeName).toLowerCase()
        if (
            skip.has(name) ||
            node.hasAttribute('hidden') ||
            node.getAttribute('aria-hidden') === 'true' ||
            /(?:display\s*:\s*none|visibility\s*:\s*hidden)/i.test(node.getAttribute('style') || '')
        )
            return
        if (block.has(name)) result += '\n'
        for (let child = node.firstChild; child; child = child.nextSibling) visit(child)
        if (block.has(name)) result += '\n'
    }
    visit(body)
    return result.replace(/\s+/g, ' ').trim()
}

export async function createEpubTextReader(openBook, { signal, toc = [], Parser } = {}) {
    let book = openBook,
        alive = true
    const ensure = () => {
        if (!alive || signal?.aborted) throw stopped()
    }
    const safe = (error) =>
        signal?.aborted
            ? stopped()
            : error instanceof EpubReadError
              ? error
              : new EpubReadError('EPUB文字读取失败，请等待阅读器载入或重新打开电子书')
    try {
        if (!book) throw new EpubReadError('EPUB尚未载入，请稍后再试')
        await waitFor(book.opened, signal)
        ensure()
        const spine = book.spine?.spineItems
        if (
            !book.archived ||
            !book.archive?.zip ||
            !Array.isArray(spine) ||
            !spine.length ||
            spine.length > 10000
        )
            throw new EpubReadError('EPUB分节目录无效或暂不支持此格式')
        const labels = new Map()
        for (const item of toc) {
            const section = book.spine.get(item.href)
            if (section && !labels.has(section.index))
                labels.set(
                    section.index,
                    String(item.label || '')
                        .replace(/\s+/g, ' ')
                        .trim()
                        .slice(0, 160)
                )
        }
        const units = spine.map((section, index) => ({
            chapter: index + 1,
            label: `第 ${index + 1} 节${labels.get(section.index) ? ` · ${labels.get(section.index)}` : ''}`
        }))
        return {
            units,
            numChapters: units.length,
            async chapters(numbers) {
                try {
                    ensure()
                    if (
                        !Array.isArray(numbers) ||
                        !numbers.length ||
                        numbers.length > 5 ||
                        new Set(numbers).size !== numbers.length ||
                        numbers.some(
                            (number) =>
                                !Number.isInteger(number) || number < 1 || number > spine.length
                        )
                    )
                        throw new EpubReadError('请选择1到5节EPUB正文')
                    const chapters = []
                    let total = 0
                    for (const number of [...numbers].sort((a, b) => a - b)) {
                        ensure()
                        const source = spine[number - 1].url
                        if (
                            typeof source !== 'string' ||
                            !source.startsWith('/') ||
                            source.startsWith('//')
                        )
                            throw new EpubReadError('EPUB分节地址无效')
                        const path = decodeURIComponent(source.slice(1).split('#')[0])
                        if (path.split('/').includes('..') || path.includes('\0'))
                            throw new EpubReadError('EPUB分节地址无效')
                        const markup = await entryText(book.archive.zip.file(path), signal)
                        ensure()
                        const text = epubMarkupText(markup, Parser)
                        total += text.length
                        if (text.length > 40000 || total > 80000)
                            throw new EpubReadError('EPUB文字超出限制，请减少分节或选择较短分节')
                        chapters.push({ chapter: number, text })
                    }
                    if (!chapters.some((chapter) => chapter.text))
                        throw new EpubReadError('所选EPUB分节没有可读取文字；图片内容暂不支持OCR')
                    return chapters
                } catch (error) {
                    throw safe(error)
                }
            },
            async destroy() {
                alive = false
                book = null
            }
        }
    } catch (error) {
        throw safe(error)
    }
}
