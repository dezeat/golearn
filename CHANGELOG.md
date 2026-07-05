# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
From the next release onward, entries are generated automatically by
[release-please](https://github.com/googleapis/release-please) from
Conventional Commit history.

## [0.1.2](https://github.com/dezeat/golearn/compare/v0.1.1...v0.1.2) (2026-07-03)

### Build System

* bump modernc.org/sqlite from 1.50.1 to 1.53.0
* bump actions/checkout from 6 to 7

## [0.1.1](https://github.com/dezeat/golearn/compare/v0.1.0...v0.1.1) (2026-05-24)

### Build System

* bump Go toolchain to 1.25.10 to patch stdlib vulnerabilities
* bump modernc.org/sqlite from 1.46.1 to 1.50.1
* bump goreleaser/goreleaser-action from 6 to 7

### Continuous Integration

* remove CodeQL from CI workflow in favour of GitHub default setup

## [0.1.0](https://github.com/dezeat/golearn/releases/tag/v0.1.0) (2026-02-25)

### Features

* initial release — local-first, offline, CGo-free terminal engine for
  practising multiple-choice questions: YAML/JSON pack import, SQLite storage,
  Bubble Tea TUI, per-user stats, deterministic export
* add GoReleaser config and tag-triggered release workflow
