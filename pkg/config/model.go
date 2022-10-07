package config

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tidb/parser/ast"

	"github.com/stumble/needle/pkg/parser"
)

const (
	cacheForever = "forever"
)

// SQLStmt is just a string.
type SQLStmt string

// Parse - yes, parse it.
// Although parser is heavy, needle is not performance sensitive, so...
func (s SQLStmt) Parse() (ast.StmtNode, error) {
	p := parser.NewSQLParser()
	return p.ParseOneStmt(string(s))
}

// NeedleConfig the root structure of a needle xml config file
type NeedleConfig struct {
	XMLName xml.Name `xml:"needle"`
	Schema  Schema   `xml:"schema"`
	Stmts   Stmts    `xml:"stmts"`
}

// Schema schema of this config and imported sources.
type Schema struct {
	HiddenFieldsStr string      `xml:"hiddenFields,attr"`
	Name            string      `xml:"name,attr"`
	MainObj         string      `xml:"mainObj,attr"`
	SQL             SQLStmt     `xml:"sql"`
	Refs            []Reference `xml:"ref"`
}

// HiddenFields return hidden fields of this schema.
func (s Schema) HiddenFields() []string {
	return commaSplitList(s.HiddenFieldsStr)
}

// IsValid return nil if valid.
func (s Schema) IsValid() error {
	if !validName(s.Name) {
		return errors.New("invalid schema name: " + string(s.Name))
	}
	if !validName(s.MainObj) {
		return errors.New("invalid mainObj name: " + string(s.MainObj))
	}
	if s.Name == s.MainObj {
		return errors.New("mainObj name and schema name cannot be the same: " + s.Name)
	}
	return nil
}

// Reference is imported schemas. stmts like join may need stmts from others.
// SQL is set after importing from source.
type Reference struct {
	Src string `xml:"src,attr"`
	SQL SQLStmt
}

// Stmts -
type Stmts struct {
	Queries   []Query    `xml:"query"`
	Mutations []Mutation `xml:"mutation"`

	QueryMap    map[string]*Query
	MutationMap map[string]*Mutation
}

const (
	single = "single"
	many   = "many"
)

// Query is the type of select statement.
type Query struct {
	XMLName          xml.Name `xml:"query"`
	Name             string   `xml:"name,attr"`
	Type             string   `xml:"type,attr"`
	CacheDurationStr string   `xml:"cacheDuration,attr"`
	SQL              SQLStmt  `xml:"sql"`
}

// IsValid nil if Query is valid.
func (q Query) IsValid() error {
	if !validName(q.Name) {
		return errors.New("invalid query name near: " + string(q.SQL) + " name: " + q.Name)
	}
	if !(q.Type == single || q.Type == many) {
		return errors.New("query type illegal:" + q.Name)
	}
	if q.CacheDurationStr != "" && q.CacheDurationStr != cacheForever {
		v, err := time.ParseDuration(q.CacheDurationStr)
		if err != nil {
			return err
		}
		if v <= 0 {
			return errors.New("cache-duration <= 0s is invalid: " + q.CacheDurationStr)
		}
	}
	if q.CacheDurationStr == "" {
		fmt.Printf("WARNING: query %s is not cached\n", q.Name)
	}
	return nil
}

// IsSingleRow whether the result of query is single row or
// many row.
func (q Query) IsSingleRow() bool {
	return q.Type == single
}

// CacheDuration cache duration of query result.
// nil = nocache
// 0 time.Duration indicates cache forever.
func (q Query) CacheDuration() *time.Duration {
	if q.CacheDurationStr == "" {
		return nil
	}
	if q.CacheDurationStr == cacheForever {
		v := time.Duration(0)
		return &v
	}
	d, err := time.ParseDuration(q.CacheDurationStr)
	if err != nil {
		panic(err)
	}
	return &d
}

// Mutation are one of Insert/Update/Delete
type Mutation struct {
	XMLName       xml.Name `xml:"mutation"`
	Name          string   `xml:"name,attr"`
	InvalidateStr string   `xml:"invalidate,attr"`
	SQL           SQLStmt  `xml:"sql"`
}

// IsValid - return nil if valid.
func (m Mutation) IsValid() error {
	if !validName(m.Name) {
		return errors.New("invalid mutation name: " + string(m.SQL))
	}
	return nil
}

// InvalidateQueries - query names
func (m Mutation) InvalidateQueries() []string {
	return commaSplitList(m.InvalidateStr)
}

// parseConfig -
func parseConfig(config io.Reader, path string, doImport bool) (*NeedleConfig, error) {
	bytes, err := ioutil.ReadAll(config)
	if err != nil {
		return nil, err
	}
	var data NeedleConfig
	err = xml.Unmarshal(bytes, &data)
	if err != nil {
		return nil, errors.New("xml unmarshal error: " + err.Error())
	}

	// validate schema
	err = data.Schema.IsValid()
	if err != nil {
		return nil, err
	}

	for i, imp := range data.Schema.Refs {
		src := filepath.Join(filepath.Dir(path), imp.Src)
		// one layer import, do not recursively import because people can write cyclic deps.
		importedConf, err := parseConfigFromFileImport(src, false)
		if err != nil {
			return nil, err
		}
		data.Schema.Refs[i].SQL = importedConf.Schema.SQL
	}

	// validate queries.
	data.Stmts.QueryMap = make(map[string]*Query)
	for i, q := range data.Stmts.Queries {
		err := q.IsValid()
		if err != nil {
			return nil, err
		}
		_, has := data.Stmts.QueryMap[q.Name]
		if has {
			return nil, errors.New("duplicated query name: " + q.Name)
		}
		data.Stmts.QueryMap[q.Name] = &data.Stmts.Queries[i]
	}

	// validate mutations.
	data.Stmts.MutationMap = make(map[string]*Mutation)
	for i, m := range data.Stmts.Mutations {
		err := m.IsValid()
		if err != nil {
			return nil, err
		}

		// name check
		_, dupName := data.Stmts.QueryMap[m.Name]
		if dupName {
			return nil, errors.New("mutation name conflicts with query name: " + m.Name)
		}
		_, dupName = data.Stmts.MutationMap[m.Name]
		if dupName {
			return nil, errors.New("mutation name conflicts: " + m.Name)
		}
		data.Stmts.MutationMap[m.Name] = &data.Stmts.Mutations[i]

		invalidates := m.InvalidateQueries()
		for _, q := range invalidates {
			query, has := data.Stmts.QueryMap[q]
			if !has {
				return nil, errors.New("'invalidate query' not found: " + q + " in " + m.Name)
			}
			if query.CacheDuration() == nil {
				return nil, errors.New(
					"a query in invalidate list is not cached: " + q + " in " + m.Name)
			}
		}
	}

	return &data, nil
}

func parseConfigFromFileImport(path string, doImport bool) (*NeedleConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseConfig(file, path, doImport)
}

// ParseConfigFromFile -
func ParseConfigFromFile(path string) (*NeedleConfig, error) {
	return parseConfigFromFileImport(path, true)
}

func commaSplitList(str string) []string {
	strs := strings.Split(strings.Trim(strings.TrimSpace(str), ","), ",")
	for i := range strs {
		strs[i] = strings.TrimSpace(strs[i])
	}
	if len(strs) == 1 && strs[0] == "" {
		return []string{}
	}
	return strs
}

var reservedIdentifies = []string{
	"Check",
}

func validName(str string) bool {
	if len(str) < 2 {
		return false
	}
	for _, v := range reservedIdentifies {
		if v == str {
			return false
		}
	}
	return startWithUpper(str)
}

func startWithUpper(str string) bool {
	f := string(str[0])
	return strings.ToUpper(f) == f
}
