const identifier = /^[a-f0-9]{32}$/
const fail = () => {
    throw new Error('请求的知识库数据无效，请更新助手后重试')
}
const id = (value) => {
    if (typeof value !== 'string' || !identifier.test(value)) fail()
    return value
}

// Only these fields cross from renderer to main. Storage paths, account owner,
// model settings and selected filesystem paths are owned by the main process.
export function normalizeKnowledgeInput(input) {
    if (
        !input ||
        !['list', 'import', 'delete', 'search', 'ask', 'read', 'sync', 'open'].includes(
            input.action
        )
    )
        fail()
    if (input.library !== undefined && !['local', 'platform'].includes(input.library)) fail()
    if (['sync', 'open'].includes(input.action) && input.library !== 'platform') fail()
    if (input.library === 'platform' && ['import', 'delete'].includes(input.action)) fail()
    const mode = input.mode ?? 'keyword'
    if (!['keyword', 'hybrid'].includes(mode)) fail()
    const result = { action: input.action, mode }
    if (input.library !== undefined) result.library = input.library
    if (['search', 'ask'].includes(input.action)) {
        const question = input.query?.question
        const ids = input.query?.documentIds ?? []
        if (
            typeof question !== 'string' ||
            !question.trim() ||
            question.length > 2000 ||
            !Array.isArray(ids) ||
            ids.length > 50 ||
            new Set(ids).size !== ids.length
        )
            fail()
        result.query = { question: question.trim(), documentIds: ids.map(id), mode, limit: 6 }
    }
    if (['delete', 'read', 'open'].includes(input.action)) result.documentId = id(input.documentId)
    if (['read', 'open'].includes(input.action)) result.chunkId = id(input.chunkId)
    return result
}

export function normalizePlatformSource(value) {
    if (
        !value ||
        !['file', 'ebook'].includes(value.kind) ||
        typeof value.id !== 'string' ||
        !/^[1-9]\d{0,18}$/.test(value.id) ||
        typeof value.revision !== 'string' ||
        !/^[a-f0-9]{64}$/.test(value.revision) ||
        typeof value.title !== 'string' ||
        !value.title ||
        value.title.length > 255 ||
        !['pdf', 'epub', 'txt', 'md'].includes(value.format) ||
        typeof value.subject !== 'string' ||
        value.subject.length > 160 ||
        typeof value.school !== 'string' ||
        value.school.length > 160 ||
        !Number.isInteger(value.year) ||
        value.year < 0 ||
        value.year > 9999
    )
        fail()
    return Object.fromEntries(
        ['kind', 'id', 'revision', 'title', 'format', 'subject', 'school', 'year'].map((key) => [
            key,
            value[key]
        ])
    )
}

// Use the configured platform origin and fixed content API, never a stored URL
// or model-produced navigation target. The content API rechecks public status.
export function platformKnowledgeNode(source, baseUrl, location) {
    source = normalizePlatformSource(source)
    if (!Number.isInteger(location) || location < 1 || location > 10000) fail()
    const url = new URL(`${baseUrl.replace(/\/$/, '')}/datum/knowledge/content`)
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) fail()
    for (const key of ['kind', 'id', 'revision']) url.searchParams.set(key, source[key])
    const node = {
        type: source.kind,
        id: `${source.kind}-${source.id}`,
        rawId: source.id,
        label: source.title,
        fileExt: source.format,
        url: url.href,
        knowledgeLocation: location,
        knowledgeRevision: source.revision
    }
    if (source.kind === 'file')
        node.fileInfo = {
            fileId: source.id,
            fileName: source.title,
            fileFormat: source.format,
            fileUrl: url.href,
            fileSubject: source.subject,
            fileSchool: source.school,
            fileYear: source.year,
            fileStatus: 1
        }
    else
        node.originalData = {
            ebookId: source.id,
            ebookName: source.title,
            ebookFormat: source.format,
            ebookSubject: source.subject,
            ebookStatus: 1
        }
    return node
}

function chunk(value) {
    if (
        !value ||
        !Number.isInteger(value.location) ||
        value.location < 1 ||
        value.location > 10000 ||
        !['pdf', 'epub', 'text'].includes(value.format) ||
        typeof value.title !== 'string' ||
        !value.title ||
        value.title.length > 200 ||
        typeof value.text !== 'string' ||
        !value.text ||
        value.text.length > 1200
    )
        fail()
    return {
        documentId: id(value.documentId),
        chunkId: id(value.chunkId),
        location: value.location,
        title: value.title,
        format: value.format,
        text: value.text,
        ...(value.source ? { source: normalizePlatformSource(value.source) } : {})
    }
}

function document(value) {
    if (
        !value ||
        typeof value.title !== 'string' ||
        !value.title ||
        value.title.length > 200 ||
        !['pdf', 'epub', 'text'].includes(value.format) ||
        !Number.isInteger(value.characters) ||
        value.characters < 1 ||
        !Number.isInteger(value.chunks) ||
        value.chunks < 1 ||
        value.chunks > 3000 ||
        !Number.isInteger(value.dimension) ||
        value.dimension < 0 ||
        value.dimension > 4096 ||
        typeof value.created !== 'string' ||
        value.created.length > 80
    )
        fail()
    return {
        documentId: id(value.documentId),
        title: value.title,
        format: value.format,
        characters: value.characters,
        chunks: value.chunks,
        dimension: value.dimension,
        created: value.created,
        ...(value.source ? { source: normalizePlatformSource(value.source) } : {})
    }
}

export function normalizeKnowledgeResult(value, request) {
    if (!value || typeof value !== 'object') fail()
    const coverage = {}
    if (request.library === 'platform') {
        const c = value.coverage
        if (
            !c ||
            !Number.isInteger(c.available) ||
            c.available < 0 ||
            c.available > 200 ||
            !Number.isInteger(c.total) ||
            c.total < c.available ||
            typeof c.partial !== 'boolean' ||
            c.partial !== c.total > c.available
        )
            fail()
        coverage.coverage = { available: c.available, total: c.total, partial: c.partial }
    }
    if (value.cancelled === true && request.action === 'import') return { cancelled: true }
    if (['list', 'sync'].includes(request.action)) {
        if (!Array.isArray(value.documents) || value.documents.length > 200) fail()
        const result = { documents: value.documents.map(document), ...coverage }
        if (request.library === 'platform' && result.documents.some((doc) => !doc.source)) fail()
        if (request.action === 'sync') {
            const s = value.sync
            if (
                !s ||
                !Number.isInteger(s.indexed) ||
                s.indexed < 0 ||
                s.indexed > 200 ||
                !Number.isInteger(s.unchanged) ||
                s.unchanged < 0 ||
                s.unchanged > 200 ||
                !Array.isArray(s.failures) ||
                s.failures.length > 200
            )
                fail()
            result.sync = {
                indexed: s.indexed,
                unchanged: s.unchanged,
                failures: s.failures.map((item) => {
                    if (
                        typeof item?.title !== 'string' ||
                        item.title.length > 200 ||
                        typeof item.reason !== 'string' ||
                        item.reason.length > 200
                    )
                        fail()
                    return { title: item.title, reason: item.reason }
                })
            }
        }
        return result
    }
    if (request.action === 'import') {
        if (typeof value.document?.inserted !== 'boolean') fail()
        return { document: { ...document(value.document), inserted: value.document.inserted } }
    }
    if (request.action === 'delete') {
        if (typeof value.deleted !== 'boolean') fail()
        return { deleted: value.deleted }
    }
    if (['read', 'open'].includes(request.action)) {
        const result = chunk(value.chunk)
        if (result.documentId !== request.documentId || result.chunkId !== request.chunkId) fail()
        if (request.library === 'platform' && !result.source) fail()
        return { chunk: result, ...coverage }
    }
    const retrieval = value.retrieval
    if (
        !retrieval ||
        !Array.isArray(retrieval.chunks) ||
        retrieval.chunks.length > 6 ||
        retrieval.mode !== request.mode ||
        !['bm25', 'rrf-k60'].includes(retrieval.fusion) ||
        !Number.isInteger(retrieval.scannedChunks) ||
        retrieval.scannedChunks < 0 ||
        retrieval.scannedChunks > 10000 ||
        !Number.isInteger(retrieval.vectorChunks) ||
        retrieval.vectorChunks < 0 ||
        retrieval.vectorChunks > retrieval.scannedChunks
    )
        fail()
    const chunks = retrieval.chunks.map(chunk)
    if (request.library === 'platform' && chunks.some((item) => !item.source)) fail()
    if (
        request.query.documentIds.length &&
        chunks.some((c) => !request.query.documentIds.includes(c.documentId))
    )
        fail()
    const result = {
        ...coverage,
        retrieval: {
            chunks,
            mode: retrieval.mode,
            scannedChunks: retrieval.scannedChunks,
            vectorChunks: retrieval.vectorChunks,
            fusion: retrieval.fusion
        }
    }
    if (request.action === 'search') return result
    if (
        !Array.isArray(value.answers) ||
        value.answers.length > 5 ||
        typeof value.content !== 'string' ||
        value.content.length > 8200
    )
        fail()
    result.answers = value.answers.map((answer) => {
        if (
            typeof answer?.text !== 'string' ||
            !answer.text ||
            answer.text.length > 1600 ||
            !Array.isArray(answer.citations) ||
            answer.citations.length < 1 ||
            answer.citations.length > 3
        )
            fail()
        const citations = answer.citations.map((citation) => {
            const evidence = chunks.find(
                (c) =>
                    c.documentId === citation?.documentId &&
                    c.chunkId === citation.chunkId &&
                    c.location === citation.location
            )
            if (
                !evidence ||
                typeof citation.quote !== 'string' ||
                citation.quote.length < 4 ||
                citation.quote.length > 600 ||
                !evidence.text.includes(citation.quote)
            )
                fail()
            return {
                documentId: evidence.documentId,
                chunkId: evidence.chunkId,
                location: evidence.location,
                quote: citation.quote
            }
        })
        return { text: answer.text, citations }
    })
    result.content = value.content
    return result
}

export function knowledgeLocation(chunk) {
    return chunk.format === 'pdf'
        ? `第 ${chunk.location} 页`
        : chunk.format === 'epub'
          ? `第 ${chunk.location} 节`
          : `第 ${chunk.location} 段`
}
