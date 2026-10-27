// Isolated synthetic PDF: three physical pages with no personal or real records.
export function readingPdf() {
    const texts = [
        'A vector has magnitude and direction.',
        'The derivative measures the rate of change.',
        'This is an isolated synthetic reading test.'
    ]
    const objects = [
        '<< /Type /Catalog /Pages 2 0 R >>',
        '<< /Type /Pages /Kids [4 0 R 6 0 R 8 0 R] /Count 3 >>',
        '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>'
    ]
    for (const [index, text] of texts.entries()) {
        const stream = `BT /F1 18 Tf 50 740 Td (${text}) Tj ET\n`
        objects.push(
            `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents ${5 + index * 2} 0 R >>`
        )
        objects.push(`<< /Length ${stream.length} >>\nstream\n${stream}endstream`)
    }
    let pdf = '%PDF-1.4\n',
        offsets = [0]
    objects.forEach((object, index) => {
        offsets.push(pdf.length)
        pdf += `${index + 1} 0 obj\n${object}\nendobj\n`
    })
    const xref = pdf.length
    pdf += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n`
    for (const offset of offsets.slice(1)) pdf += `${String(offset).padStart(10, '0')} 00000 n \n`
    pdf += `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`
    return new TextEncoder().encode(pdf)
}
