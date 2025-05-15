package exte

import (
	"fmt"
	"io"
	"os"
	"regexp"

	"alesgaroth.com/anterior/ante"
	uritemplate "github.com/yosida95/uritemplate/v3"
	yaml "gopkg.in/yaml.v3"
	"net/http"
)

type Queryr interface {
	DoQuery() ante.DataSource
}

type Extedata struct {
	Path     string  `yaml:"path"`
	Template string  `yaml:"template"`
	Queries  []Query `yaml:"queries"`
}
type Query struct {
	Name    string   `yaml:"name"`
	SQL     string   `yaml:"sql"`
	Columns []string `yaml:"columns"`
	Single  bool     `yaml:"single"`
	Joins   []Joined `yaml:"joins"`
}

type Joined struct {
	Name    string   `yaml:"name"`
	Columns []string `yaml:"columns"`
}

type ExteQueryr struct {
	Db      DB
	Queries []Query
}

type DB interface {
	Query(string) ante.DataSource
}

type qd struct {
	ds       ante.DataSource
	q        Query
	ds_cache ante.DataSource
}

type exteDataSource struct {
	datasources map[string]qd
}

func (eds *exteDataSource) Get(key string) string {
	return ""
}
func (eds *exteDataSource) GetDS(key string) ante.DataSource {
	q := eds.datasources[key]
	if q.ds_cache == nil {
		q.ds_cache = &riorAdapter{q.q, q.q.Columns, q.ds, make(map[string]ante.DataSource)}
	}
	return q.ds_cache
}

func (q *exteDataSource) GetNext() ante.DataSource {
	return nil
}

type riorAdapter struct {
	q        Query
	cols     []string
	ds       ante.DataSource
	ds_cache map[string]ante.DataSource
}

func (q *riorAdapter) Get(key string) string {
	for _, col := range q.cols {
		if col == key {
			return q.ds.Get(key)
		}
	}
	return ""
}
func (q riorAdapter) GetDS(key string) ante.DataSource {
	return emptyDS(false)
}
func (q riorAdapter) GetNext() ante.DataSource {
	return nil
}

type emptyDS bool

func (emptyDS) GetDS(key string) ante.DataSource {
	return emptyDS(false)
}
func (emptyDS) Get(key string) string {
	return ""
}
func (q emptyDS) GetNext() ante.DataSource {
	return nil
}

func (cq *ExteQueryr) DoQuery() ante.DataSource {
	eds := &exteDataSource{make(map[string]qd)}
	for _, query := range cq.Queries {
		if cq.Db == nil {
			panic("cq.Db is nil")
		}
		eds.datasources[query.Name] = qd{cq.Db.Query(query.SQL), query, nil}
	}
	return eds
}

type HandlerEntry struct {
	re      *regexp.Regexp
	handler http.HandlerFunc
}

type handlerCollector struct {
	errs       []error
	db         DB
	tmplengine TemplateEngine
	handlers   *[]HandlerEntry
	plugins    []Plugin
}

type Plugin interface {
	GetHandlers(ed Extedata, tmplengine TemplateEngine, db DB) []HandlerEntry
}
type TemplateEngine interface {
	ParseTemplate(f io.Reader) (ante.AnteTemplate, error)
}

func (e *handlerCollector) collectHandlers(ed Extedata) {
	f, err := os.Open(ed.Template)
	if err != nil {
		e.errs = append(e.errs, err)
		return
	}
	defer f.Close()
	tmplt, err := e.tmplengine.ParseTemplate(f)
	if err != nil {
		e.errs = append(e.errs, err)
		return
	}
	if e.db == nil {
		panic("e.db is nil")
	}
	handler := CreateHandler(tmplt, &ExteQueryr{e.db, ed.Queries})
	defer func() {
		if r := recover(); r != nil {
			e.errs = append(e.errs, fmt.Errorf("\nrecovering from panic in mux.Handle()\n%v", r))
		}
	}()
	tmpl, err := uritemplate.New(ed.Path)
	if err != nil {
		e.errs = append(e.errs, err)
	}
	re := tmpl.Regexp()
	*e.handlers = append(*e.handlers, HandlerEntry{re, handler})
	ed.plugins(e)
}

func (ed Extedata) plugins(e *handlerCollector) {
	for _, plugin := range e.plugins {
		*e.handlers = append(*e.handlers, plugin.GetHandlers(ed, e.tmplengine, e.db)...)
	}
}

func CreateHandlers(filename string, db DB, tmplengine TemplateEngine, plugins []Plugin) (http.HandlerFunc, error) {
	extedata, err := ParseYaml(filename)
	if err != nil {
		return nil, err
	}
	entries := []HandlerEntry{}
	handlerrs := &handlerCollector{[]error{}, db, tmplengine, &entries, plugins}
	for _, ed := range extedata {
		handlerrs.collectHandlers(ed)
	}
	notFoundHandler := func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("yo, I think we hit a snag and an error 404"))
	}
	handler := func(rw http.ResponseWriter, req *http.Request) {
		for _, entry := range *handlerrs.handlers {
			if entry.re.MatchString(req.URL.Path) { // we can do better!
				req.Pattern = entry.re.String()
				entry.handler(rw, req)
				return
			}
		}
		// 404!
		notFoundHandler(rw, req)
	}
	if len(handlerrs.errs) > 0 {
		return handler, fmt.Errorf("errors: %v", handlerrs.errs)
	}
	return handler, nil
}

func CreateHandler(template ante.AnteTemplate, q Queryr) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		template.FillIn(rw, q.DoQuery())
	}
}

func ParseYaml(filename string) ([]Extedata, error) {
	f, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Unable to read file %s %v", filename, err)
	}
	var extedata []Extedata
	if err := yaml.Unmarshal(f, &extedata); err != nil {
		return nil, fmt.Errorf("Unable to unmarshal file %s %v", filename, err)
	}
	return extedata, nil
}
