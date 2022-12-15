package document

import (
	"aqwari.net/xml/xmltree"
)

type Header struct {
	*xmltree.Element
	Name     string
	Document *Document
}

func (header *Header) Paragraphs() []*Paragraph {
	space := header.StartElement.Name.Space

	var paragraphs []*Paragraph

	for _, paragraph := range header.Search(space, "p") {
		paragraphs = append(paragraphs, &Paragraph{paragraph})
	}

	return paragraphs
}

func (header *Header) LinkImage(imageName string) (string, error) {
	return header.Document.linkImage(imageName, header.Name)
}
