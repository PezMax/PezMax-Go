"""Provider-independent strict output schemas; no coercion of citation indexes."""
from typing import Annotated, Any

from pydantic import BaseModel, BeforeValidator, ConfigDict, Field, StringConstraints


class StrictOutput(BaseModel):
    model_config = ConfigDict(strict=True, extra="forbid")


Text = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=1600)]
Quote = Annotated[str, BeforeValidator(lambda value: " ".join(value.split()) if isinstance(value, str) else value), Field(min_length=4, max_length=600)]
Location = Annotated[int, Field(ge=1, le=10000)]


class PageCitation(StrictOutput):
    page: Location
    quote: Quote


class ChapterCitation(StrictOutput):
    chapter: Location
    quote: Quote


class PageAnswer(StrictOutput):
    text: Text
    citations: Annotated[list[PageCitation], Field(min_length=1, max_length=3)]


class ChapterAnswer(StrictOutput):
    text: Text
    citations: Annotated[list[ChapterCitation], Field(min_length=1, max_length=3)]


class PageOutput(StrictOutput):
    answers: Annotated[list[PageAnswer], Field(max_length=5)]


class ChapterOutput(StrictOutput):
    answers: Annotated[list[ChapterAnswer], Field(max_length=5)]


class ReadingEnvelope(StrictOutput):
    # Validate entries independently so one bad answer cannot suppress valid ones.
    answers: Annotated[list[Any], Field(max_length=5)]
