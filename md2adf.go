// Package md2adf converts Markdown text into Atlassian Document Format (ADF).
//
// ADF is the JSON-based document format used by Jira Cloud and Confluence
// for rich text content in issue descriptions, comments, and page bodies.
// See https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
//
// The conversion pipeline works as follows:
//
//  1. Parse the Markdown string using goldmark (with table, strikethrough, and linkify extensions).
//  2. Walk the resulting goldmark AST.
//  3. Recursively build an ADF node tree from the AST.
//
// # Supported Markdown elements
//
// Block-level: paragraphs, headings (1-6), bullet lists, ordered lists,
// nested lists, fenced/indented code blocks, blockquotes, thematic breaks,
// and tables (with header rows).
//
// Inline: bold, italic, strikethrough, inline code, links, autolinks
// (rendered as ADF inlineCard nodes), images (converted to links), hard
// breaks, and soft breaks.
//
// # Usage
//
//	doc := md2adf.Convert("# Hello\n\nSome **bold** text.")
//	jsonBytes, _ := json.Marshal(doc)
package md2adf

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// Node represents a single ADF node as a generic JSON-like map.
//
// Every node has at least a "type" key (e.g. "doc", "paragraph", "text").
// Depending on the type it may also carry "content" (child nodes), "attrs"
// (type-specific attributes such as heading level or link href), "text"
// (leaf text content), and "marks" (inline formatting such as bold or link).
//
// Because Node is simply map[string]any it can be passed directly to
// [encoding/json.Marshal] to produce the JSON payload expected by Atlassian
// REST APIs.
type Node map[string]any

// Convert transforms a Markdown string into an ADF document node.
//
// The returned [Node] is a top-level "doc" node (version 1) whose "content"
// array contains the converted block-level elements. An empty Markdown string
// produces a valid doc node with an empty content array.
//
// The Markdown parser is configured with the goldmark table, strikethrough,
// and linkify extensions, so GFM-style tables, ~~strikethrough~~, and bare
// URLs are all recognized.
func Convert(markdown string) Node {
	source := []byte(markdown)
	reader := text.NewReader(source)
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.Linkify,
		),
	)
	doc := md.Parser().Parse(reader)

	return Node{
		"version": 1,
		"type":    "doc",
		"content": convertChildren(doc, source),
	}
}

// convertChildren iterates over the direct children of n and converts each
// one via [convertNode]. Nil results (e.g. empty paragraphs) are silently
// dropped.
func convertChildren(n ast.Node, source []byte) []Node {
	var nodes []Node
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if node := convertNode(child, source); node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// convertNode maps a single goldmark AST block node to its ADF equivalent.
//
// Supported block types:
//   - [ast.Paragraph] / [ast.TextBlock] → "paragraph"
//   - [ast.Heading]                     → "heading" (with level attr)
//   - [ast.List]                        → "bulletList" or "orderedList"
//   - [ast.FencedCodeBlock]             → "codeBlock" (with optional language attr)
//   - [ast.CodeBlock]                   → "codeBlock" (indented, no language)
//   - [ast.Blockquote]                  → "blockquote"
//   - [ast.ThematicBreak]               → "rule"
//   - [extast.Table]                    → "table"
//
// Unrecognized block types with children fall through: the first converted
// child is returned so that content is not silently lost. Truly unknown or
// empty nodes return nil.
func convertNode(n ast.Node, source []byte) Node {
	switch node := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		content := convertInlineChildren(node, source, nil)
		if len(content) == 0 {
			return nil
		}
		return Node{
			"type":    "paragraph",
			"content": content,
		}

	case *ast.Heading:
		return Node{
			"type":    "heading",
			"attrs":   Node{"level": node.Level},
			"content": convertInlineChildren(node, source, nil),
		}

	case *ast.List:
		listType := "bulletList"
		if node.IsOrdered() {
			listType = "orderedList"
		}
		return Node{
			"type":    listType,
			"content": convertListItems(node, source),
		}

	case *ast.FencedCodeBlock:
		var buf bytes.Buffer
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			buf.Write(line.Value(source))
		}
		// Remove trailing newline if present
		code := buf.String()
		if len(code) > 0 && code[len(code)-1] == '\n' {
			code = code[:len(code)-1]
		}

		adfNode := Node{
			"type": "codeBlock",
			"content": []Node{
				{"type": "text", "text": code},
			},
		}
		if lang := string(node.Language(source)); lang != "" {
			adfNode["attrs"] = Node{"language": lang}
		}
		return adfNode

	case *ast.CodeBlock:
		var buf bytes.Buffer
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			buf.Write(line.Value(source))
		}
		code := buf.String()
		if len(code) > 0 && code[len(code)-1] == '\n' {
			code = code[:len(code)-1]
		}
		return Node{
			"type": "codeBlock",
			"content": []Node{
				{"type": "text", "text": code},
			},
		}

	case *ast.Blockquote:
		return Node{
			"type":    "blockquote",
			"content": convertChildren(node, source),
		}

	case *ast.ThematicBreak:
		return Node{"type": "rule"}

	case *extast.Table:
		return convertTable(node, source)

	default:
		// For unknown block types, try to process children
		if n.HasChildren() && n.Type() == ast.TypeBlock {
			children := convertChildren(n, source)
			if len(children) > 0 {
				return children[0] // Return first child for unknown blocks
			}
		}
		return nil
	}
}

// convertListItems converts the children of an [ast.List] into ADF "listItem"
// nodes. Each list item's block-level content (typically paragraphs and
// possibly nested lists) is preserved in the item's "content" array.
func convertListItems(list *ast.List, source []byte) []Node {
	var items []Node
	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		if li, ok := child.(*ast.ListItem); ok {
			// List items contain block content (usually paragraphs)
			// We need to wrap it properly for ADF
			content := convertChildren(li, source)
			items = append(items, Node{
				"type":    "listItem",
				"content": content,
			})
		}
	}
	return items
}

// convertInlineChildren recursively processes the inline children of a block
// node and returns a flat slice of ADF text/inlineCard/hardBreak nodes.
//
// The marks parameter carries the accumulated formatting context (bold,
// italic, code, link, strikethrough) from parent inline nodes and is attached
// to every leaf text node produced by the traversal. Marks are copied before
// being extended so that sibling branches do not share slices.
//
// Supported inline types:
//   - [ast.Text]              → "text" (with optional hardBreak / soft-break space)
//   - [ast.Emphasis]          → adds "em" (level 1) or "strong" (level 2) mark
//   - [ast.CodeSpan]          → "text" with "code" mark
//   - [ast.Link]              → adds "link" mark with href attr
//   - [ast.AutoLink]          → "inlineCard" with url attr
//   - [ast.Image]             → "text" with "link" mark (ADF has no inline image)
//   - [extast.Strikethrough]  → adds "strike" mark
//   - [ast.RawHTML]           → skipped
//
// After collecting all nodes the result is passed through [mergeTextNodes] to
// consolidate adjacent text nodes that share the same marks.
func convertInlineChildren(n ast.Node, source []byte, marks []Node) []Node {
	var nodes []Node

	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch node := child.(type) {
		case *ast.Text:
			text := string(node.Segment.Value(source))
			if text == "" {
				continue
			}
			textNode := Node{"type": "text", "text": text}
			if len(marks) > 0 {
				textNode["marks"] = copyMarks(marks)
			}
			nodes = append(nodes, textNode)

			// Handle soft/hard line breaks
			if node.HardLineBreak() {
				nodes = append(nodes, Node{"type": "hardBreak"})
			} else if node.SoftLineBreak() {
				// Soft breaks become spaces in ADF
				nodes = append(nodes, Node{"type": "text", "text": " "})
			}

		case *ast.Emphasis:
			// Single * or _ is italic (em), double ** or __ is bold (strong)
			markType := "em"
			if node.Level == 2 {
				markType = "strong"
			}
			newMarks := append(copyMarks(marks), Node{"type": markType})
			nodes = append(nodes, convertInlineChildren(node, source, newMarks)...)

		case *ast.CodeSpan:
			text := string(node.Text(source))
			textNode := Node{"type": "text", "text": text}
			newMarks := append(copyMarks(marks), Node{"type": "code"})
			textNode["marks"] = newMarks
			nodes = append(nodes, textNode)

		case *ast.Link:
			linkMark := Node{
				"type":  "link",
				"attrs": Node{"href": string(node.Destination)},
			}
			newMarks := append(copyMarks(marks), linkMark)
			nodes = append(nodes, convertInlineChildren(node, source, newMarks)...)

		case *ast.AutoLink:
			url := string(node.URL(source))
			nodes = append(nodes, Node{
				"type":  "inlineCard",
				"attrs": Node{"url": url},
			})

		case *ast.Image:
			// ADF doesn't support inline images the same way
			// Convert to a link with the alt text
			alt := string(node.Text(source))
			if alt == "" {
				alt = string(node.Destination)
			}
			linkMark := Node{
				"type":  "link",
				"attrs": Node{"href": string(node.Destination)},
			}
			newMarks := append(copyMarks(marks), linkMark)
			textNode := Node{"type": "text", "text": alt, "marks": newMarks}
			nodes = append(nodes, textNode)

		case *extast.Strikethrough:
			newMarks := append(copyMarks(marks), Node{"type": "strike"})
			nodes = append(nodes, convertInlineChildren(node, source, newMarks)...)

		case *ast.RawHTML:
			// Skip raw HTML
			continue

		default:
			// For other inline nodes, try to recurse
			if child.HasChildren() {
				nodes = append(nodes, convertInlineChildren(child, source, marks)...)
			}
		}
	}

	return mergeTextNodes(nodes)
}

// mergeTextNodes consolidates adjacent "text" nodes that share identical marks
// by concatenating their text values. This is necessary because goldmark
// extensions (e.g. Linkify) can split what is logically one text run at
// internal probe points, producing fragmented nodes that would result in
// unnecessarily verbose ADF output.
func mergeTextNodes(nodes []Node) []Node {
	if len(nodes) <= 1 {
		return nodes
	}
	merged := []Node{nodes[0]}
	for _, node := range nodes[1:] {
		prev := merged[len(merged)-1]
		if prev["type"] == "text" && node["type"] == "text" && marksEqual(prev, node) {
			prev["text"] = prev["text"].(string) + node["text"].(string)
			continue
		}
		merged = append(merged, node)
	}
	return merged
}

// marksEqual reports whether two text nodes carry the same set of marks.
// Two nodes are considered equal if they both have no marks, or if their mark
// slices are the same length and each pair of marks has identical string
// representations. This is used by [mergeTextNodes] to decide whether
// adjacent text nodes can be combined.
func marksEqual(a, b Node) bool {
	aMarks, aOk := a["marks"].([]Node)
	bMarks, bOk := b["marks"].([]Node)
	if !aOk && !bOk {
		return true
	}
	if !aOk || !bOk || len(aMarks) != len(bMarks) {
		return false
	}
	for i := range aMarks {
		if fmt.Sprint(aMarks[i]) != fmt.Sprint(bMarks[i]) {
			return false
		}
	}
	return true
}

// convertTable converts a goldmark [extast.Table] into an ADF "table" node.
//
// The resulting table has "isNumberColumnEnabled" set to false and layout
// "default". The first child (TableHeader) produces cells of type
// "tableHeader"; subsequent TableRow children produce "tableCell" nodes.
func convertTable(table *extast.Table, source []byte) Node {
	var rows []Node
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch row := child.(type) {
		case *extast.TableHeader:
			rows = append(rows, Node{
				"type":    "tableRow",
				"content": convertTableCells(row, source, "tableHeader"),
			})
		case *extast.TableRow:
			rows = append(rows, Node{
				"type":    "tableRow",
				"content": convertTableCells(row, source, "tableCell"),
			})
		}
	}
	return Node{
		"type":    "table",
		"attrs":   Node{"isNumberColumnEnabled": false, "layout": "default"},
		"content": rows,
	}
}

// convertTableCells converts the [extast.TableCell] children of a table row
// into ADF nodes of the given cellType ("tableHeader" or "tableCell"). Each
// cell's inline content is wrapped in a paragraph node, as required by the
// ADF schema. Empty cells receive a paragraph with an empty content array.
func convertTableCells(row ast.Node, source []byte, cellType string) []Node {
	var cells []Node
	for child := row.FirstChild(); child != nil; child = child.NextSibling() {
		if _, ok := child.(*extast.TableCell); ok {
			inlineContent := convertInlineChildren(child, source, nil)
			var content []Node
			if len(inlineContent) > 0 {
				content = []Node{{
					"type":    "paragraph",
					"content": inlineContent,
				}}
			} else {
				content = []Node{{
					"type":    "paragraph",
					"content": []Node{},
				}}
			}
			cells = append(cells, Node{
				"type":    cellType,
				"content": content,
			})
		}
	}
	return cells
}

// copyMarks returns a shallow copy of the marks slice so that callers can
// safely append to it without mutating the slice shared by sibling inline
// nodes. A nil input produces a nil result.
func copyMarks(marks []Node) []Node {
	if marks == nil {
		return nil
	}
	result := make([]Node, len(marks))
	copy(result, marks)
	return result
}
