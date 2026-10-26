"""Transport-independent, bounded knowledge contracts."""
from typing import Annotated, Literal

from pydantic import Field, model_validator

from .reading_schema import Quote, StrictOutput, Text


Identifier = Annotated[str, Field(pattern=r"^[a-f0-9]{32}$")]


class Section(StrictOutput):
    location: Annotated[int, Field(ge=1, le=10000)]
    text: Annotated[str, Field(max_length=100000)]


class DocumentInput(StrictOutput):
    title: Annotated[str, Field(min_length=1, max_length=200)]
    format: Literal["pdf", "epub", "text"]
    sections: Annotated[list[Section], Field(min_length=1, max_length=1000)]

    @model_validator(mode="after")
    def bounded_document(self):
        if not self.title.strip() or len({s.location for s in self.sections}) != len(self.sections):
            raise ValueError("Title or section locations invalid")
        if sum(len(s.text) for s in self.sections) > 2_000_000:
            raise ValueError("Document too large")
        return self


class SearchInput(StrictOutput):
    question: Annotated[str, Field(min_length=1, max_length=2000)]
    documentIds: Annotated[list[Identifier], Field(max_length=50)] = Field(default_factory=list)
    limit: Annotated[int, Field(ge=1, le=6)] = 6
    mode: Literal["keyword", "hybrid"] = "keyword"

    @model_validator(mode="after")
    def nonblank(self):
        if not self.question.strip():
            raise ValueError("Question required")
        return self


class DocumentCitation(StrictOutput):
    documentId: Identifier
    chunkId: Identifier
    location: Annotated[int, Field(ge=1, le=10000)]
    quote: Quote


class KnowledgeAnswer(StrictOutput):
    text: Text
    citations: Annotated[list[DocumentCitation], Field(min_length=1, max_length=3)]


class KnowledgeOutput(StrictOutput):
    answers: Annotated[list[KnowledgeAnswer], Field(max_length=5)]


class KnowledgeChunk(StrictOutput):
    documentId: Identifier
    chunkId: Identifier
    location: Annotated[int, Field(ge=1, le=10000)]
    title: Annotated[str, Field(min_length=1, max_length=200)]
    format: Literal['pdf', 'epub', 'text']
    text: Annotated[str, Field(min_length=1, max_length=1200)]


class DocumentMetadata(StrictOutput):
    documentId: Identifier
    title: Annotated[str, Field(min_length=1, max_length=200)]
    format: Literal['pdf', 'epub', 'text']
    characters: Annotated[int, Field(ge=1)]
    chunks: Annotated[int, Field(ge=1, le=3000)]
    profile: Annotated[str, Field(max_length=64)]
    dimension: Annotated[int, Field(ge=0, le=4096)]
    created: str


class DocumentList(StrictOutput):
    documents: Annotated[list[DocumentMetadata], Field(max_length=200)]


class IndexResult(DocumentMetadata):
    inserted: bool


class SearchResult(StrictOutput):
    chunks: Annotated[list[KnowledgeChunk], Field(max_length=6)]
    mode: Literal['keyword', 'hybrid']
    scannedChunks: Annotated[int, Field(ge=0, le=10000)]
    vectorChunks: Annotated[int, Field(ge=0, le=10000)]
    partial: bool
    fusion: Literal['bm25', 'rrf-k60']
