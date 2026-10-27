"""Clearly synthetic software documents for desktop source/frozen verification."""
import sys
import zipfile
from pathlib import Path


def create(root):
    from pypdf import PdfWriter
    from pypdf.generic import DecodedStreamObject, DictionaryObject, NameObject

    root = Path(root)
    root.mkdir(parents=True, exist_ok=True)
    writer = PdfWriter()
    writer.add_blank_page(width=300, height=300)
    page = writer.add_blank_page(width=300, height=300)
    font = DictionaryObject({NameObject('/Type'): NameObject('/Font'), NameObject('/Subtype'): NameObject('/Type1'),
                             NameObject('/BaseFont'): NameObject('/Helvetica')})
    page[NameObject('/Resources')] = DictionaryObject({NameObject('/Font'): DictionaryObject({NameObject('/F1'): font})})
    contents = DecodedStreamObject()
    contents.set_data(b'BT /F1 12 Tf 20 260 Td (A queue uses first in first out ordering.) Tj ET')
    page[NameObject('/Contents')] = writer._add_object(contents)
    writer.write(root / 'synthetic-queue.pdf')
    with zipfile.ZipFile(root / 'synthetic-queue.epub', 'w') as book:
        book.writestr('mimetype', 'application/epub+zip')
        book.writestr('META-INF/container.xml', '<container><rootfiles><rootfile full-path="OPS/book.opf"/></rootfiles></container>')
        book.writestr('OPS/book.opf', '<package><manifest><item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>'
                      '<item id="queue" href="queue.xhtml" media-type="application/xhtml+xml"/></manifest>'
                      '<spine><itemref idref="cover"/><itemref idref="queue"/></spine></package>')
        book.writestr('OPS/cover.xhtml', '<html><head><title>Synthetic software fixture</title></head><body></body></html>')
        book.writestr('OPS/queue.xhtml', '<html><body><p>A queue uses first in first out ordering.</p></body></html>')
    (root / 'synthetic-queue.md').write_text('A queue uses first in first out ordering.\n队列使用先进先出顺序。', encoding='utf-8')


if __name__ == '__main__':
    create(sys.argv[1])
