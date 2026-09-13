# Contributing to SOLV

Thank you for your interest in contributing to SOLV.

## Before submitting a pull request:
- Run `go test -race ./...` in the `agent/` directory and ensure all tests pass.
- Run `pytest` in the `server/` directory.
- Follow existing Go and Python conventions (`gofmt`, standard naming).
- Strictly adhere to the zero-emojis rule in code, comments, documentation, and UI strings (per D3).
- Add tests for any new functionality or regression fixes.
- Update documentation if user-facing behavior changes.

## What we are looking for:
- Bug fixes accompanied by unit tests.
- Additional language translations for the i18n catalog.
- Documentation clarifications and architectural improvements.
- Platform-specific bug reports and fixes (macOS darwin, linux/arm64).

## What we are not accepting at this time:
- New features without prior discussion (please open an issue first).
- Windows native host agent support (use WSL2).
- Breaking changes to the cryptographic vault format (`~/.solv/vault.enc`).
