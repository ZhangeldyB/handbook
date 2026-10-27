# Handbook Authoring Guide

This repository is the source of truth for Go service architecture guidance: a documentation corpus, compiled examples, and an agent bundle (`.agents/`). `scripts/install.sh` copies it into service repositories at `.handbook/` and projects the bundle into `.claude/` and the root `AGENTS.md`/`CLAUDE.md`. Every file except repo-meta files (`.git`, `.github`, `.gitignore`, this `CLAUDE.md`, `Makefile`, `go.work`, `tools/`) is production agent context in those repositories.

Go conventions live in the corpus. This file states none — it only governs how the corpus is written.

## Surface Map

| Surface | Location | Purpose |
|---|---|---|
| Docs | `architecture/`, `layers/`, `patterns/`, `implementation/`, `guides/`, `testing/` | Authoritative topic docs |
| ADRs | `decisions/` | Recorded decisions |
| Rules | `.agents/rules/` | Path-scoped rules → `.claude/rules/` |
| Skills | `.agents/skills/` | Task recipes → `.claude/skills/` |
| Agents | `.agents/agents/` | Subagents → `.claude/agents/` |
| Hooks | `.agents/hooks/`, `.agents/settings.json` | Portable hook scripts and their registration |
| Templates | `templates/` | Copy-source scaffolds and config files |
| Examples | `examples/` | Compiled, tested Go modules |
| Reference | `reference/` | Checklist, glossary, sources, distribution, validation |
| Tooling | `tools/doccheck`, `scripts/install.sh`, `Makefile` | Verification and installation |

## Authoring Standards

- **Root instruction budget:** `guides/ai-agent-instructions.md` (projected to every repository's `AGENTS.md` and `CLAUDE.md`) stays at or under 200 lines. `make verify-docs` enforces it.
- **Must/prefer:** use the definitions in `.agents/rules/go-quality.md` — *must* for correctness, security, compatibility, or architecture requirements; *prefer* for defaults that may yield to clearer code or measured evidence.
- **One authoritative source per topic:** deep content lives in exactly one document; everything else links to it. Rules hold rules, docs hold explanations, skills hold workflows, examples hold code. Skills and agents point at docs and examples rather than embedding copies.
- **Code in docs matches the examples:** snippets that show the handbook's own types (`aggregate.Tracker`, `Save`, `grpcerr.Mapper`, ...) are taken from `examples/` and updated with them. Illustrative snippets for code the examples do not contain must still compile in spirit (correct imports, no shadowed packages, current APIs).
- **Verified dates:** any claim about a tool, library, or Go version carries a `Last verified: <date>` note and is re-checked whenever it is touched.
- **Path discipline:** references meant for installed repositories use `.handbook/...`; links between documents use relative paths so they work both here and under `.handbook/`.
- **Current-only:** no superseded material, migration archives, dated audits, or investigation notes. Git keeps history.
- **Neutral and generic:** no organization, product, customer, or internal-library names. `tools/doccheck` keeps a banned-term list; extend it when needed.
- **Frontmatter:** rules declare a `paths:` list; skills declare `name` (equal to the directory) and `description`; agents declare `name`, `description`, and `tools` and never pin a model.

## Changing a Rule

1. Decide whether it is universal Go guidance, handbook architecture, or repository-local policy (the last does not belong here).
2. Change the one authoritative document, then every rule, skill, agent, template, and example that generates the pattern.
3. Record a load-bearing decision as a new ADR (or supersede an old one).
4. Run `make verify`.

## Validation

```bash
make verify
```

Runs example tests and vet, lints the orders example with `templates/golangci.yml`, checks links, anchors, `.handbook/` references, banned terms, frontmatter, and the root budget, runs the hook fixtures, and installs into a scratch repository. For manual checks (hook behavior in a live session, agent canaries), follow `reference/validation-runbook.md`.

## Git Policy

Agents working in this repository make file changes and stop. They do not run `git add`, `git commit`, or `git push`; the maintainer reviews and commits.
