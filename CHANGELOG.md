# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-24

### Added
- **Global Branding**: Standardized project name to **Antigravity-Go** and command to `ago`.
- **Industrial Backend**: Introduced generic RPC wrappers and consistent error handling in `internal/corecap`.
- **Frontend Modernization**: Migrated workspace to **Bun 1.3.9** and **Vite 8.0.2**.
- **Commander Paradigm 2.0**: Enforced strict 0px border-radius and minimalist UI aesthetics across all components.
- **Enhanced Setup**: Added `Shift+Tab` support for returning to previous steps in the TUI setup wizard.
- **Self-Healing Infrastructure**: Implemented `ago doctor` with environment auto-repair and safe-start capabilities.
- **MCP Ecosystem**: Full Model Context Protocol (MCP) integration with dynamic mounting and config overriding.
- **Data Plane Convergence**: Shared `AsyncContent` components for Trajectory, Memory, and Approval modals.
- **Robust Observability**: Unified task ledger with success rate statistics and failure backtracking.

### Changed
- Refactored `internal/server` to use centralized HTTP helpers for JSON and method validation.
- Improved TUI stability by adding layout boundary checks to prevent panics on small windows.
- Optimized frontend build pipeline reducing bundle size and increasing build speed by 70%.
- Updated all technical documentation and README to reflect the industrialized architecture.

### Fixed
- Fixed a panic in the TUI when terminal height was less than 15 lines.
- Resolved race conditions in concurrent agent worker execution.
- Fixed inconsistent padding and "soft" UI artifacts (shadows/blurs) to align with Commander Paradigm.

### Removed
- Removed legacy `agy` references and deprecated `cmd/debug` entry point.
- Purged all `.bak` and temporary build artifacts from the repository.
- Eliminated redundant `node_modules` and `package-lock.json` in favor of Bun lockfile.

---

## [0.0.x] - Historical Milestones (Phase 1 - 5)

- **Phase 5**: Implemented Containerized Deployment and GitHub Actions integration.
- **Phase 4**: Introduced "Maker-Checker" workflow with automated code review (ReviewerAgent).
- **Phase 3**: Added support for Visual Self-Testing and project metadata extraction.
- **Phase 2**: Core Capability (Trajectory/Memory/Workspace) RPC orchestration.
- **Phase 1**: Initial Host-Kernel handshake and multi-channel LLM provider factory.
