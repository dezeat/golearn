# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
From the next release onward, entries are generated automatically by
[release-please](https://github.com/googleapis/release-please) from
Conventional Commit history.

## [0.2.0](https://github.com/dezeat/golearn/compare/v0.1.2...v0.2.0) (2026-08-18)


### Features

* adopt GitHub-native operating model (CLAUDE.md, decision log, skills) ([#50](https://github.com/dezeat/golearn/issues/50)) ([3016f70](https://github.com/dezeat/golearn/commit/3016f70dbd4d54e752b3faba1e09fae0654d56a2))


### Documentation

* add the Forge design spec ([#109](https://github.com/dezeat/golearn/issues/109)) ([1889ae0](https://github.com/dezeat/golearn/commit/1889ae0cb085af44d978d2e6bdad8f0b9503a192))
* adopt the portfolio operating model (labels, boards, wayfinder) ([#111](https://github.com/dezeat/golearn/issues/111)) ([8459332](https://github.com/dezeat/golearn/commit/84593321031cbd4e80dbdac6c603c01ba483b4f7))
* record Forge authoring-boundary decisions (D-015–D-017) ([#107](https://github.com/dezeat/golearn/issues/107)) ([7c66aca](https://github.com/dezeat/golearn/commit/7c66aca926ef0e2a382877996f8180ff11b776b3))
* record pre-1.0.0 hardening posture decisions (D-011–D-014) ([#93](https://github.com/dezeat/golearn/issues/93)) ([7483d35](https://github.com/dezeat/golearn/commit/7483d352b1f6d62f0295098b280505b60e8227da))
* render working hero GIF and add ASCII logo to README ([#64](https://github.com/dezeat/golearn/issues/64)) ([6a801af](https://github.com/dezeat/golearn/commit/6a801afced7d0e4104ed24c533cddbc3d6cd96c7))
* restructure README with marketing skeleton and architecture diagram ([#62](https://github.com/dezeat/golearn/issues/62)) ([b042d51](https://github.com/dezeat/golearn/commit/b042d5146f18bd80f6d99e1146cbc63095f2dcf4))


### Build System

* **deps:** bump actions/dependency-review-action from 4.9.0 to 5.0.0 ([#69](https://github.com/dezeat/golearn/issues/69)) ([77ed5fd](https://github.com/dezeat/golearn/commit/77ed5fdf5c268bb054687e4c7da2710b64ee60c5))
* **deps:** bump modernc.org/sqlite from 1.53.0 to 1.54.0 ([#70](https://github.com/dezeat/golearn/issues/70)) ([db65400](https://github.com/dezeat/golearn/commit/db65400c2273c1108f0675cd42924a1023331f4d))

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
