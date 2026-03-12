# Plan: VitePress Documentation Site with Simpsons Theme

## Introduction

The current README.md has grown too large and serves as the only documentation source. This plan migrates all documentation into a VitePress-powered GitHub Pages site with a Simpsons-inspired visual theme (yellow/blue color scheme, playful tone). The README.md will be slimmed down to a short intro pointing to the docs site. Deployment happens automatically via GitHub Actions on release tags.

## Goals

- Restructure all existing documentation into a multi-page VitePress site in `docs/`
- Apply a Simpsons-inspired theme (Simpsons yellow `#FDD835`, blue `#2196F3`, playful typography)
- Deploy to GitHub Pages automatically when a release tag is pushed
- Slim the README.md to a short elevator pitch + link to the docs site
- Ensure every Maggus feature is documented clearly enough for a first-time user

## User Stories

### TASK-001: Scaffold VitePress project in docs/
**Description:** As a developer, I want the VitePress project initialized in `docs/` so that we have a working local dev setup.

**Acceptance Criteria:**
- [x] `docs/` contains `package.json` with vitepress as a dev dependency
- [x] `docs/.vitepress/config.ts` exists with basic site config (title: "Maggus", description)
- [x] `docs/index.md` exists with a placeholder homepage
- [x] Running `npm install && npm run docs:dev` inside `docs/` starts the dev server without errors
- [x] `docs/.vitepress/dist` and `docs/node_modules` and `docs/.vitepress/cache` are added to the repo `.gitignore`
- [x] The existing `docs/avatar.png` is preserved and moved/referenced as the site logo

### TASK-002: Apply Simpsons-inspired theme
**Description:** As a visitor, I want the docs site to have a Simpsons-inspired visual identity so that it feels fun and matches Maggus's personality.

**Acceptance Criteria:**
- [x] VitePress theme config uses Simpsons yellow (`#FDD835`) as the primary brand color
- [x] Accent/secondary color is Simpsons sky blue (`#2196F3`)
- [x] The homepage hero section has a playful tagline (e.g. "Your best and worst co-worker at the same time")
- [x] The Maggus avatar (`avatar.png`) is displayed prominently on the homepage as the hero image
- [x] Homepage has a features grid with 3-4 key selling points (plan-driven, autonomous, commits for you, etc.)
- [x] Custom CSS overrides are in `docs/.vitepress/theme/custom.css` or equivalent
- [x] The color scheme works in both light and dark mode
- [x] ⚠️ BLOCKED: Verify in browser using dev-browser skill — dev-browser skill is not available in this environment

### TASK-003: Create "Getting Started" guide
**Description:** As a new user, I want a getting started page so that I can install and run Maggus for the first time.

**Acceptance Criteria:**
- [ ] Page at `docs/guide/getting-started.md`
- [ ] Covers prerequisites (Go 1.22+, Claude Code CLI on PATH)
- [ ] Covers installation: build from source and pre-built binaries (link to GitHub Releases)
- [ ] Covers first run: creating a `.maggus/` directory, writing a minimal plan file, running `maggus work`
- [ ] Includes a minimal end-to-end example a user can copy-paste and try
- [ ] Navigation sidebar shows this page under a "Guide" section

### TASK-004: Create "Writing Plans" documentation page
**Description:** As a user, I want to understand how to write implementation plans so that Maggus can process my tasks.

**Acceptance Criteria:**
- [ ] Page at `docs/guide/writing-plans.md`
- [ ] Explains the plan file format: location (`.maggus/plan_*.md`), task heading format (`### TASK-NNN: Title`), description, acceptance criteria checkboxes
- [ ] Explains what makes a task "complete" (all criteria checked)
- [ ] Explains blocked tasks: the `BLOCKED:` prefix convention, how Maggus skips them
- [ ] Explains completed plans: automatic rename to `plan_N_completed.md`
- [ ] Includes a full example plan file
- [ ] Mentions the `maggus-plan` skill as an alternative to writing plans manually
- [ ] Navigation sidebar shows this page under "Guide"

### TASK-005: Create "CLI Commands" reference page
**Description:** As a user, I want a reference of all CLI commands so that I know what flags and options are available.

**Acceptance Criteria:**
- [ ] Page at `docs/reference/commands.md`
- [ ] Documents `maggus work` with all flags (`--count`, `--model`, `--no-bootstrap`) and examples
- [ ] Documents `maggus list` with all flags (`--count`, `--all`, `--plain`) and examples
- [ ] Documents `maggus status` with all flags (`--all`, `--plain`) and examples
- [ ] Documents `maggus blocked` with usage description and examples
- [ ] Each command section includes example output where helpful
- [ ] Navigation sidebar shows this page under a "Reference" section

### TASK-006: Create "Configuration" reference page
**Description:** As a user, I want to understand all configuration options so that I can customize Maggus for my project.

**Acceptance Criteria:**
- [ ] Page at `docs/reference/configuration.md`
- [ ] Documents `.maggus/config.yml` with all fields (`model`, `include`)
- [ ] Includes the model alias table (sonnet, opus, haiku → full IDs)
- [ ] Explains how `--model` CLI flag overrides config
- [ ] Explains the `include` paths for additional context files
- [ ] Documents bootstrap context files (CLAUDE.md, AGENTS.md, etc.) and the `--no-bootstrap` flag
- [ ] Navigation sidebar shows this page under "Reference"

### TASK-007: Create "Concepts" page covering run logs, memory, and TUI
**Description:** As a user, I want to understand Maggus's runtime behavior so that I know what to expect when it runs.

**Acceptance Criteria:**
- [ ] Page at `docs/guide/concepts.md`
- [ ] Explains the work loop lifecycle (parse → find task → branch → prompt → run → commit → repeat)
- [ ] Documents run logs: `.maggus/runs/<RUN_ID>/`, `run.md`, `iteration-NN.md`
- [ ] Documents project memory: `.maggus/MEMORY.md`, its purpose, that it's gitignored
- [ ] Documents the TUI: header, progress bar, task info, status section, recent commits
- [ ] Documents the startup safety pause (3-second window to abort)
- [ ] Documents Ctrl+C behavior (graceful stop, double Ctrl+C force-kill)
- [ ] Documents git branch behavior (auto-creates feature branch from protected branches)
- [ ] Navigation sidebar shows this page under "Guide"

### TASK-008: Configure sidebar navigation and header nav
**Description:** As a visitor, I want clear navigation so that I can find any documentation page quickly.

**Acceptance Criteria:**
- [ ] Top navigation bar has: "Guide" (link to getting-started), "Reference" (link to commands), GitHub link (icon)
- [ ] Sidebar for `/guide/` pages shows: Getting Started, Writing Plans, Concepts
- [ ] Sidebar for `/reference/` pages shows: CLI Commands, Configuration
- [ ] All pages are reachable from the sidebar and navigation
- [ ] Footer or nav includes a link to the GitHub repository
- [ ] Verify in browser using dev-browser skill

### TASK-009: Add GitHub Actions workflow for deployment on release
**Description:** As a maintainer, I want the docs site to deploy automatically when I publish a release so that docs stay up to date.

**Acceptance Criteria:**
- [ ] Workflow file at `.github/workflows/docs.yml`
- [ ] Triggers on push of tags matching `v*` or `[0-9]*` (release tags)
- [ ] Workflow installs Node.js, runs `npm ci` and `npm run docs:build` in `docs/`
- [ ] Deploys the built `docs/.vitepress/dist` to GitHub Pages using `actions/deploy-pages`
- [ ] Uses `actions/configure-pages` and `actions/upload-pages-artifact` as required
- [ ] Workflow sets appropriate permissions (`pages: write`, `id-token: write`)
- [ ] VitePress `base` config is set correctly for the GitHub Pages URL (e.g., `/maggus/`)

### TASK-010: Slim down README.md
**Description:** As a visitor on GitHub, I want a concise README that quickly explains what Maggus is and links to the full docs.

**Acceptance Criteria:**
- [ ] README.md contains: Maggus logo/avatar, one-paragraph description, quick install snippet, link to the docs site, link to the Skills Marketplace
- [ ] README.md is no longer than ~40 lines
- [ ] All detailed documentation (usage, config, blocked tasks, run logs, etc.) is removed from README.md
- [ ] The docs site URL is prominently displayed (e.g., "Read the full documentation at ...")
- [ ] The roadmap section is preserved in the README or moved to the docs site

## Functional Requirements

- FR-1: The VitePress site must build without errors using `npm run docs:build`
- FR-2: All content from the current README.md must exist somewhere in the docs site — nothing is lost
- FR-3: The Simpsons color theme must work in both light and dark mode without accessibility issues
- FR-4: The GitHub Actions workflow must only run on release tags, not on every push
- FR-5: The site must be navigable — every page reachable within 2 clicks from the homepage
- FR-6: Code blocks in documentation must use appropriate syntax highlighting (bash, yaml, markdown)
- FR-7: The `docs/avatar.png` must be used as both the site logo and homepage hero image

## Non-Goals

- No custom Vue components beyond basic theme overrides
- No search integration (VitePress built-in local search is fine if available, but not required)
- No versioned docs (single version reflecting latest release)
- No blog section
- No API/internals reference for Go packages — this is user-facing docs only
- No i18n/multilingual support

## Technical Considerations

- VitePress requires Node.js 18+; the GitHub Actions workflow should use Node 20
- The `base` option in VitePress config must match the GitHub Pages path (typically `/<repo-name>/`)
- The existing `docs/avatar.png` needs to be accessible from VitePress — place it in `docs/public/` for static asset serving
- GoReleaser already handles release creation; the docs workflow triggers on the same tag

## Success Metrics

- All README content is present in the docs site with better structure
- A new user can go from zero to running `maggus work` by following the Getting Started guide alone
- README.md fits on one screen without scrolling
- Docs deploy automatically on every release with no manual steps

## Open Questions

- What should the exact GitHub Pages URL be? (e.g., `leberkas-org.github.io/maggus/`)
- Should we add an `Edit this page on GitHub` link to each page for community contributions?
