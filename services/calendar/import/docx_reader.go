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

// XML structures for parsing DOCX

// WordDocument represents the main document.xml structure
type WordDocument struct {
	XMLName xml.Name   `xml:"document"`
	Body    WordBody   `xml:"body"`
}

// WordBody represents the body element
type WordBody struct {
	Content []WordBodyContent `xml:",any"`
}

// WordBodyContent represents any content in the body
type WordBodyContent struct {
	XMLName xml.Name
	Content []byte `xml:",innerxml"`
}

// WordParagraph represents a paragraph (w:p)
type WordParagraph struct {
	XMLName xml.Name  `xml:"p"`
	Runs    []WordRun `xml:"r"`
}

// WordRun represents a run (w:r)
type WordRun struct {
	XMLName xml.Name   `xml:"r"`
	Text    []WordText `xml:"t"`
}

// WordText represents text (w:t)
type WordText struct {
	XMLName xml.Name `xml:"t"`
	Content string   `xml:",chardata"`
}

// WordTable represents a table (w:tbl)
type WordTable struct {
	XMLName xml.Name       `xml:"tbl"`
	Rows    []WordTableRow `xml:"tr"`
}

// WordTableRow represents a table row (w:tr)
type WordTableRow struct {
	XMLName xml.Name        `xml:"tr"`
	Cells   []WordTableCell `xml:"tc"`
}

// WordTableCell represents a table cell (w:tc)
type WordTableCell struct {
	XMLName    xml.Name        `xml:"tc"`
	Paragraphs []WordParagraph `xml:"p"`
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

// parseDocumentXML parses the document.xml content
func parseDocumentXML(xmlContent []byte) (*DocxDocument, error) {
	doc := &DocxDocument{
		Paragraphs: make([]DocxParagraph, 0),
		Tables:     make([]DocxTable, 0),
	}

	// Create decoder
	decoder := xml.NewDecoder(bytes.NewReader(xmlContent))

	// Parse the XML manually to handle the Word namespace properly
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml parse error: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			localName := elem.Name.Local

			if localName == "p" {
				// Parse paragraph
				para, err := parseParagraph(decoder)
				if err != nil {
					continue
				}
				if para.Text != "" {
					doc.Paragraphs = append(doc.Paragraphs, para)
				}
			} else if localName == "tbl" {
				// Parse table
				table, err := parseTable(decoder)
				if err != nil {
					continue
				}
				doc.Tables = append(doc.Tables, table)
			}
		}
	}

	return doc, nil
}

// parseParagraph parses a paragraph element
func parseParagraph(decoder *xml.Decoder) (DocxParagraph, error) {
	para := DocxParagraph{}
	var textParts []string
	depth := 1

	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return para, err
		}

		switch elem := token.(type) {
		case xml.StartElement:
			depth++
			if elem.Name.Local == "t" {
				// Read text content
				textToken, err := decoder.Token()
				if err == nil {
					if charData, ok := textToken.(xml.CharData); ok {
						textParts = append(textParts, string(charData))
					}
				}
			}
		case xml.EndElement:
			depth--
		}
	}

	para.Text = strings.Join(textParts, "")
	return para, nil
}

// parseTable parses a table element
func parseTable(decoder *xml.Decoder) (DocxTable, error) {
	table := DocxTable{
		Rows: make([]DocxRow, 0),
	}
	depth := 1

	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return table, err
		}

		switch elem := token.(type) {
		case xml.StartElement:
			depth++
			if elem.Name.Local == "tr" {
				row, err := parseTableRow(decoder)
				if err == nil {
					table.Rows = append(table.Rows, row)
					depth-- // parseTableRow consumed the end element
				}
			}
		case xml.EndElement:
			depth--
		}
	}

	return table, nil
}

// parseTableRow parses a table row element
func parseTableRow(decoder *xml.Decoder) (DocxRow, error) {
	row := DocxRow{
		Cells: make([]DocxCell, 0),
	}
	depth := 1

	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return row, err
		}

		switch elem := token.(type) {
		case xml.StartElement:
			depth++
			if elem.Name.Local == "tc" {
				cell, err := parseTableCell(decoder)
				if err == nil {
					row.Cells = append(row.Cells, cell)
					depth-- // parseTableCell consumed the end element
				}
			}
		case xml.EndElement:
			depth--
		}
	}

	return row, nil
}

// parseTableCell parses a table cell element
func parseTableCell(decoder *xml.Decoder) (DocxCell, error) {
	cell := DocxCell{
		Paragraphs: make([]DocxParagraph, 0),
	}
	depth := 1

	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return cell, err
		}

		switch elem := token.(type) {
		case xml.StartElement:
			depth++
			if elem.Name.Local == "p" {
				para, err := parseParagraph(decoder)
				if err == nil {
					cell.Paragraphs = append(cell.Paragraphs, para)
					depth-- // parseParagraph consumed the end element
				}
			}
		case xml.EndElement:
			depth--
		}
	}

	return cell, nil
}
