# Swarm Release Process

This document outlines the standard operating procedure for preparing and
publishing a new release of the Swarm CLI. This process ensures codebase
hygiene, documentation alignment, and artifact stability.

## 1. Codebase Hygiene

Run the following checks to ensure the Go source code is formatted, linted,
and dependencies are clean:

```bash
go fmt ./...
go vet ./...
go mod tidy
golangci-lint run ./...
go test ./... -short
```

## 2. Documentation Formatting

All markdown documentation must be hard-wrapped to 78 columns using
`mdformat`.

**CRITICAL EXEMPTION:** You MUST NOT run `mdformat` blindly on the entire
repository (`mdformat .`). You **MUST EXEMPT** the `skills/` directory.
`SKILL.md` files contain YAML frontmatter that `mdformat` will permanently
corrupt, breaking the agent metadata parser.

The safe formatting command is:

```bash
find . -type f -name "*.md" -not -path "./skills/*" -not -path "./.swarm/*" -not -path "*/.git/*" -exec mdformat --wrap 78 {} +
```

## 3. Release Notes

Update `RELEASE_NOTES.md` with a synthesized, high-level summary of all
features, fixes, and architectural changes since the last release tag. Use
`git log --pretty=format:"* %s" v0.XX..HEAD` to gather the raw commits.

## 4. Tagging and Publishing

Once all tests pass and documentation is formatted:

1. Commit the final changes (e.g., `docs: prepare v0.0X release`).
1. Tag the release: `git tag v0.0X`
1. Push to main and push tags: `git push origin main && git push origin v0.0X`
