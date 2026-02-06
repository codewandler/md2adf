# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build ./...          # Build
go test ./...           # Run all tests
go test -run TestName   # Run a single test
go test -v ./...        # Verbose test output
```

## Git Conventions

Semantic commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`, etc.

## Architecture

Single-package Go library (`package md2adf`) that converts Markdown to Atlassian Document Format (ADF) JSON. ADF is used by Jira Cloud and Confluence for rich text content.

**Conversion pipeline:** Markdown string → goldmark parser → goldmark AST → recursive ADF node tree

- `Convert()` is the public entry point; returns a `Node` (which is `map[string]any`)
- `convertNode()` handles block-level elements (paragraphs, headings, lists, code blocks, blockquotes, thematic breaks)
- `convertInlineChildren()` handles inline elements (text, emphasis, code spans, links, images) with recursive mark accumulation
- Marks (bold, italic, code, link) are passed down through inline recursion and attached to leaf text nodes
- Images are converted to links (ADF doesn't support inline images the same way as Markdown)
