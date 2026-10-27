import JSZip from 'jszip'

// An isolated, shareable EPUB fixture, unrelated to real books or user records.
export async function readingEpub() {
    const zip = new JSZip()
    zip.file('mimetype', 'application/epub+zip', { compression: 'STORE' })
    zip.file(
        'META-INF/container.xml',
        '<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OPS/book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>'
    )
    zip.file(
        'OPS/book.opf',
        '<?xml version="1.0"?><package version="3.0" unique-identifier="id" xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">synthetic-reading-fixture</dc:identifier><dc:title>Synthetic Reading Fixture</dc:title><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-25T00:00:00Z</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="one" href="one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="two.xhtml" media-type="application/xhtml+xml"/><item id="three" href="three.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="one"/><itemref idref="two"/><itemref idref="three"/></spine></package>'
    )
    zip.file(
        'OPS/nav.xhtml',
        '<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="one.xhtml">Vectors</a></li><li><a href="two.xhtml">Derivatives</a></li><li><a href="three.xhtml">Test scope</a></li></ol></nav></body></html>'
    )
    const texts = [
        'A vector has magnitude and direction.',
        'The derivative measures the rate of change.',
        'This is an isolated synthetic reading test.'
    ]
    for (const [index, name] of ['one', 'two', 'three'].entries())
        zip.file(
            `OPS/${name}.xhtml`,
            `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Section ${index + 1}</title></head><body><h1>Section ${index + 1}</h1><p>${texts[index]}</p></body></html>`
        )
    return zip.generateAsync({ type: 'uint8array', compression: 'DEFLATE' })
}
