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
	"strconv"
	"strings"
	"sync"

	"aqwari.net/xml/xmltree"
	"golang.org/x/sync/errgroup"
)

type Document struct {
	rawFiles map[string][]byte
	xmlFiles map[string]*xmltree.Element

	maxId int
}

func OpenDocument(filename string) (*Document, error) {
	var err error
	r, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := r.Close()
		if err == nil {
			err = closeErr
		}
	}()

	document := &Document{
		rawFiles: make(map[string][]byte, len(r.File)),
		xmlFiles: make(map[string]*xmltree.Element, len(r.File)),
	}

	replacer := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;", "'", "&apos;", `"`, "&quot;")

	for _, f := range r.File {
		var rc io.ReadCloser
		rc, err = f.Open()
		if err != nil {
			return nil, err
		}

		var data []byte
		data, err = io.ReadAll(rc)
		if err != nil {
			return nil, err
		}

		if err = rc.Close(); err != nil {
			return nil, err
		}

		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".xml.rels") {
			var root *xmltree.Element
			root, err = xmltree.Parse(data)
			if err != nil {
				return nil, err
			}
			document.xmlFiles[f.Name] = root

			// dirty hack to fix xmltree's broken xml encoding
			for _, element := range root.Flatten() {
				for _, attr := range element.StartElement.Attr {
					element.SetAttr(attr.Name.Space, attr.Name.Local, replacer.Replace(attr.Value))
				}

				if element.Name.Local == "docPr" {
					id := element.Attr("", "id")
					if id != "" {
						if idInt, err := strconv.Atoi(id); err == nil && idInt > document.maxId {
							document.maxId = idInt
						}
					}
				}
			}
		} else {
			document.rawFiles[f.Name] = data
		}
	}

	// returning err so that it can be overridden in the deferred func
	return document, err
}

func (document *Document) SaveToFile(filename string) error {
	var err error
	var output *os.File
	output, err = os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := output.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	w := zip.NewWriter(output)
	defer func() {
		if closeErr := w.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	g := new(errgroup.Group)
	var mu sync.Mutex

	for name, content := range document.rawFiles {
		name, content := name, content
		g.Go(func() error {
			mu.Lock()
			defer mu.Unlock()

			f, err := w.Create(name)
			if err != nil {
				return err
			}

			if _, err = f.Write(content); err != nil {
				return err
			}
			return nil
		})
	}

	for name, node := range document.xmlFiles {
		name, node := name, node
		g.Go(func() error {
			content := xmltree.Marshal(node)

			mu.Lock()
			defer mu.Unlock()

			f, err := w.Create(name)
			if err != nil {
				return err
			}

			if _, err = f.Write(content); err != nil {
				return err
			}
			return nil
		})
	}

	err = g.Wait()

	return err
}

func (document *Document) Headers() []*Header {
	var headers []*Header
	for filename, root := range document.xmlFiles {
		if strings.HasPrefix(filename, "word/header") {
			header := &Header{
				Element:  root,
				name:     strings.TrimPrefix(filename, "word/"),
				document: document,
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
		name:     "document.xml",
		document: document,
	}
}

func (document *Document) AddImageFromFile(filename string) (string, error) {
	imgData, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return document.AddImage(imgData)
}

func (document *Document) AddImage(raw []byte) (string, error) {
	index := 1
	for name := range document.rawFiles {
		if strings.HasPrefix(name, "word/media/image") {
			index++
		}
	}
	imgName := fmt.Sprintf("image%v.png", index)
	imgPath := fmt.Sprintf("word/media/%v", imgName)
	document.rawFiles[imgPath] = raw

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

func (document *Document) GetImageDimensions(imgName string) (int, int, error) {
	imgPath := fmt.Sprintf("word/media/%v", imgName)
	img, _, err := image.DecodeConfig(bytes.NewReader(document.rawFiles[imgPath]))
	if err != nil {
		return 0, 0, err
	}
	return img.Width, img.Height, nil
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
	maxRelId := 0
	for _, rel := range relRoot.Children {
		id := rel.Attr("", "Id")
		if rel.Attr("", "Target") == target {
			return id, nil
		}
		relId, err := strconv.Atoi(strings.TrimPrefix(id, relIdPrefix))
		if err == nil && relId > maxRelId {
			maxRelId = relId
		}
	}
	// if we reach this point the relationship didn't exist => create it
	rId := fmt.Sprintf("%v%v", relIdPrefix, maxRelId+1)
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
