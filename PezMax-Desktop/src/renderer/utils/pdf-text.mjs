// Parsing is lazy and local. Model requests never see the PDF URL or buffer.
const limit = 50 * 1024 * 1024
class PdfReadError extends Error {}
const stopped = () => new PdfReadError('已停止读取PDF')
function abortable(promise, signal) {
    if (!signal) return promise
    if (signal.aborted) {
        Promise.resolve(promise).catch(() => {})
        return Promise.reject(stopped())
    }
    return new Promise((resolve, reject) => {
        const abort = () => reject(stopped())
        signal.addEventListener('abort', abort, { once: true })
        Promise.resolve(promise)
            .then(resolve, reject)
            .finally(() => signal.removeEventListener('abort', abort))
    })
}
function publicError(error, signal) {
    return signal?.aborted
        ? stopped()
        : error instanceof PdfReadError
          ? error
          : new PdfReadError('PDF文字提取失败，可能已加密、损坏或无法访问')
}
export async function loadPdfText(source, { signal, getDocument, fetcher = fetch } = {}) {
    let url
    try {
        url = new URL(source, globalThis.location?.href || 'https://reader.invalid')
    } catch {
        throw new PdfReadError('请求的PDF地址无效')
    }
    if (!['http:', 'https:', 'blob:'].includes(url.protocol) || url.username || url.password)
        throw new PdfReadError('请求的PDF地址无效')
    url.hash = ''
    let task, destroyPromise
    const destroy = () => {
        signal?.removeEventListener('abort', abort)
        if (!destroyPromise) destroyPromise = Promise.resolve().then(() => task?.destroy()).catch(() => {})
        return destroyPromise
    }
    const abort = () => {
        void destroy()
    }
    signal?.addEventListener('abort', abort, { once: true })
    try {
        signal?.throwIfAborted()
        let response
        try {
            response = await abortable(
                fetcher(url.href, {
                    signal,
                    redirect: 'error',
                    credentials: 'same-origin',
                    cache: 'no-cache'
                }),
                signal
            )
        } catch {
            if (signal?.aborted) throw stopped()
            throw new PdfReadError(
                'PDF下载失败，请检查网络；若预览正常，请联系管理员检查文件读取配置'
            )
        }
        if (!response.ok) throw new PdfReadError('PDF读取失败，请确认当前资料仍可预览')
        if (+response.headers.get('Content-Length') > limit) {
            void response.body?.cancel().catch(() => {})
            throw new PdfReadError('PDF超过50MB，暂不支持阅读问答')
        }
        if (!response.body) throw new PdfReadError('PDF读取失败，请重新打开资料')
        const chunks = [],
            reader = response.body.getReader()
        let length = 0
        try {
            while (true) {
                const { value, done } = await abortable(reader.read(), signal)
                if (done) break
                length += value.byteLength
                if (length > limit) throw new PdfReadError('PDF超过50MB，暂不支持阅读问答')
                chunks.push(value)
            }
        } finally {
            void reader.cancel().catch(() => {})
        }
        const buffer = new Uint8Array(length)
        let offset = 0
        for (const chunk of chunks) {
            buffer.set(chunk, offset)
            offset += chunk.length
        }
        signal?.throwIfAborted()
        if (!getDocument) {
            const pdfjs = await import('pdfjs-dist/build/pdf.mjs')
            const { default: workerUrl } = await import('pdfjs-dist/build/pdf.worker.min.mjs?url')
            pdfjs.GlobalWorkerOptions.workerSrc = workerUrl
            getDocument = pdfjs.getDocument
        }
        signal?.throwIfAborted()
        task = getDocument({
            data: buffer,
            isEvalSupported: false,
            useSystemFonts: true,
            stopAtErrors: true
        })
        let rejectPassword
        const password = new Promise((_resolve, reject) => {
            rejectPassword = reject
        })
        task.onPassword = () => {
            rejectPassword(new PdfReadError('PDF已加密，暂不支持阅读问答'))
            void destroy()
        }
        const document = await abortable(Promise.race([task.promise, password]), signal)
        if (
            !Number.isInteger(document.numPages) ||
            document.numPages < 1 ||
            document.numPages > 10000
        )
            throw new PdfReadError('PDF页数无效或超过10000，暂不支持阅读问答')
        signal?.throwIfAborted()
        return {
            numPages: document.numPages,
            destroy,
            async pages(from, to) {
                try {
                    if (
                        !Number.isInteger(from) ||
                        !Number.isInteger(to) ||
                        from < 1 ||
                        to < from ||
                        to > document.numPages ||
                        to - from >= 10
                    )
                        throw new PdfReadError('请选择PDF内连续1到10页')
                    const result = []
                    let total = 0
                    for (let number = from; number <= to; number++) {
                        signal?.throwIfAborted()
                        const page = await abortable(document.getPage(number), signal)
                        try {
                            const content = await abortable(page.getTextContent(), signal)
                            const text = content.items
                                .map((item) =>
                                    typeof item.str === 'string'
                                        ? item.str + (item.hasEOL ? '\n' : ' ')
                                        : ''
                                )
                                .join('')
                                .trim()
                            total += text.length
                            if (text.length > 20000 || total > 80000)
                                throw new PdfReadError(
                                    'PDF文字超出限制，请缩小页码范围或换用较短资料'
                                )
                            result.push({ page: number, text })
                        } finally {
                            page.cleanup()
                        }
                    }
                    signal?.throwIfAborted()
                    if (!result.some((page) => page.text.trim()))
                        throw new PdfReadError('所选页未提取到文字，可能是扫描件；本阶段不支持OCR')
                    return result
                } catch (error) {
                    throw publicError(error, signal)
                }
            }
        }
    } catch (error) {
        await destroy()
        throw publicError(error, signal)
    }
}
