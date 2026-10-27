"""Explicit local import; never fetches URLs or extracts EPUB files to disk."""
import io
import logging
import posixpath
import zipfile
from urllib.parse import unquote, urlsplit
from xml.etree import ElementTree as ET

from .errors import AgentError
from .knowledge_schema import DocumentInput

MAX_FILE_BYTES = 16 * 1024 * 1024

# pypdf warnings may contain raw header bytes from a malformed user document.
logging.getLogger('pypdf').setLevel(logging.CRITICAL + 1)


def xml(data):
    if b"<!DOCTYPE" in data.upper() or b"<!ENTITY" in data.upper():
        raise ValueError("DTD not supported")
    return ET.fromstring(data)


def visible_text(element):
    tag = element.tag.rsplit('}', 1)[-1].lower()
    if tag in ('head', 'script', 'style', 'template') or 'hidden' in element.attrib or element.attrib.get('aria-hidden') == 'true':
        return ''
    pieces = [element.text or '']
    for child in element:
        pieces.extend((visible_text(child), child.tail or ''))
    return ' '.join(pieces)


def epub_sections(data):
    with zipfile.ZipFile(io.BytesIO(data)) as book:
        entries = book.infolist()
        if len(entries) > 3000 or sum(i.file_size for i in entries) > 32 * 1024 * 1024:
            raise ValueError("Archive too large")
        if len({i.filename for i in entries}) != len(entries):
            raise ValueError("Duplicate archive member")

        def read(name):
            item = book.getinfo(name)
            if item.file_size > 4 * 1024 * 1024 or item.flag_bits & 1:
                raise ValueError("Unsupported member")
            return book.read(item)

        container = xml(read('META-INF/container.xml'))
        rootfile = container.find('.//{*}rootfile')
        opf_path = rootfile.attrib['full-path']
        opf = xml(read(opf_path))
        manifest = {i.attrib['id']: i for i in opf.findall('.//{*}manifest/{*}item')}
        spine = opf.findall('.//{*}spine/{*}itemref')
        if not spine or len(spine) > 1000:
            raise ValueError("Invalid spine")
        sections = []
        for index, ref in enumerate(spine, 1):
            item = manifest[ref.attrib['idref']]
            href = urlsplit(item.attrib['href'])
            if href.scheme or href.netloc or href.query:
                raise ValueError("External member unsupported")
            path = posixpath.normpath(posixpath.join(posixpath.dirname(opf_path), unquote(href.path)))
            if path.startswith(('/', '../')) or '\\' in path:
                raise ValueError("Invalid member path")
            if item.attrib.get('media-type') not in ('application/xhtml+xml', 'text/html'):
                sections.append({'location': index, 'text': ''})
                continue
            root = xml(read(path))
            sections.append({'location': index, 'text': visible_text(root)})
        return sections


def extract_document(data, title, file_format):
    if not isinstance(data, bytes) or not 1 <= len(data) <= MAX_FILE_BYTES:
        raise AgentError("导入文件为空或超过16 MiB", "invalid_document")
    try:
        if file_format == 'pdf':
            # Lazy import: listing/searching text corpora does not load a PDF parser.
            from pypdf import PdfReader
            reader = PdfReader(io.BytesIO(data))
            if reader.is_encrypted or len(reader.pages) > 1000:
                raise ValueError("Encrypted or too many pages")
            sections, total = [], 0
            for index, page in enumerate(reader.pages, 1):
                contents = page.get_contents()
                if contents is not None and len(contents.get_data()) > 4 * 1024 * 1024:
                    raise ValueError("Page too complex")
                text = page.extract_text() or ''
                total += len(text)
                if len(text) > 100000 or total > 2_000_000:
                    raise ValueError("Text too large")
                sections.append({'location': index, 'text': text})
        elif file_format == 'epub':
            sections = epub_sections(data)
        elif file_format == 'text':
            text = data.decode('utf-8-sig')
            sections = [{'location': i // 50000 + 1, 'text': text[i:i + 50000]} for i in range(0, len(text), 50000)]
        else:
            raise ValueError("Unsupported format")
        return DocumentInput.model_validate({'title': title, 'format': file_format, 'sections': sections})
    except Exception:
        # Parser exception strings may contain local paths or document content.
        raise AgentError("无法提取文档，请提供未加密PDF、标准EPUB或UTF-8文本；暂不支持OCR", "invalid_document") from None
