const normalizeText = (text) => text.replace(/\s+/g, ' ').trim()
export function normalizeReadingInput(input) {
    if (
        !input ||
        typeof input.question !== 'string' ||
        !input.question.trim() ||
        input.question.trim().length > 2000
    )
        throw new Error('请输入1到2000字的阅读问题')
    if (!Array.isArray(input.pages) || !input.pages.length || input.pages.length > 10)
        throw new Error('请选择1到10页PDF文字')
    let total = 0
    const seen = new Set()
    const pages = input.pages
        .map((item) => {
            if (
                !item ||
                !Number.isInteger(item.page) ||
                item.page < 1 ||
                item.page > 10000 ||
                seen.has(item.page) ||
                typeof item.text !== 'string' ||
                item.text.length > 20000
            )
                throw new Error('请求中的PDF页码或文字超出限制')
            seen.add(item.page)
            total += item.text.length
            return { page: item.page, text: normalizeText(item.text) }
        })
        .sort((a, b) => a.page - b.page)
    if (total > 80000) throw new Error('请求中的PDF文字过长，请缩小页码范围')
    return { question: input.question.trim(), pages }
}

export function normalizeReadingResult(result, input) {
    if (!result || !Array.isArray(result.answers))
        throw new Error('模型返回的阅读答案格式无效，请重试')
    const pages = new Map(input.pages.map((item) => [item.page, normalizeText(item.text)]))
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
                    Number.isInteger(cite.page) &&
                    pages.has(cite.page) &&
                    typeof cite.quote === 'string' &&
                    normalizeText(cite.quote).length >= 4 &&
                    normalizeText(cite.quote).length <= 600 &&
                    pages.get(cite.page).includes(normalizeText(cite.quote))
            )
        })
        .map((answer) => ({
            text: answer.text.trim(),
            citations: answer.citations.map((cite) => ({
                page: cite.page,
                quote: normalizeText(cite.quote)
            }))
        }))
    return {
        content:
            answers.map((answer) => answer.text).join('\n\n') ||
            '所选页面中没有足够的已核验引用支持回答，请调整问题或页码范围。',
        answers,
        contextPages: [
            ...new Set(
                (Array.isArray(result.contextPages) ? result.contextPages : []).filter(
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
