package document

import (
	"aqwari.net/xml/xmltree"
)

type Body struct {
	*xmltree.Element
	Name     string
	Document *Document
}

func (body *Body) Paragraphs() []*Paragraph {
	space := body.StartElement.Name.Space

	var paragraphs []*Paragraph

	for _, paragraph := range body.Search(space, "p") {
		paragraphs = append(paragraphs, &Paragraph{paragraph})
	}

	return paragraphs
}

func (body *Body) LinkImage(imageName string) (string, error) {
	return body.Document.linkImage(imageName, body.Name)
}
