"""Deployment identity and endpoint policy; never controlled by model input."""
import hmac
import json
import os
import re
from dataclasses import dataclass, field
from pathlib import Path

from .contracts import ProviderConfig, base_url
from .errors import AgentError


@dataclass(frozen=True)
class ServiceSettings:
    database: Path
    tokens: dict[str, str] = field(repr=False)
    provider_urls: tuple[str, ...] = ()
    catalog_url: str | None = None
    concurrency: int = 4
    body_limit: int = 8 * 1024 * 1024
    timeout: float = 180

    def __post_init__(self):
        if (not isinstance(self.tokens, dict) or not self.tokens or not 1 <= self.concurrency <= 16
                or not 1 <= self.body_limit <= 8 * 1024 * 1024 or self.timeout <= 0):
            raise ValueError('Invalid service settings')
        if len(set(self.tokens.values())) != len(self.tokens):
            raise ValueError('Service identities must have distinct tokens')
        for owner, token in self.tokens.items():
            if not re.fullmatch(r'[A-Za-z0-9_-]{1,64}', owner) or not re.fullmatch(r'[A-Za-z0-9_-]{32,128}', token):
                raise ValueError('Use stable owner identifiers and random URL-safe tokens of at least 32 characters')
        for url in self.provider_urls:
            if base_url(url, provider=True) != url:
                raise ValueError('Provider allowlist must contain normalized base URLs')
        if self.catalog_url and base_url(self.catalog_url) != self.catalog_url:
            raise ValueError('Catalog base URL must be normalized')

    def authenticate(self, header):
        scheme, _, token = header.partition(' ')
        if scheme.lower() != 'bearer' or not re.fullmatch(r'[A-Za-z0-9_-]{32,128}', token):
            return None
        for owner, expected in self.tokens.items():
            if hmac.compare_digest(token, expected):
                return owner
        return None

    def provider(self, value):
        config = ProviderConfig.parse(value)
        if config.base_url not in self.provider_urls:
            raise AgentError('该模型接口未被部署者加入白名单', 'provider_not_allowed')
        return config

    @classmethod
    def from_env(cls):
        try:
            return cls(database=Path(os.environ.get('PEZMAX_AGENT_DB', 'data/knowledge.sqlite3')).resolve(),
                tokens=json.loads(os.environ['PEZMAX_AGENT_ACCESS_TOKENS']),
                provider_urls=tuple(json.loads(os.environ.get('PEZMAX_AGENT_PROVIDER_URLS', '[]'))),
                catalog_url=os.environ.get('PEZMAX_AGENT_CATALOG_URL') or None)
        except (ValueError, TypeError, KeyError, AgentError):
            raise ValueError('请配置有效的 PEZMAX_AGENT_ACCESS_TOKENS、PEZMAX_AGENT_DB 和模型接口白名单') from None
