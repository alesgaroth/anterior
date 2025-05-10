package ante

import (
	"fmt"
	"io"
	"maps"
	"strings"

	"golang.org/x/net/html"
)

type DataSource interface {
	Get(key string) string
	GetDS(key string) DataSource
	GetNext() DataSource
}

type AnteTemplate interface {
	FillIn(w io.Writer, ds DataSource) error
}

type anteTemplate struct {
	blocks []AnteTemplate
}
type dsTemplate struct {
	isALoop bool
	item    string
	blocks  []AnteTemplate
}

type stringTemplate struct {
	block string
}
type attrsTemplate struct {
	tagname   string
	attrs     map[string]string
	slash     string
	dataAttrs map[string]string
}

type substituteTemplate struct {
	block string
}

type templateLevel interface {
	onError(templates []AnteTemplate) AnteTemplate
	onEndTag(templates []AnteTemplate, endTag string) (AnteTemplate, []AnteTemplate)
	updateTagName(startTagName string)
}

type topLevel bool

func (topLevel) onError(templates []AnteTemplate) AnteTemplate {
	return &anteTemplate{templates}
}

func (topLevel) onEndTag(templates []AnteTemplate, endTag string) (AnteTemplate, []AnteTemplate) {
	return nil, append(templates, &stringTemplate{"</" + endTag + ">"})
}
func (topLevel) updateTagName(startTagName string) {
	// do nothing
}

type subTemplate struct {
	tagName  string
	isALoop  bool
	dataItem string
	depth    int
}

func (st *subTemplate) onError(templates []AnteTemplate) AnteTemplate {
	templates = append(templates, &stringTemplate{"</" + st.tagName + ">"})
	return &dsTemplate{st.isALoop, st.dataItem, templates}
}
func (st *subTemplate) onEndTag(templates []AnteTemplate, endTag string) (AnteTemplate, []AnteTemplate) {
	templates = append(templates, &stringTemplate{"</" + endTag + ">"})
	if st.tagName == endTag {
		st.depth -= 1
		if st.depth < 1 {
			return &dsTemplate{st.isALoop, st.dataItem, templates}, nil
		}
	}
	return nil, templates
}

func (st *subTemplate) updateTagName(startTagName string) {
	if startTagName == st.tagName {
		st.depth += 1
	}
}

func NewAnteTemplate(tmplt string) AnteTemplate {
	z := html.NewTokenizer(strings.NewReader(tmplt))
	var templates []AnteTemplate

	return parseTemplate(z, topLevel(false), templates)
}

func parseTemplate(z *html.Tokenizer, level templateLevel, templates []AnteTemplate) AnteTemplate {
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return level.onError(templates)
		case html.TextToken:
			templates = append(templates, &stringTemplate{string(z.Text()[:])})
		case html.CommentToken:
			text := string(z.Text()[:])
			templates = append(templates, &stringTemplate{"<!--" + text + "-->"})
		case html.DoctypeToken:
			text := string(z.Text()[:])
			templates = append(templates, &stringTemplate{"<!DOCTYPE " + text + ">"})
		case html.EndTagToken:
			eTagName, _ := z.TagName()
			var retval AnteTemplate
			retval, templates = level.onEndTag(templates, string(eTagName[:]))
			if retval != nil {
				return retval
			}
		case html.SelfClosingTagToken, html.StartTagToken:
			tagName, hasAttrs := z.TagName()
			level.updateTagName(string(tagName[:]))
			templates = recurseIt(hasAttrs, string(tagName[:]), tt == html.SelfClosingTagToken, templates, z)
		}
	}
}

func newItem(initTemplate AnteTemplate, tagName string, dataItem string, z *html.Tokenizer, isALoop bool) AnteTemplate {
	templates := []AnteTemplate{initTemplate}
	level := &subTemplate{tagName, isALoop, dataItem, 1}
	return parseTemplate(z, level, templates)
}

func newField(initTemplate AnteTemplate, tagName string, dataField string, z *html.Tokenizer) AnteTemplate {
	var templates []AnteTemplate
	templates = append(templates, initTemplate)
	templates = append(templates, &substituteTemplate{dataField})
	templates = append(templates, &stringTemplate{"</" + tagName + ">"})
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return &anteTemplate{templates}
		case html.TextToken:
			// continue
		// case html.StartToken:
		// error!
		case html.EndTagToken:
			//endTagName, _ := z.TagName()
			//if string(endTagName[:]) != tagName {
			// error?
			//}
			return &anteTemplate{templates}
		}
	}

}

func recurseIt(hasAttrs bool, startTagName string, isSelfClosing bool, templates []AnteTemplate, z *html.Tokenizer) []AnteTemplate {

	newdataField := ""
	newdataItem := ""
	dataRepeating := ""
	dataAttrs := make(map[string]string)
	allAttrs := make(map[string]string)
	for hasAttrs {
		var bkey, bval []byte
		bkey, bval, hasAttrs = z.TagAttr()
		key := string(bkey[:])
		val := string(bval[:])
		switch key {
		case "data-field":
			newdataField = val
		case "data-item":
			newdataItem = val
		case "data-repeating":
			dataRepeating = val
		default:
			if attr, hasPrefix := strings.CutPrefix(key, "data-attr-"); hasPrefix {
				dataAttrs[attr] = val
			}
		}
		allAttrs[key] = val
	}

	slash := "/"
	if !isSelfClosing || newdataItem != "" || newdataField != "" {
		slash = ""
	}

	var initTemplate AnteTemplate
	if len(dataAttrs) < 1 {
		initTemplate = &stringTemplate{"<" + buildTag(startTagName, allAttrs, slash) + ">"}
	} else {
		initTemplate = &attrsTemplate{startTagName, allAttrs, slash, dataAttrs}
	}
	if newdataItem != "" || dataRepeating != "" {
		templates = append(templates, newItem(initTemplate, startTagName, newdataItem, z, dataRepeating != ""))
	} else if newdataField != "" {
		templates = append(templates, newField(initTemplate, startTagName, newdataField, z))
	} else {
		templates = append(templates, initTemplate)
	}
	return templates
}

func attr(key, val string) string {
	return key + "='" + val + "'"
}

/* run time below */

func (at *anteTemplate) FillIn(w io.Writer, ds DataSource) error {
	var errs []error
	for _, block := range at.blocks {
		err := block.FillIn(w, ds)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors while filling in %v", errs)
	}
	return nil
}

func (at *dsTemplate) fillInLoop(w io.Writer, ds DataSource) []error {
	if ds == nil {
		return []error{fmt.Errorf(" no loop for '%v'", at.item) }
	}
	var errs []error
	for innerDs := ds.GetNext(); innerDs != nil; innerDs = ds.GetNext() {
		errs = at.fillInOnce(w, ds, innerDs, errs)
	}
	return errs
}

func (at *dsTemplate) fillInOnce(w io.Writer, ds DataSource, innerDs DataSource, errs []error) []error {
	for _, block := range at.blocks {
		err := block.FillIn(w, innerDs)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (at *dsTemplate) fillIn(w io.Writer, ds DataSource) []error {
	if at.isALoop {
		return at.fillInLoop(w, ds)
	} else {
		var errs []error
		innerDs := ds.GetDS(at.item)
		return at.fillInOnce(w, ds, innerDs, errs)
	}
}
func (at *dsTemplate) FillIn(w io.Writer, ds DataSource) error {
	errs := at.fillIn(w, ds)
	if len(errs) > 0 {
		return fmt.Errorf("'%s' %v", at.item, errs)
	}
	return nil
}

func (at *stringTemplate) FillIn(w io.Writer, ds DataSource) error {
	_, err := w.Write([]byte(at.block))
	return err
}

func (at *substituteTemplate) FillIn(w io.Writer, ds DataSource) error {
	_, err := w.Write([]byte(ds.Get(at.block)))
	return err
}

func (at *attrsTemplate) getReplacedAttrs(ds DataSource) map[string]string {
	myAttrs := maps.Clone(at.attrs)
	for attr, key := range at.dataAttrs {
		myAttrs[attr] = ds.Get(key)
	}
	return myAttrs
}

func buildTag(tagname string, myAttrs map[string]string, slash string) string {
	attrs := []string{tagname}
	for key, val := range myAttrs {
		attrs = append(attrs, attr(key, val))
	}
	return strings.Join(attrs, " ") + slash
}

func (at *attrsTemplate) FillIn(w io.Writer, ds DataSource) error {
	myAttrs := at.getReplacedAttrs(ds)
	tagString := buildTag(at.tagname, myAttrs, at.slash)
	_, err := w.Write([]byte("<" + tagString + ">"))
	return err
}
