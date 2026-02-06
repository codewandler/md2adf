// Package md2adf converts Markdown to Atlassian Document Format (ADF).
//
// ADF is the JSON-based document format used by Jira Cloud and Confluence
// for rich text content in issue descriptions, comments, and page bodies.
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

// Node represents an ADF node.
type Node map[string]any

// Convert transforms Markdown text into an ADF document structure.
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

// convertChildren processes all child nodes and returns their ADF representations.
func convertChildren(n ast.Node, source []byte) []Node {
	var nodes []Node
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if node := convertNode(child, source); node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// convertNode converts a single goldmark AST node to an ADF node.
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

// convertListItems converts list item nodes.
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

// convertInlineChildren processes inline content (text, emphasis, links, etc.)
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

// mergeTextNodes consolidates adjacent text nodes that share identical marks.
// This is needed because goldmark extensions (e.g. Linkify) can split text at
// probe points, producing fragmented text nodes.
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

// marksEqual returns true if two nodes have identical mark sets.
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

// convertTable converts a goldmark table to an ADF table node.
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

// convertTableCells converts table cell children into ADF tableHeader or tableCell nodes.
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

// copyMarks creates a copy of the marks slice to avoid mutation issues.
func copyMarks(marks []Node) []Node {
	if marks == nil {
		return nil
	}
	result := make([]Node, len(marks))
	copy(result, marks)
	return result
}
