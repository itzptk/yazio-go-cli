# Security Policy

## Unofficial API caveat

This project talks to YAZIO's unofficial/private API. Endpoints, payloads, authentication behavior, and availability may change without notice.

## Sensitive data

Do not commit real YAZIO credentials, tokens, local config files, or `.env` files. The CLI stores account tokens in the OS config directory after `yazio auth login`; those files should remain local only.

## Reporting security issues

Please open a private report or contact the repository owner instead of filing a public issue for credential leaks or security-sensitive behavior.
