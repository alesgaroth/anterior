package ante_test

import (
	"alesgaroth.com/anterior/ante"
	"bytes"
	"slices"
	"strconv"
	"testing"
)
import _ "embed"

func TestNoDataNoSub(t *testing.T) {
	template := "<html></html>"
	testIt(t, nil, template, template)
}

func TestAttrsPassThrough(t *testing.T) {
	template := "<p class='bob'></p>"
	testIt(t, nil, template, template)
}

func TestDocTypePassesThrough(t *testing.T) {
	template := "<!DOCTYPE html><html></html>"
	testIt(t, nil, template, template)
}

func TestSelfClosingPassesThrough(t *testing.T) {
	template := "<html><p/></html>"
	testIt(t, nil, template, template)
}

func TestCommentsPassThrough(t *testing.T) {
	template := "<p class='bob'><!-- this is a comment --></p>"
	testIt(t, nil, template, template)
}

var ds = &MapDataSource{
	map[string]string{
		"foo": "Baz",
	},
}

func TestField(t *testing.T) {
	template := "<p data-field='foo'>Bar</p>"
	expected := "<p data-field='foo'>Baz</p>"
	testIt(t, ds, template, expected)
}

func TestSelfClosingField(t *testing.T) {
	template := "<p data-field='foo'/>"
	expected := "<p data-field='foo'>Baz</p>"
	testIt(t, ds, template, expected)
}

func TestEmptyField(t *testing.T) {
	template := "<p data-field='fool'>Bar</p>"
	expected := "<p data-field='fool'></p>"
	testIt(t, ds, template, expected)
}

func TestItem(t *testing.T) {
	myds := &MapDataSourceDataSource{
		map[string]ante.DataSource{
			"item": ds,
		},
	}
	template := "\n<div data-item='item'><p data-field='foo'>Bar</p></div>"
	expected := "\n<div data-item='item'><p data-field='foo'>Baz</p></div>"
	testIt(t, myds, template, expected)
}

func TestLoop(t *testing.T) {
	myds := &ListDataSource{
		[]ante.DataSource{
			ds,
			&MapDataSource{
				map[string]string{
					"foo": "Bat",
				},
			},
		},
		0,
	}
	template := "\n<div data-list='1'><div data-repeating='true'><p data-field='foo'>Bar</p></div></div>"
	expected := "\n<div data-list='1'><div data-repeating='true'><p data-field='foo'>Baz</p></div><div data-repeating='true'><p data-field='foo'>Bat</p></div></div>"
	testIt(t, myds, template, expected)
}

func TestLoopy(t *testing.T) {
	myds := &ListDataSource{
		[]ante.DataSource{
			ds,
		},
		0,
	}
	template := "\n<div data-list='1'><div data-repeating='true'><p data-field='foo'>Bar</p></div><div data-repeating='true'><p data-field='foo'>Bag</p></div></div>"
	expected := "\n<div data-list='1'><div data-repeating='true'><p data-field='foo'>Baz</p></div></div>"
	testIt(t, myds, template, expected)
}

func TestAttr(t *testing.T) {
	template := "<a data-attr-href='foo'>Bar</a>"
	expected := "<a data-attr-href='foo' href='Baz'>Bar</a>"
	expected2 := "<a href='Baz' data-attr-href='foo'>Bar</a>"
	testIt2(t, ds, template, []string{expected, expected2})
}

func testIt(t *testing.T, ds ante.DataSource, template string, expected string) {
	testIt2(t, ds, template, []string{expected})
}
func testIt2(t *testing.T, ds ante.DataSource, template string, expected []string) {

	tmplt := ante.NewAnteTemplate(template)
	output := &bytes.Buffer{}
	tmplt.FillIn(output, ds)

	if !slices.Contains(expected, output.String()) {
		t.Errorf("template output does not match : got %v expected %v", output, expected)
		return
	}
}

//go:embed bmc.html
var templateString string

func TestBig(t *testing.T) {
	tmplt := ante.NewAnteTemplate(templateString)
	output := &bytes.Buffer{}
	tmplt.FillIn(output, ds)
	//t.Errorf("Got : %v", output)
}

type MapDataSource struct {
	mp map[string]string
}

func (mds *MapDataSource) Get(key string) string {
	return mds.mp[key]
}
func (mds *MapDataSource) GetDS(key string) ante.DataSource {
	return nil
}
func (*MapDataSource) GetNext() ante.DataSource {
	return nil
}

type MapDataSourceDataSource struct {
	mp map[string]ante.DataSource
}

func (mds *MapDataSourceDataSource) Get(key string) string {
	return ""
}
func (mds *MapDataSourceDataSource) GetDS(key string) ante.DataSource {
	return mds.mp[key]
}
func (*MapDataSourceDataSource) GetNext() ante.DataSource {
	return nil
}

type ListDataSource struct {
	mp  []ante.DataSource
	num int
}

func (lds *ListDataSource) Get(key string) string {
	switch key {
	case "count":
		return strconv.Itoa(len(lds.mp))
	}
	return ""
}
func (lds *ListDataSource) GetDS(key string) ante.DataSource {
	return nil
}
func (lds *ListDataSource) GetNext() ante.DataSource {
	num := lds.num
	if len(lds.mp) <= num {
		return nil
	}
	lds.num += 1
	return lds.mp[num]
}
