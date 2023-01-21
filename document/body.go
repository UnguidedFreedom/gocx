package document

import (
	"aqwari.net/xml/xmltree"
)

type Body struct {
	*xmltree.Element
	name     string
	document *Document
}

func (body *Body) Paragraphs() []*Paragraph {
	space := body.StartElement.Name.Space

	var paragraphs []*Paragraph

	for _, paragraph := range body.Search(space, "p") {
		paragraphs = append(paragraphs, &Paragraph{Element: paragraph, document: body.document})
	}

	return paragraphs
}

func (body *Body) LinkImage(imageName string) (string, error) {
	return body.document.linkImage(imageName, body.name)
}
