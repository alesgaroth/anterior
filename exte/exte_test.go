package exte_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"alesgaroth.com/anterior/ante"
	"alesgaroth.com/anterior/exte"
)

type TestResponseWriter struct {
	buf *bytes.Buffer
}

func (TestResponseWriter) Header() http.Header {
	return nil
}

func (t TestResponseWriter) Write(b []byte) (int, error) {
	return t.buf.Write(b)
}

func (TestResponseWriter) WriteHeader(statuscode int) {
}

func createRootRequest() (*http.Request, error) {
	return createRequest("/")
}
func createRequest(path string) (*http.Request, error) {
	uri, err := url.Parse("https://alesgaroth.com" + path)
	if err != nil {
		return nil, fmt.Errorf("unable to parse https://alesgaroth.com")
	}
	return &http.Request{
		Method:     "GET",
		URL:        uri,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
		Header: map[string][]string{
			"Accept-Encoding": {"gzip", "deflate"},
			"Accept-Language": {"en-ca"},
		},
		Body:             io.NopCloser(strings.NewReader("")),
		ContentLength:    0,
		TransferEncoding: []string{},
		Host:             "alesgaroth.com",
		Form:             url.Values{},
		Trailer:          map[string][]string{},
		RemoteAddr:       "127.0.0.1:2345",
		RequestURI:       path,
	}, nil
}

func TestStaticTemplate(t *testing.T) {
	request, err := createRootRequest()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	mytemplate := "<html/>"

	template, err := StaticAnte(1).ParseTemplate(strings.NewReader(mytemplate))
	if err != nil {
		t.Errorf("unable to parse template \"%s\"  %v", mytemplate, err)
		return
	}

	testIt(t, template, request, SimpleRior(1), mytemplate)
}

func TestDifferentHandlersGiveDifferent(t *testing.T) {

	request, err := createRootRequest()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	mytemplate := "Hello "
	template, err := SimpleAnte(1).ParseTemplate(strings.NewReader(mytemplate))
	if err != nil {
		t.Errorf("sigh, %v", err)
	}

	testIt(t, template, request, SimpleRior(1), "Hello 1")
	template2, err := SimpleAnte(1).ParseTemplate(strings.NewReader("GoodBye "))
	if err != nil {
		t.Errorf("sigh, %v", err)
	}
	testIt(t, template2, request, SimpleRior(2), "GoodBye 2")
}

func TestRouting(t *testing.T) {
	filename := "config.yaml"
	handlers, err := exte.CreateHandlers(filename, SimpleRior(3), StaticAnte(1), nil)
	if err != nil {
		t.Error(err)
	}
	type testdata struct {
		path     string
		expected string
	}
	tests := []testdata{
		testdata{
			path:     "/",
			expected: "<html><head></head><body></body></html>\n",
		},
		testdata{
			path: "/blog/",
			expected: `<html>
<head>
<title>My blog</title>
</head>
<body>
	<div class="aw_menusections">
		<div class="aw_`,
		},
		testdata{
			path: "/blog/p7.html",
			expected: `<html>
<head>
<title>My blog post</title>
</head>
<body>
	<div class="aw_menusections">
		<div class`,
		},
	}
	for _, td := range tests {
		req, err := createRequest(td.path)
		if err != nil {
			t.Errorf("%v", err)
			return
		}
		tester(t, handlers, req, td.expected)
	}
}

func TestSingles(t *testing.T) {

	// test that queries returning a single row return the expected
	queries := []exte.Query{
		exte.Query{
			Name:    "foo",
			SQL:     "Query1",
			Columns: []string{"bar"},
			Single:  true,
			Joins:   []exte.Joined{},
		},
	}
	back := &ArrRior{map[string]string{"bar": "baz"}}
	eq := exte.ExteQueryr{
		Db:      back,
		Queries: queries,
	}

	ds := eq.DoQuery()
	foo := ds.GetDS("foo")
	if got := foo.Get("bar"); got != "baz" {
		t.Errorf("oops eq bar: expected 'baz' got '%v'", got)
	}
}

func TestMultipleRows(t *testing.T) {
	// test that queries returning multiple rows return the expected
	// I'm not testing what I think I'm testing
	queries := []exte.Query{
		exte.Query{
			Name:    "foo",
			SQL:     "Query1",
			Columns: []string{"bar"},
			Single:  false,
			Joins:   []exte.Joined{},
		},
	}
	back := &ArrArrRior{map[string]*ArrRior{
		"1": &ArrRior{map[string]string{"bar": "baz"}},
		"2": &ArrRior{map[string]string{"bar": "bat"}},
	}}
	eq := exte.ExteQueryr{
		Db:      back,
		Queries: queries,
	}

	ds := eq.DoQuery()
	foo := ds.GetDS("foo")
	row1 := foo.GetDS("1")
	if got := row1.Get("bar"); got != "baz" {
		t.Errorf("oops eq bar: expected 'baz' got '%v'", got)
	}
	row2 := foo.GetDS("2")
	if got := row2.Get("bar"); got != "bat" {
		t.Errorf("oops eq bar: expected 'bat' got '%v'", got)
	}
}

func TestColumns(t *testing.T) {
	// test that you can only see the columns for the table, not its joins
	queries := []exte.Query{
		exte.Query{
			Name:    "foo",
			SQL:     "Query1",
			Columns: []string{"bar"},
			Single:  false,
			Joins:   []exte.Joined{},
		},
	}
	back := &ArrRior{map[string]string{"bar": "baz", "splat": "sploit"}}
	eq := exte.ExteQueryr{
		Db:      back,
		Queries: queries,
	}

	ds := eq.DoQuery()
	foo := ds.GetDS("foo")
	if got := foo.Get("splat"); got != "" {
		t.Errorf("eq splat:  shouldn't be visible, got %v", got)
	}
}

func TestJoins(t *testing.T) {
	// test that you can navigate into the joins ...
	queries := []exte.Query{
		exte.Query{
			Name:    "foo",
			SQL:     "Query1",
			Columns: []string{"bar"},
			Single:  false,
			Joins: []exte.Joined{
				exte.Joined{
					Name:    "join1",
					Columns: []string{"jcol1", "jcol2"},
				},
			},
		},
	}
	back := &ArrRior{map[string]string{"bar": "baz", "jcol1": "sploit", "jcol2": "splat"}}
	eq := exte.ExteQueryr{
		Db:      back,
		Queries: queries,
	}
	ds := eq.DoQuery()
	foo := ds.GetDS("foo")
	join1 := foo.GetDS("join1")
	if got := join1.Get("jcol1"); got != "sploit" {
		t.Errorf("eq jcol1:  should be 'sploit', got '%v'", got)
	}
}

func TestJoints(t *testing.T) {
	// test that you can navigate into the joins that have their own subqueries ...
	// and it will show the correct data
}

func TestParsing(t *testing.T) {
	filename := "config.yaml"
	extedata, err := exte.ParseYaml(filename)
	if err != nil {
		t.Error(err)
	}

	if len(extedata) != 3 {
		t.Errorf("expected 3 got %d %v\n", len(extedata), extedata)
	}
	if extedata[1].Template != "blog.html" {
		t.Errorf("expected second template to be blog.html was %s in %v", extedata[1].Template, extedata[1])
	}
	if extedata[2].Template != "post.html" {
		t.Errorf("expected third template to be post.html was %s in %v", extedata[2].Template, extedata[2])
	}
}

func testIt(t *testing.T, template ante.AnteTemplate, request *http.Request, rior exte.Queryr, expected string) {
	handler := exte.CreateHandler(template, rior)
	tester(t, handler, request, expected)
}

func tester(t *testing.T, handler http.HandlerFunc, request *http.Request, expected string) {
	buf := bytes.Buffer{}
	rw := TestResponseWriter{&buf}

	handler.ServeHTTP(rw, request)

	got := string(buf.Bytes())
	if expected != got {
		t.Errorf("buf : expected \"%v\" got \"%v\"", expected, got)
	}
}

type SimpleRior int

func (s SimpleRior) DoQuery() ante.DataSource {
	return SimpleDS(s)
}

func (s SimpleRior) Query(sql string) ante.DataSource {
	return SimpleDS(s)
}

type SimpleDS int

func (s SimpleDS) Get(name string) string {
	return fmt.Sprintf("%d", s)
}
func (SimpleDS) GetDS(name string) ante.DataSource {
	return nil
}
func (SimpleDS) GetNext() ante.DataSource {
	return nil
}

type SimpleAnte int
type SimpleTemplate struct {
	bytes []byte
}

func (s SimpleAnte) ParseTemplate(r io.Reader) (ante.AnteTemplate, error) {
	st := SimpleTemplate{make([]byte, 100)}
	len, _ := r.Read(st.bytes)
	return &SimpleTemplate{st.bytes[:len]}, nil
}

func (s *SimpleTemplate) FillIn(w io.Writer, ds ante.DataSource) error {
	if _, err := w.Write(s.bytes); err != nil {
		return err
	}
	if _, err := io.WriteString(w, ds.Get("me")); err != nil {
		return err
	}
	return nil
}

type StaticAnte int
type StaticTemplate struct {
	bytes []byte
}

func (s StaticAnte) ParseTemplate(r io.Reader) (ante.AnteTemplate, error) {
	st := make([]byte, 100)
	len, _ := r.Read(st)
	return &StaticTemplate{st[:len]}, nil
}

func (s *StaticTemplate) FillIn(w io.Writer, ds ante.DataSource) error {
	_, err := w.Write(s.bytes)
	return err
}

type MyAnte struct {
	colsToGet []string
	reponses  []string
}
type MyTemplate struct {
	ante MyAnte
}

func (m MyAnte) ParseTemplate(r io.Reader) (ante.AnteTemplate, error) {
	return &MyTemplate{m}, nil
}

func (s *MyTemplate) FillIn(w io.Writer, ds ante.DataSource) error {
	s.ante.reponses = make([]string, len(s.ante.colsToGet))
	for i, col := range s.ante.colsToGet {
		s.ante.reponses[i] = ds.Get(col)
	}
	return nil
}

type ArrRior struct {
	m map[string]string
}

func (ar *ArrRior) Query(name string) ante.DataSource {
	return ar
}

func (ar *ArrRior) Get(name string) string {
	return ar.m[name]
}
func (ar *ArrRior) GetDS(name string) ante.DataSource {
	return nil
}
func (ar *ArrRior) GetNext() ante.DataSource {
	return nil
}

type ArrArrRior struct {
	arr map[string]*ArrRior
}

func (arr *ArrArrRior) Query(name string) ante.DataSource {
	return arr
}

func (ar *ArrArrRior) Get(name string) string {
	return ""
}
func (ar *ArrArrRior) GetDS(name string) ante.DataSource {
	return ar.arr[name]
}
func (ar *ArrArrRior) GetNext() ante.DataSource {
	return nil
}
