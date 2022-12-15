package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
	"strings"

	"aqwari.net/xml/xmltree"
)

type Document struct {
	rawFiles map[string][]byte
	xmlFiles map[string]*xmltree.Element
}

func OpenDocument(filename string) (*Document, error) {
	r, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	document := &Document{
		rawFiles: make(map[string][]byte, len(r.File)),
		xmlFiles: make(map[string]*xmltree.Element, len(r.File)),
	}

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}

		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}

		rc.Close()

		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".xml.rels") {
			root, err := xmltree.Parse(data)
			if err != nil {
				return nil, err
			}
			document.xmlFiles[f.Name] = root
		} else {
			document.rawFiles[f.Name] = data
		}
	}

	return document, nil
}

func (document *Document) SaveToFile(filename string) error {
	output, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer output.Close()

	w := zip.NewWriter(output)
	defer w.Close()

	for name, content := range document.rawFiles {
		f, err := w.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(content)
		if err != nil {
			return err
		}
	}

	for name, node := range document.xmlFiles {
		f, err := w.Create(name)
		if err != nil {
			return err
		}

		if _, err := f.Write(xmltree.Marshal(node)); err != nil {
			return err
		}
	}

	return nil
}

func (document *Document) Headers() []*Header {
	var headers []*Header
	for filename, root := range document.xmlFiles {
		if strings.HasPrefix(filename, "word/header") {
			header := &Header{
				Element:  root,
				Name:     strings.TrimPrefix(filename, "word/"),
				Document: document,
			}
			headers = append(headers, header)
		}
	}
	return headers
}

func (document *Document) Body() *Body {
	body := document.xmlFiles["word/document.xml"]
	return &Body{
		Element:  body,
		Name:     "document.xml",
		Document: document,
	}
}

func (document *Document) AddImage(filename string) (string, error) {
	imgData, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	index := 1
	for name := range document.rawFiles {
		if strings.HasPrefix(name, "word/media/image") {
			index++
		}
	}
	imgName := fmt.Sprintf("image%v.png", index)
	imgPath := fmt.Sprintf("word/media/%v", imgName)
	document.rawFiles[imgPath] = imgData

	// ensure PNG is in the content types
	contentTypes := document.xmlFiles["[Content_Types].xml"]
	found := false
	for _, child := range contentTypes.Children {
		if child.Name.Local == "Default" && child.Attr("", "Extension") == "png" {
			found = true
			break
		}
	}
	if !found {
		pngDefault := xmltree.Element{
			StartElement: xml.StartElement{
				Name: xml.Name{Local: "Default"},
				Attr: []xml.Attr{
					{
						Name:  xml.Name{Local: "Extension"},
						Value: "png",
					},
					{
						Name:  xml.Name{Local: "ContentType"},
						Value: "image/png",
					},
				},
			},
			Scope: xmltree.Scope{},
		}
		contentTypes.Children = append(contentTypes.Children, pngDefault)
	}
	return imgName, nil
}

func (document *Document) GetImageDimensions(imgName string) (int, int) {
	imgPath := fmt.Sprintf("word/media/%v", imgName)
	img, _, err := image.DecodeConfig(bytes.NewReader(document.rawFiles[imgPath]))
	if err != nil {
		panic(err)
	}
	return img.Width, img.Height
}

func (document *Document) linkImage(imageName string, fileName string) (string, error) {
	relPath := fmt.Sprintf("word/_rels/%v.rels", fileName)
	if _, hasRel := document.xmlFiles[relPath]; !hasRel {
		relRoot, err := xmltree.Parse([]byte(relationshipsXml))
		if err != nil {
			return "", err
		}
		document.xmlFiles[relPath] = relRoot
	}
	relRoot := document.xmlFiles[relPath]
	target := fmt.Sprintf("media/%v", imageName)
	for _, rel := range relRoot.Children {
		if rel.Attr("", "Target") == target {
			return rel.Attr("", "Id"), nil
		}
	}
	// if we reach this point the relationship didn't exist => create it
	rId := fmt.Sprintf("rId%v", len(relRoot.Children)+1)
	rel := xmltree.Element{
		StartElement: xml.StartElement{
			Name: xml.Name{Local: "Relationship"},
			Attr: []xml.Attr{
				{
					Name:  xml.Name{Local: "Id"},
					Value: rId,
				},
				{
					Name:  xml.Name{Local: "Target"},
					Value: target,
				},
				{
					Name:  xml.Name{Local: "Type"},
					Value: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
				},
			},
		},
		Scope:    relRoot.Scope,
		Content:  nil,
		Children: nil,
	}
	relRoot.Children = append(relRoot.Children, rel)
	return rId, nil
}
