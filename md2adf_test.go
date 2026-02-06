package md2adf

import (
	"encoding/json"
	"testing"
)

func TestConvert_Paragraph(t *testing.T) {
	result := Convert("Hello world")

	assertType(t, result, "doc")
	content := result["content"].([]Node)
	if len(content) != 1 {
		t.Fatalf("expected 1 content node, got %d", len(content))
	}

	para := content[0]
	assertType(t, para, "paragraph")

	paraContent := para["content"].([]Node)
	if len(paraContent) != 1 {
		t.Fatalf("expected 1 text node, got %d", len(paraContent))
	}
	assertText(t, paraContent[0], "Hello world")
}

func TestConvert_Heading(t *testing.T) {
	tests := []struct {
		input string
		level int
		text  string
	}{
		{"# Heading 1", 1, "Heading 1"},
		{"## Heading 2", 2, "Heading 2"},
		{"### Heading 3", 3, "Heading 3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Convert(tt.input)
			content := result["content"].([]Node)
			heading := content[0]

			assertType(t, heading, "heading")
			attrs := heading["attrs"].(Node)
			if attrs["level"] != tt.level {
				t.Errorf("expected level %d, got %v", tt.level, attrs["level"])
			}

			headingContent := heading["content"].([]Node)
			assertText(t, headingContent[0], tt.text)
		})
	}
}

func TestConvert_BulletList(t *testing.T) {
	input := `- Item 1
- Item 2
- Item 3`

	result := Convert(input)
	content := result["content"].([]Node)
	list := content[0]

	assertType(t, list, "bulletList")

	items := list["content"].([]Node)
	if len(items) != 3 {
		t.Fatalf("expected 3 list items, got %d", len(items))
	}

	for _, item := range items {
		assertType(t, item, "listItem")
		itemContent := item["content"].([]Node)
		para := itemContent[0]
		assertType(t, para, "paragraph")
	}
}

func TestConvert_OrderedList(t *testing.T) {
	input := `1. First
2. Second
3. Third`

	result := Convert(input)
	content := result["content"].([]Node)
	list := content[0]

	assertType(t, list, "orderedList")

	items := list["content"].([]Node)
	if len(items) != 3 {
		t.Fatalf("expected 3 list items, got %d", len(items))
	}
}

func TestConvert_CodeBlock(t *testing.T) {
	input := "```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```"

	result := Convert(input)
	content := result["content"].([]Node)
	codeBlock := content[0]

	assertType(t, codeBlock, "codeBlock")

	attrs := codeBlock["attrs"].(Node)
	if attrs["language"] != "go" {
		t.Errorf("expected language 'go', got %v", attrs["language"])
	}

	codeContent := codeBlock["content"].([]Node)
	if len(codeContent) != 1 {
		t.Fatalf("expected 1 text node in code block, got %d", len(codeContent))
	}
}

func TestConvert_Bold(t *testing.T) {
	result := Convert("This is **bold** text")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Should have: "This is ", "bold" (with strong mark), " text"
	if len(paraContent) < 3 {
		t.Fatalf("expected at least 3 nodes, got %d", len(paraContent))
	}

	// Check the bold text has strong mark
	boldNode := paraContent[1]
	marks := boldNode["marks"].([]Node)
	if len(marks) == 0 {
		t.Fatal("expected marks on bold text")
	}
	if marks[0]["type"] != "strong" {
		t.Errorf("expected 'strong' mark, got %v", marks[0]["type"])
	}
}

func TestConvert_Italic(t *testing.T) {
	result := Convert("This is *italic* text")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	if len(paraContent) < 3 {
		t.Fatalf("expected at least 3 nodes, got %d", len(paraContent))
	}

	// Check the italic text has em mark
	italicNode := paraContent[1]
	marks := italicNode["marks"].([]Node)
	if len(marks) == 0 {
		t.Fatal("expected marks on italic text")
	}
	if marks[0]["type"] != "em" {
		t.Errorf("expected 'em' mark, got %v", marks[0]["type"])
	}
}

func TestConvert_InlineCode(t *testing.T) {
	result := Convert("Use `fmt.Println()` here")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Find the code node
	var codeNode Node
	for _, n := range paraContent {
		if marks, ok := n["marks"].([]Node); ok {
			for _, m := range marks {
				if m["type"] == "code" {
					codeNode = n
					break
				}
			}
		}
	}

	if codeNode == nil {
		t.Fatal("expected to find inline code node")
	}
	if codeNode["text"] != "fmt.Println()" {
		t.Errorf("expected 'fmt.Println()', got %v", codeNode["text"])
	}
}

func TestConvert_Link(t *testing.T) {
	result := Convert("Click [here](https://example.com) for more")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Find the link node
	var linkNode Node
	for _, n := range paraContent {
		if marks, ok := n["marks"].([]Node); ok {
			for _, m := range marks {
				if m["type"] == "link" {
					linkNode = n
					break
				}
			}
		}
	}

	if linkNode == nil {
		t.Fatal("expected to find link node")
	}
	if linkNode["text"] != "here" {
		t.Errorf("expected link text 'here', got %v", linkNode["text"])
	}

	marks := linkNode["marks"].([]Node)
	linkMark := marks[0]
	attrs := linkMark["attrs"].(Node)
	if attrs["href"] != "https://example.com" {
		t.Errorf("expected href 'https://example.com', got %v", attrs["href"])
	}
}

func TestConvert_Blockquote(t *testing.T) {
	result := Convert("> This is a quote")
	content := result["content"].([]Node)
	quote := content[0]

	assertType(t, quote, "blockquote")

	quoteContent := quote["content"].([]Node)
	if len(quoteContent) == 0 {
		t.Fatal("expected content in blockquote")
	}
}

func TestConvert_ComplexDocument(t *testing.T) {
	input := "# Project Update\n\n" +
		"This is a **summary** of the work done.\n\n" +
		"## Changes\n\n" +
		"- Added new feature\n" +
		"- Fixed bug in *authentication*\n" +
		"- Updated `config.yaml`\n\n" +
		"## Code Example\n\n" +
		"```go\n" +
		"func main() {\n" +
		"    fmt.Println(\"Hello\")\n" +
		"}\n" +
		"```\n\n" +
		"For more info, see [documentation](https://docs.example.com).\n"

	result := Convert(input)

	// Should parse without panic
	_, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	content := result["content"].([]Node)
	if len(content) < 5 {
		t.Errorf("expected at least 5 top-level nodes, got %d", len(content))
	}

	// First should be heading
	assertType(t, content[0], "heading")
}

func TestConvert_ThematicBreak(t *testing.T) {
	result := Convert("Above\n\n---\n\nBelow")
	content := result["content"].([]Node)

	if len(content) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(content))
	}
	assertType(t, content[0], "paragraph")
	assertType(t, content[1], "rule")
	assertType(t, content[2], "paragraph")
}

func TestConvert_Image(t *testing.T) {
	result := Convert("![alt text](https://example.com/img.png)")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Image should be converted to a link
	if len(paraContent) == 0 {
		t.Fatal("expected content in paragraph")
	}
	imgNode := paraContent[0]
	if imgNode["text"] != "alt text" {
		t.Errorf("expected text 'alt text', got %v", imgNode["text"])
	}

	marks := imgNode["marks"].([]Node)
	linkMark := marks[0]
	if linkMark["type"] != "link" {
		t.Errorf("expected 'link' mark, got %v", linkMark["type"])
	}
	attrs := linkMark["attrs"].(Node)
	if attrs["href"] != "https://example.com/img.png" {
		t.Errorf("expected href 'https://example.com/img.png', got %v", attrs["href"])
	}
}

func TestConvert_Strikethrough(t *testing.T) {
	result := Convert("This is ~~deleted~~ text")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	if len(paraContent) < 3 {
		t.Fatalf("expected at least 3 nodes, got %d", len(paraContent))
	}

	strikeNode := paraContent[1]
	marks := strikeNode["marks"].([]Node)
	if len(marks) == 0 {
		t.Fatal("expected marks on strikethrough text")
	}
	if marks[0]["type"] != "strike" {
		t.Errorf("expected 'strike' mark, got %v", marks[0]["type"])
	}
	if strikeNode["text"] != "deleted" {
		t.Errorf("expected text 'deleted', got %v", strikeNode["text"])
	}
}

func TestConvert_Table(t *testing.T) {
	input := "| Name | Age |\n| --- | --- |\n| Alice | 30 |\n| Bob | 25 |"

	result := Convert(input)
	content := result["content"].([]Node)

	if len(content) != 1 {
		t.Fatalf("expected 1 node, got %d", len(content))
	}
	table := content[0]
	assertType(t, table, "table")

	rows := table["content"].([]Node)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (1 header + 2 data), got %d", len(rows))
	}

	// Check header row
	headerRow := rows[0]
	assertType(t, headerRow, "tableRow")
	headerCells := headerRow["content"].([]Node)
	if len(headerCells) != 2 {
		t.Fatalf("expected 2 header cells, got %d", len(headerCells))
	}
	assertType(t, headerCells[0], "tableHeader")
	assertType(t, headerCells[1], "tableHeader")

	// Check data rows
	dataRow := rows[1]
	assertType(t, dataRow, "tableRow")
	dataCells := dataRow["content"].([]Node)
	if len(dataCells) != 2 {
		t.Fatalf("expected 2 data cells, got %d", len(dataCells))
	}
	assertType(t, dataCells[0], "tableCell")
	assertType(t, dataCells[1], "tableCell")

	// Check cell content
	cellContent := dataCells[0]["content"].([]Node)
	para := cellContent[0]
	assertType(t, para, "paragraph")
	paraContent := para["content"].([]Node)
	assertText(t, paraContent[0], "Alice")
}

func TestConvert_TableWithFormatting(t *testing.T) {
	input := "| Feature | Status |\n| --- | --- |\n| **Auth** | ~~removed~~ |"

	result := Convert(input)
	content := result["content"].([]Node)
	table := content[0]
	assertType(t, table, "table")

	rows := table["content"].([]Node)
	dataRow := rows[1]
	cells := dataRow["content"].([]Node)

	// First cell should contain bold text
	cell0Content := cells[0]["content"].([]Node)
	para0 := cell0Content[0]
	paraContent := para0["content"].([]Node)
	boldNode := paraContent[0]
	marks := boldNode["marks"].([]Node)
	if marks[0]["type"] != "strong" {
		t.Errorf("expected 'strong' mark in table cell, got %v", marks[0]["type"])
	}

	// Second cell should contain strikethrough text
	cell1Content := cells[1]["content"].([]Node)
	para1 := cell1Content[0]
	para1Content := para1["content"].([]Node)
	strikeNode := para1Content[0]
	strikeMarks := strikeNode["marks"].([]Node)
	if strikeMarks[0]["type"] != "strike" {
		t.Errorf("expected 'strike' mark in table cell, got %v", strikeMarks[0]["type"])
	}
}

func TestConvert_NestedList(t *testing.T) {
	input := "- Item 1\n  - Nested A\n  - Nested B\n- Item 2"

	result := Convert(input)
	content := result["content"].([]Node)
	list := content[0]

	assertType(t, list, "bulletList")

	items := list["content"].([]Node)
	if len(items) != 2 {
		t.Fatalf("expected 2 top-level items, got %d", len(items))
	}

	// First item should contain a paragraph and a nested bullet list
	item1Content := items[0]["content"].([]Node)
	if len(item1Content) < 2 {
		t.Fatalf("expected at least 2 children in first item (paragraph + nested list), got %d", len(item1Content))
	}
	assertType(t, item1Content[0], "paragraph")
	assertType(t, item1Content[1], "bulletList")

	nestedItems := item1Content[1]["content"].([]Node)
	if len(nestedItems) != 2 {
		t.Fatalf("expected 2 nested items, got %d", len(nestedItems))
	}
}

func TestConvert_CodeBlockNoLanguage(t *testing.T) {
	input := "```\nplain code\n```"

	result := Convert(input)
	content := result["content"].([]Node)
	codeBlock := content[0]

	assertType(t, codeBlock, "codeBlock")

	// Should not have attrs when no language specified
	if _, hasAttrs := codeBlock["attrs"]; hasAttrs {
		t.Error("expected no attrs for code block without language")
	}

	codeContent := codeBlock["content"].([]Node)
	assertText(t, codeContent[0], "plain code")
}

func TestConvert_BoldItalicCombined(t *testing.T) {
	result := Convert("This is ***bold and italic*** text")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Find the node with both marks
	var found bool
	for _, n := range paraContent {
		if marks, ok := n["marks"].([]Node); ok && len(marks) == 2 {
			markTypes := map[string]bool{}
			for _, m := range marks {
				markTypes[m["type"].(string)] = true
			}
			if markTypes["strong"] && markTypes["em"] {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected to find node with both 'strong' and 'em' marks")
	}
}

func TestConvert_InlineCard_AutoLink(t *testing.T) {
	result := Convert("Check <https://jira.example.com/browse/DEV-123>")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Find the inlineCard node
	var card Node
	for _, n := range paraContent {
		if n["type"] == "inlineCard" {
			card = n
			break
		}
	}
	if card == nil {
		t.Fatal("expected to find inlineCard node")
	}
	attrs := card["attrs"].(Node)
	if attrs["url"] != "https://jira.example.com/browse/DEV-123" {
		t.Errorf("expected url 'https://jira.example.com/browse/DEV-123', got %v", attrs["url"])
	}
}

func TestConvert_InlineCard_BareURL(t *testing.T) {
	result := Convert("See https://jira.example.com/browse/DEV-456 for details")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Find the inlineCard node
	var card Node
	for _, n := range paraContent {
		if n["type"] == "inlineCard" {
			card = n
			break
		}
	}
	if card == nil {
		t.Fatal("expected to find inlineCard node for bare URL")
	}
	attrs := card["attrs"].(Node)
	if attrs["url"] != "https://jira.example.com/browse/DEV-456" {
		t.Errorf("expected url 'https://jira.example.com/browse/DEV-456', got %v", attrs["url"])
	}
}

func TestConvert_ExplicitLink_StaysAsLink(t *testing.T) {
	result := Convert("Click [this ticket](https://jira.example.com/browse/DEV-789)")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Should NOT be an inlineCard — user chose display text
	for _, n := range paraContent {
		if n["type"] == "inlineCard" {
			t.Fatal("explicit markdown link should not become inlineCard")
		}
	}

	// Should be a text node with link mark
	var linkNode Node
	for _, n := range paraContent {
		if marks, ok := n["marks"].([]Node); ok {
			for _, m := range marks {
				if m["type"] == "link" {
					linkNode = n
					break
				}
			}
		}
	}
	if linkNode == nil {
		t.Fatal("expected to find link mark on explicit link")
	}
	if linkNode["text"] != "this ticket" {
		t.Errorf("expected link text 'this ticket', got %v", linkNode["text"])
	}
}

func TestConvert_EmailAutoLink(t *testing.T) {
	result := Convert("Contact <user@example.com> for help")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Find the email link node
	var emailNode Node
	for _, n := range paraContent {
		if marks, ok := n["marks"].([]Node); ok {
			for _, m := range marks {
				if m["type"] == "link" {
					emailNode = n
					break
				}
			}
		}
	}
	if emailNode == nil {
		t.Fatal("expected to find link mark on email autolink")
	}
	if emailNode["text"] != "user@example.com" {
		t.Errorf("expected text 'user@example.com', got %v", emailNode["text"])
	}
	marks := emailNode["marks"].([]Node)
	attrs := marks[0]["attrs"].(Node)
	if attrs["href"] != "mailto:user@example.com" {
		t.Errorf("expected href 'mailto:user@example.com', got %v", attrs["href"])
	}

	// Should NOT be an inlineCard
	for _, n := range paraContent {
		if n["type"] == "inlineCard" {
			t.Fatal("email autolink should not become inlineCard")
		}
	}
}

func TestConvert_BareEmail(t *testing.T) {
	result := Convert("Send mail to support@example.com please")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Find the email link node
	var emailNode Node
	for _, n := range paraContent {
		if marks, ok := n["marks"].([]Node); ok {
			for _, m := range marks {
				if m["type"] == "link" {
					emailNode = n
					break
				}
			}
		}
	}
	if emailNode == nil {
		t.Fatal("expected to find link mark on bare email")
	}
	if emailNode["text"] != "support@example.com" {
		t.Errorf("expected text 'support@example.com', got %v", emailNode["text"])
	}
	marks := emailNode["marks"].([]Node)
	attrs := marks[0]["attrs"].(Node)
	if attrs["href"] != "mailto:support@example.com" {
		t.Errorf("expected href 'mailto:support@example.com', got %v", attrs["href"])
	}
}

func TestConvert_ExplicitMailtoLink(t *testing.T) {
	result := Convert("[Email us](mailto:info@example.com)")
	content := result["content"].([]Node)
	para := content[0]
	paraContent := para["content"].([]Node)

	// Should be a text node with link mark (handled by ast.Link, not AutoLink)
	var linkNode Node
	for _, n := range paraContent {
		if marks, ok := n["marks"].([]Node); ok {
			for _, m := range marks {
				if m["type"] == "link" {
					linkNode = n
					break
				}
			}
		}
	}
	if linkNode == nil {
		t.Fatal("expected to find link mark on explicit mailto link")
	}
	if linkNode["text"] != "Email us" {
		t.Errorf("expected text 'Email us', got %v", linkNode["text"])
	}
	marks := linkNode["marks"].([]Node)
	attrs := marks[0]["attrs"].(Node)
	if attrs["href"] != "mailto:info@example.com" {
		t.Errorf("expected href 'mailto:info@example.com', got %v", attrs["href"])
	}
}

func TestConvert_EmptyInput(t *testing.T) {
	result := Convert("")
	assertType(t, result, "doc")
	content := result["content"].([]Node)
	if len(content) != 0 {
		t.Errorf("expected 0 content nodes for empty input, got %d", len(content))
	}
}

// Helper functions

func assertType(t *testing.T, node Node, expectedType string) {
	t.Helper()
	if node["type"] != expectedType {
		t.Errorf("expected type '%s', got '%v'", expectedType, node["type"])
	}
}

func assertText(t *testing.T, node Node, expectedText string) {
	t.Helper()
	if node["text"] != expectedText {
		t.Errorf("expected text '%s', got '%v'", expectedText, node["text"])
	}
}
