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
    if (!input || !['list', 'import', 'delete', 'search', 'ask', 'read'].includes(input.action))
        fail()
    const mode = input.mode ?? 'keyword'
    if (!['keyword', 'hybrid'].includes(mode)) fail()
    const result = { action: input.action, mode }
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
    if (['delete', 'read'].includes(input.action)) result.documentId = id(input.documentId)
    if (input.action === 'read') result.chunkId = id(input.chunkId)
    return result
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
        text: value.text
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
        created: value.created
    }
}

export function normalizeKnowledgeResult(value, request) {
    if (!value || typeof value !== 'object') fail()
    if (value.cancelled === true && request.action === 'import') return { cancelled: true }
    if (request.action === 'list') {
        if (!Array.isArray(value.documents) || value.documents.length > 200) fail()
        return { documents: value.documents.map(document) }
    }
    if (request.action === 'import') {
        if (typeof value.document?.inserted !== 'boolean') fail()
        return { document: { ...document(value.document), inserted: value.document.inserted } }
    }
    if (request.action === 'delete') {
        if (typeof value.deleted !== 'boolean') fail()
        return { deleted: value.deleted }
    }
    if (request.action === 'read') {
        const result = chunk(value.chunk)
        if (result.documentId !== request.documentId || result.chunkId !== request.chunkId) fail()
        return { chunk: result }
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
    if (
        request.query.documentIds.length &&
        chunks.some((c) => !request.query.documentIds.includes(c.documentId))
    )
        fail()
    const result = {
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
