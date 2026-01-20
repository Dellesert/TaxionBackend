package importschedule

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DocxDocument represents a parsed DOCX document
type DocxDocument struct {
	Paragraphs []DocxParagraph
	Tables     []DocxTable
}

// DocxParagraph represents a paragraph in the document
type DocxParagraph struct {
	Text string
}

// DocxTable represents a table in the document
type DocxTable struct {
	Rows []DocxRow
}

// DocxRow represents a row in a table
type DocxRow struct {
	Cells []DocxCell
}

// DocxCell represents a cell in a table row
type DocxCell struct {
	Paragraphs []DocxParagraph
}

// GetText returns all text from the cell
func (c *DocxCell) GetText() string {
	var texts []string
	for _, p := range c.Paragraphs {
		if p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, " ")
}

// ReadDocx reads a DOCX file from bytes and returns a DocxDocument
func ReadDocx(content []byte) (*DocxDocument, error) {
	// Open ZIP archive
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to open docx as zip: %w", err)
	}

	// Find and read document.xml
	var documentXML []byte
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open document.xml: %w", err)
			}
			documentXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read document.xml: %w", err)
			}
			break
		}
	}

	if documentXML == nil {
		return nil, fmt.Errorf("document.xml not found in docx")
	}

	// Parse document
	return parseDocumentXML(documentXML)
}

// xmlNode represents a node in the XML tree
type xmlNode struct {
	Name     string
	Text     string
	Children []*xmlNode
}

// parseDocumentXML parses the document.xml content
func parseDocumentXML(xmlContent []byte) (*DocxDocument, error) {
	// Build a tree structure first
	decoder := xml.NewDecoder(bytes.NewReader(xmlContent))
	root, err := buildXMLTree(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to build xml tree: %w", err)
	}

	doc := &DocxDocument{
		Paragraphs: make([]DocxParagraph, 0),
		Tables:     make([]DocxTable, 0),
	}

	// Find body element
	body := findNode(root, "body")
	if body == nil {
		return nil, fmt.Errorf("body element not found")
	}

	// Process body children
	for _, child := range body.Children {
		switch child.Name {
		case "p":
			// Top-level paragraph
			text := extractAllText(child)
			if text != "" {
				doc.Paragraphs = append(doc.Paragraphs, DocxParagraph{Text: text})
			}
		case "tbl":
			// Table
			table := parseTableNode(child)
			doc.Tables = append(doc.Tables, table)
		}
	}

	return doc, nil
}

// buildXMLTree builds a tree structure from XML
func buildXMLTree(decoder *xml.Decoder) (*xmlNode, error) {
	var root *xmlNode
	var stack []*xmlNode

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch elem := token.(type) {
		case xml.StartElement:
			node := &xmlNode{
				Name:     elem.Name.Local,
				Children: make([]*xmlNode, 0),
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			} else {
				root = node
			}
			stack = append(stack, node)

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			text := strings.TrimSpace(string(elem))
			if text != "" && len(stack) > 0 {
				current := stack[len(stack)-1]
				current.Text += text
			}
		}
	}

	return root, nil
}

// findNode finds first child node with given name (recursive)
func findNode(node *xmlNode, name string) *xmlNode {
	if node == nil {
		return nil
	}
	if node.Name == name {
		return node
	}
	for _, child := range node.Children {
		if found := findNode(child, name); found != nil {
			return found
		}
	}
	return nil
}

// extractAllText extracts all text from a node and its descendants
func extractAllText(node *xmlNode) string {
	if node == nil {
		return ""
	}

	var parts []string

	// Only collect text from 't' elements (w:t in Word XML)
	if node.Name == "t" && node.Text != "" {
		parts = append(parts, node.Text)
	}

	for _, child := range node.Children {
		childText := extractAllText(child)
		if childText != "" {
			parts = append(parts, childText)
		}
	}

	return strings.Join(parts, "")
}

// parseTableNode parses a table node
func parseTableNode(node *xmlNode) DocxTable {
	table := DocxTable{
		Rows: make([]DocxRow, 0),
	}

	for _, child := range node.Children {
		if child.Name == "tr" {
			row := parseRowNode(child)
			table.Rows = append(table.Rows, row)
		}
	}

	return table
}

// parseRowNode parses a table row node
func parseRowNode(node *xmlNode) DocxRow {
	row := DocxRow{
		Cells: make([]DocxCell, 0),
	}

	for _, child := range node.Children {
		if child.Name == "tc" {
			cell := parseCellNode(child)
			row.Cells = append(row.Cells, cell)
		}
	}

	return row
}

// parseCellNode parses a table cell node
func parseCellNode(node *xmlNode) DocxCell {
	cell := DocxCell{
		Paragraphs: make([]DocxParagraph, 0),
	}

	for _, child := range node.Children {
		if child.Name == "p" {
			text := extractAllText(child)
			cell.Paragraphs = append(cell.Paragraphs, DocxParagraph{Text: text})
		}
	}

	return cell
}
