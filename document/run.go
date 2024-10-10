package document

import (
	"encoding/xml"
	"fmt"
	"strings"

	"aqwari.net/xml/xmltree"
)

type Run struct {
	*xmltree.Element
	document *Document
}

func (run *Run) Clear() {
	newChildren := make([]xmltree.Element, 0, len(run.Children))
	for _, child := range run.Children {
		if child.Name.Local == "rPr" {
			newChildren = append(newChildren, child)
		}
	}
	run.Children = newChildren
	run.Content = nil
}

func (run *Run) Text() string {
	space := run.Name.Space
	output := strings.Builder{}
	for _, text := range run.Search(space, "t") {
		output.Write(text.Content)
	}
	return output.String()
}

func (run *Run) AddText(text string) {
	newElement := xmltree.Element{
		StartElement: xml.StartElement{
			Name: xml.Name{
				Space: run.Name.Space,
				Local: "t",
			},
		},
		Scope:    run.Scope,
		Content:  []byte(text),
		Children: nil,
	}
	run.Children = append(run.Children, newElement)
}

func (run *Run) AddInlineImage(rId string, w, h int) error {
	drawing, err := xmltree.Parse([]byte(drawingXml))
	if err != nil {
		return err
	}
	drawing.Scope = *run.Scope.JoinScope(&drawing.Scope)

	// update Id
	run.document.maxId++
	for _, node := range drawing.Search("", "docPr") {
		node.SetAttr("", "id", fmt.Sprintf("%d", run.document.maxId))
	}

	// update reference
	for _, node := range drawing.Search("", "blip") {
		node.SetAttr("http://schemas.openxmlformats.org/officeDocument/2006/relationships", "embed", rId)
	}

	// update dimensions
	wStr := fmt.Sprintf("%v", w)
	hStr := fmt.Sprintf("%v", h)

	// in extent
	for _, node := range drawing.Search("", "extent") {
		node.SetAttr("", "cx", wStr)
		node.SetAttr("", "cy", hStr)
	}
	// in ext
	for _, node := range drawing.Search("", "ext") {
		node.SetAttr("", "cx", wStr)
		node.SetAttr("", "cy", hStr)
	}
	run.Children = append(run.Children, *drawing)

	return nil
}

func (run *Run) TrimPrefix(prefix string) {
	for _, node := range run.Search("", "t") {
		strContent := string(node.Content)
		if len(strContent) == 0 {
			continue
		}
		if strings.HasPrefix(strContent, prefix) {
			node.Content = []byte(strings.TrimPrefix(strContent, prefix))
		}
		return
	}
}

func (run *Run) TrimSuffix(suffix string) {
	nodes := run.Search("", "t")
	for i := len(nodes) - 1; i >= 0; i-- {
		node := nodes[i]
		strContent := string(node.Content)
		if len(strContent) == 0 {
			continue
		}
		if strings.HasSuffix(strContent, suffix) {
			node.Content = []byte(strings.TrimSuffix(strContent, suffix))
		}
		return
	}
}
