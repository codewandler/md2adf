# md2adf

A Go library that converts Markdown to [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/) — the JSON-based rich text format used by Jira Cloud, Confluence, and other Atlassian products.

## Installation

```bash
go get github.com/codewandler/md2adf
```

Requires **Go 1.25.6** or later.

## Quick start

```go
package main

import (
    "encoding/json"
    "fmt"

    "github.com/codewandler/md2adf"
)

func main() {
    doc := md2adf.Convert("# Hello\n\nSome **bold** and *italic* text.")
    out, _ := json.MarshalIndent(doc, "", "  ")
    fmt.Println(string(out))
}
```

Output:

```json
{
  "version": 1,
  "type": "doc",
  "content": [
    {
      "type": "heading",
      "attrs": { "level": 1 },
      "content": [{ "type": "text", "text": "Hello" }]
    },
    {
      "type": "paragraph",
      "content": [
        { "type": "text", "text": "Some " },
        { "type": "text", "text": "bold", "marks": [{ "type": "strong" }] },
        { "type": "text", "text": " and " },
        { "type": "text", "text": "italic", "marks": [{ "type": "em" }] },
        { "type": "text", "text": " text." }
      ]
    }
  ]
}
```

## Supported Markdown features

### Block elements

| Markdown | ADF node type |
|---|---|
| Paragraphs | `paragraph` |
| `# Heading` (levels 1-6) | `heading` with `level` attr |
| `- item` / `* item` | `bulletList` → `listItem` |
| `1. item` | `orderedList` → `listItem` |
| Nested lists | Nested `bulletList` / `orderedList` inside `listItem` |
| `` ```lang `` fenced code | `codeBlock` with optional `language` attr |
| Indented code blocks | `codeBlock` |
| `> quote` | `blockquote` |
| `---` / `***` | `rule` |
| GFM tables | `table` → `tableRow` → `tableHeader` / `tableCell` |

### Inline elements

| Markdown | ADF representation |
|---|---|
| `**bold**` | `"strong"` mark |
| `*italic*` | `"em"` mark |
| `~~strikethrough~~` | `"strike"` mark |
| `` `code` `` | `"code"` mark |
| `[text](url)` | `"link"` mark with `href` attr |
| `<https://...>` autolinks | `inlineCard` with `url` attr |
| Bare URLs (e.g. `https://...`) | `inlineCard` with `url` attr |
| `![alt](url)` images | Text node with `"link"` mark (ADF has no inline image) |
| Hard line breaks | `hardBreak` node |
| Soft line breaks | Space text node |

Marks can be combined — e.g. `***bold italic***` produces a text node with both `strong` and `em` marks.

## API

### `md2adf.Node`

```go
type Node map[string]any
```

A generic JSON-like map representing a single ADF node. Serialise it with `json.Marshal` to produce the payload expected by Atlassian REST APIs.

### `md2adf.Convert`

```go
func Convert(markdown string) Node
```

Converts a Markdown string into a top-level ADF `"doc"` node (version 1). The Markdown parser uses the [goldmark](https://github.com/yuin/goldmark) library with the **table**, **strikethrough**, and **linkify** extensions enabled.

An empty input produces a valid doc node with an empty content array.

## How it works

```
Markdown string
      │
      ▼
goldmark parser  (with table + strikethrough + linkify extensions)
      │
      ▼
goldmark AST
      │
      ▼
recursive conversion
  ├── convertNode()            — block-level elements
  ├── convertInlineChildren()  — inline elements with mark accumulation
  ├── convertListItems()       — list item wrappers
  ├── convertTable()           — table structure
  └── mergeTextNodes()         — consolidate fragmented text runs
      │
      ▼
ADF Node tree  (map[string]any — ready for json.Marshal)
```

## Running tests

```bash
go test ./...          # Run all tests
go test -v ./...       # Verbose output
go test -run TestName  # Run a specific test
```

## License

See [LICENSE](LICENSE) for details.
