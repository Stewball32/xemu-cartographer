# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `/api/version` endpoint returning the git-tag-derived version, short commit, and build date.
- Container images now tagged with both `:VERSION` and `:latest`.
- ADR-0001 documents the single-source-of-truth version pipeline (git tag → ldflags → `internal/version` → `/api/version` → frontend `PUBLIC_APP_VERSION`).

### Changed

### Deprecated

### Removed

### Fixed

### Security
