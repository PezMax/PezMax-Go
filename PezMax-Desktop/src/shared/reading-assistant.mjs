const normalizeText = (text) => text.replace(/\s+/g, ' ').trim()
export const readingModes = Object.freeze({
    qa: { label: '阅读问答', prompt: '' },
    summary: {
        label: '内容摘要',
        prompt: '请概括所选文字的核心观点与结论，每一项附原文依据。仅总结提供的文字，不宣称覆盖整本书。'
    },
    outline: {
        label: '内容提纲',
        prompt: '请将所选文字整理成最多5项有顺序的内容提纲，每项说明主题及要点并附原文依据。不要补充原文没有的章节。'
    }
})
export const readingLocationKey = (input) => (input.format === 'epub' ? 'chapter' : 'page')
export const readingItems = (input) => (input.format === 'epub' ? input.chapters : input.pages)
export function normalizeReadingInput(input) {
    if (
        !input ||
        !['pdf', 'epub'].includes(input.format ?? 'pdf') ||
        typeof (input.mode ?? 'qa') !== 'string' ||
        !Object.hasOwn(readingModes, input.mode ?? 'qa')
    )
        throw new Error('请求的阅读格式或模式无效')
    const format = input.format ?? 'pdf',
        mode = input.mode ?? 'qa',
        key = readingLocationKey(input)
    const question = mode === 'qa' ? input.question : readingModes[mode].prompt
    if (typeof question !== 'string' || !question.trim() || question.trim().length > 2000)
        throw new Error('请输入1到2000字的阅读问题')
    const items = readingItems(input)
    if (!Array.isArray(items) || !items.length || items.length > (format === 'epub' ? 5 : 10))
        throw new Error(format === 'epub' ? '请选择1到5节EPUB文字' : '请选择1到10页PDF文字')
    let total = 0
    const seen = new Set()
    const pages = items
        .map((item) => {
            if (
                !item ||
                !Number.isInteger(item[key]) ||
                item[key] < 1 ||
                item[key] > 10000 ||
                seen.has(item[key]) ||
                typeof item.text !== 'string' ||
                item.text.length > (format === 'epub' ? 40000 : 20000)
            )
                throw new Error('请求中的阅读位置或文字超出限制')
            seen.add(item[key])
            total += item.text.length
            return { [key]: item[key], text: normalizeText(item.text) }
        })
        .sort((a, b) => a[key] - b[key])
    if (total > 80000) throw new Error('请求中的阅读文字过长，请缩小所选范围')
    return {
        question: question.trim(),
        [format === 'epub' ? 'chapters' : 'pages']: pages,
        ...(format === 'epub' ? { format } : {}),
        ...(mode !== 'qa' ? { mode } : {})
    }
}

export function normalizeReadingResult(result, input) {
    if (!result || !Array.isArray(result.answers))
        throw new Error('模型返回的阅读答案格式无效，请重试')
    const key = readingLocationKey(input),
        contextKey = key === 'chapter' ? 'contextChapters' : 'contextPages'
    const pages = new Map(readingItems(input).map((item) => [item[key], normalizeText(item.text)]))
    const answers = result.answers
        .slice(0, 5)
        .filter((answer) => {
            if (
                !answer ||
                typeof answer.text !== 'string' ||
                !answer.text.trim() ||
                answer.text.length > 1600 ||
                !Array.isArray(answer.citations) ||
                !answer.citations.length ||
                answer.citations.length > 3
            )
                return false
            return answer.citations.every(
                (cite) =>
                    cite &&
                    Number.isInteger(cite[key]) &&
                    pages.has(cite[key]) &&
                    typeof cite.quote === 'string' &&
                    normalizeText(cite.quote).length >= 4 &&
                    normalizeText(cite.quote).length <= 600 &&
                    pages.get(cite[key]).includes(normalizeText(cite.quote))
            )
        })
        .map((answer) => ({
            text: answer.text.trim(),
            citations: answer.citations.map((cite) => ({
                [key]: cite[key],
                quote: normalizeText(cite.quote)
            }))
        }))
    return {
        ...(result.diagnostics &&
        result.diagnostics.task === 'reading' &&
        Number.isInteger(result.diagnostics.modelCalls) &&
        result.diagnostics.modelCalls >= 0 &&
        result.diagnostics.modelCalls <= 2 &&
        Number.isInteger(result.diagnostics.durationMs) &&
        result.diagnostics.durationMs >= 0 &&
        result.diagnostics.durationMs <= 180000
            ? {
                  diagnostics: {
                      modelCalls: result.diagnostics.modelCalls,
                      durationMs: result.diagnostics.durationMs
                  }
              }
            : {}),
        content:
            answers.map((answer) => answer.text).join('\n\n') ||
            '所选内容中没有足够的已核验引用支持回答，请调整问题或所选范围。',
        answers,
        [contextKey]: [
            ...new Set(
                (Array.isArray(result[contextKey]) ? result[contextKey] : []).filter(
                    (page) => Number.isInteger(page) && pages.has(page)
                )
            )
        ].sort((a, b) => a - b),
        partial: result.partial === true,
        messages: [],
        resources: [],
        reports: []
    }
}

export function pdfPageUrl(source, page) {
    if (!Number.isInteger(page) || page < 1 || page > 10000) throw new Error('请求的PDF页码无效')
    if (
        typeof source !== 'string' ||
        !source ||
        !['http:', 'https:', 'blob:'].includes(new URL(source, 'https://reader.invalid').protocol)
    )
        throw new Error('请求的PDF地址无效')
    return `${source.split('#')[0]}#page=${page}`
}
