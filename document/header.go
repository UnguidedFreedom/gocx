package document

import (
	"aqwari.net/xml/xmltree"
)

type Header struct {
	*xmltree.Element
	name     string
	document *Document
}

func (header *Header) Paragraphs() []*Paragraph {
	space := header.StartElement.Name.Space

	var paragraphs []*Paragraph

	for _, paragraph := range header.Search(space, "p") {
		paragraphs = append(paragraphs, &Paragraph{Element: paragraph, document: header.document})
	}

	return paragraphs
}

func (header *Header) LinkImage(imageName string) (string, error) {
	return header.document.linkImage(imageName, header.name)
}
