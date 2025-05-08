package exte

import (
	"fmt"
	"os"
	"regexp"
	//"strconv"

	"alesgaroth.com/anterior/ante"
	"alesgaroth.com/anterior/rior"
	uritemplate "github.com/yosida95/uritemplate/v3"
	yaml "gopkg.in/yaml.v3"
	"net/http"
)

type Queryr interface {
	DoQuery() rior.DataSource
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
	Db      rior.DB
	Queries []Query
}

type qd struct {
	ds rior.DataSource
	q Query
	ds_cache rior.DataSource
}

type exteDataSource struct {
	datasources map[string]qd
}

func (eds *exteDataSource) Get(key string) string {
	return ""
}
func (eds *exteDataSource) GetDS(key string) rior.DataSource {
  q := eds.datasources[key]
	if q.ds_cache == nil {
		q.ds_cache = &riorAdapter{q.q, q.q.Columns, q.ds, make(map[string]rior.DataSource)}
	}
	return q.ds_cache
}

type riorAdapter struct {
	q Query
	cols []string
	ds rior.DataSource
	ds_cache map[string]rior.DataSource
}

func (q *riorAdapter) Get(key string) string {
	for _, col := range q.cols {
	  if col == key {
			return q.ds.Get(key)
		}
	}
	return ""
}
func (q riorAdapter) GetDS(key string) rior.DataSource {
	return emptyDS(false)
}

type emptyDS bool
func (emptyDS) GetDS(key string) rior.DataSource {
	return emptyDS(false)
}
func (emptyDS) Get(key string) string {
	return ""
}


func (cq *ExteQueryr) DoQuery() rior.DataSource {
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
	errs     []error
	db       rior.DB
	ante     ante.Ante
	handlers *[]HandlerEntry
	plugins []Plugin
}

type Plugin interface {
	GetHandlers(ed Extedata, ante ante.Ante, db rior.DB) []HandlerEntry
}

func (e *handlerCollector) collectHandlers(ed Extedata) {
	f, err := os.Open(ed.Template)
	if err != nil {
		e.errs = append(e.errs, err)
		return
	}
	defer f.Close()
	tmplt, err := e.ante.ParseTemplate(f)
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
		*e.handlers = append(*e.handlers, plugin.GetHandlers(ed, e.ante, e.db)...)
	}
}

func CreateHandlers(filename string, db rior.DB, ante ante.Ante, plugins []Plugin) (http.HandlerFunc, error) {
	extedata, err := ParseYaml(filename)
	if err != nil {
		return nil, err
	}
	entries := []HandlerEntry{}
	handlerrs := &handlerCollector{[]error{}, db, ante, &entries, plugins}
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

func CreateHandler(template ante.Template, q Queryr) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		template.Execute(q.DoQuery(), rw)
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
