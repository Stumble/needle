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
	"github.com/rs/zerolog/log"

	"github.com/stumble/needle/pkg/parser"
)

const (
	cacheForever = "forever"
)

var (
	ErrInvalidIdentifier = errors.New("Invalid identifier")
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
	if err := validName(s.Name); err != nil {
		return err
	}
	if err := validName(s.MainObj); err != nil {
		return err
	}
	if s.Name == s.MainObj {
		return errors.New("mainObj name and schema name cannot be the same.")
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
	if err := validName(q.Name); err != nil {
		return fmt.Errorf("invalid query name: %s, because %w", q.Name, err)
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
		log.Warn().Msgf("WARNING: query %s is not cached\n", q.Name)
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
	if err := validName(m.Name); err != nil {
		return fmt.Errorf("invalid mutation name: %s, because %w", m.Name, err)
	}
	return nil
}

// InvalidateQueries - query names
func (m Mutation) InvalidateQueries() []string {
	return commaSplitList(m.InvalidateStr)
}

// parseConfig
func parseConfig(config io.Reader, path string, recursiveImport bool) (*NeedleConfig, error) {
	bytes, err := ioutil.ReadAll(config)
	if err != nil {
		return nil, errorFormatter(path, "load XML", err)
	}
	var data NeedleConfig
	err = xml.Unmarshal(bytes, &data)
	if err != nil {
		return nil, errorFormatter(path, "parse XML", err)
	}

	// validate schema
	err = data.Schema.IsValid()
	if err != nil {
		return nil, errorFormatter(path, "validate schema names", err)
	}

	// import referenced schemas, but do not recursively import all.
	if recursiveImport {
		for i, imp := range data.Schema.Refs {
			src := filepath.Join(filepath.Dir(path), imp.Src)
			importedConf, err := parseConfigFromFileImport(src, false)
			if err != nil {
				return nil, errorFormatter(path, "import referenced schema: "+src, err)
			}
			data.Schema.Refs[i].SQL = importedConf.Schema.SQL
		}
	}

	// validate queries.
	data.Stmts.QueryMap = make(map[string]*Query)
	for i, q := range data.Stmts.Queries {
		if err := q.IsValid(); err != nil {
			return nil, errorFormatter(path,
				fmt.Sprintf("validate %d-th query %s", i, q.Name), err)
		}
		_, has := data.Stmts.QueryMap[q.Name]
		if has {
			return nil, errorFormatter(path, fmt.Sprintf("validate %d-th query %s", i, q.Name),
				errors.New("duplicated query name: "+q.Name))
		}
		data.Stmts.QueryMap[q.Name] = &data.Stmts.Queries[i]
	}

	// validate mutations.
	data.Stmts.MutationMap = make(map[string]*Mutation)
	for i, m := range data.Stmts.Mutations {
		if err := m.IsValid(); err != nil {
			return nil, errorFormatter(path,
				fmt.Sprintf("validate %d-th mutation %s", i, m.Name), err)
		}

		// name check
		_, dupName := data.Stmts.QueryMap[m.Name]
		if dupName {
			return nil, errorFormatter(path, fmt.Sprintf("validate %d-th mutation %s", i, m.Name),
				errors.New("mutation name conflicts with query name: "+m.Name))
		}
		_, dupName = data.Stmts.MutationMap[m.Name]
		if dupName {
			return nil, errorFormatter(path, fmt.Sprintf("validate %d-th mutation %s", i, m.Name),
				errors.New("mutation name conflicts: "+m.Name))
		}
		data.Stmts.MutationMap[m.Name] = &data.Stmts.Mutations[i]

		invalidates := m.InvalidateQueries()
		for _, q := range invalidates {
			query, has := data.Stmts.QueryMap[q]
			if !has {
				return nil, errorFormatter(path, fmt.Sprintf("validate %d-th mutation %s", i, m.Name),
					fmt.Errorf("failed to find the query %s", q))
			}
			if query.CacheDuration() == nil {
				return nil, errorFormatter(path, fmt.Sprintf("validate %d-th mutation %s", i, m.Name),
					fmt.Errorf("query %s in invalidate list is not cached: ", q))
			}
		}
	}

	return &data, nil
}

func parseConfigFromFileImport(path string, recursiveImport bool) (*NeedleConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseConfig(file, path, recursiveImport)
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

func validName(str string) error {
	if len(str) < 2 {
		return fmt.Errorf("%w, must have length must >= 2, but %s is not",
			ErrInvalidIdentifier, str)
	}
	for _, v := range reservedIdentifies {
		if v == str {
			return fmt.Errorf("%s is a reserved identifier", v)
		}
	}
	if !startWithUpper(str) {
		return fmt.Errorf("%w, name must start with an upper-cased letter, but %s is not",
			ErrInvalidIdentifier, str)
	}
	return nil
}

func startWithUpper(str string) bool {
	f := string(str[0])
	return strings.ToUpper(f) == f
}

func errorFormatter(file string, section string, err error) error {
	return fmt.Errorf("Compiling %s, %s, error found: %w", file, section, err)
}
