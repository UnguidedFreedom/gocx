package document

import "aqwari.net/xml/xmltree"

type Paragraph struct {
	*xmltree.Element
}

func (paragraph *Paragraph) Runs() []*Run {
	space := paragraph.StartElement.Name.Space

	var runs []*Run
	for _, run := range (paragraph).Search(space, "r") {
		runs = append(runs, &Run{run})
	}

	return runs
}
