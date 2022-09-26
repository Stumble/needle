package driver

import (
	"errors"

	"github.com/pingcap/tidb/parser/ast"

	"github.com/stumble/needle/pkg/config"
	"github.com/stumble/needle/pkg/schema"
)

// Query - the stmt node and query.
type Query struct {
	Config *config.Query
	Node   ast.Node
}

// Mutation - the stmt node and mutation.
type Mutation struct {
	Config *config.Mutation
	Node   ast.Node

	Invalidates []*Query
}

// Repo is the root struct.
type Repo struct {
	Tables    []schema.SQLTable
	Queries   []*Query
	Mutations []*Mutation
	Config    *config.NeedleConfig
}

// NewRepoFromConfig - panic if error
func NewRepoFromConfig(config *config.NeedleConfig) (*Repo, error) {
	tables := make([]schema.SQLTable, 0)
	sql, err := tableFromSQL(config.Schema.SQL, config.Schema.HiddenFields())
	if err != nil {
		return nil, err
	}
	tables = append(tables, sql)
	for _, ref := range config.Schema.Refs {
		sql, err := tableFromSQL(ref.SQL, []string{})
		if err != nil {
			return nil, err
		}
		tables = append(tables, sql)
	}

	queries := make([]*Query, 0)
	queryNameObj := make(map[string]*Query)
	for i, s := range config.Stmts.Queries {
		node, err := s.SQL.Parse()
		if err != nil {
			return nil, err
		}
		queries = append(queries, &Query{Config: &config.Stmts.Queries[i], Node: node})
		queryNameObj[s.Name] = queries[len(queries)-1]
	}

	mutations := make([]*Mutation, 0)
	for i, m := range config.Stmts.Mutations {
		node, err := m.SQL.Parse()
		if err != nil {
			return nil, err
		}
		invalidates := make([]*Query, 0)
		for _, qname := range m.InvalidateQueries() {
			q, ok := queryNameObj[qname]
			if !ok {
				return nil, errors.New("query name not exist: " + qname)
			}
			invalidates = append(invalidates, q)
		}
		mutations = append(mutations, &Mutation{
			Config: &config.Stmts.Mutations[i], Node: node, Invalidates: invalidates})
	}

	return &Repo{
		Tables:    tables,
		Queries:   queries,
		Mutations: mutations,
		Config:    config,
	}, nil
}

func tableFromSQL(sql config.SQLStmt, hf []string) (schema.SQLTable, error) {
	tb, err := sql.Parse()
	if err != nil {
		return nil, err
	}
	rst := schema.NewTableInfo(tb.(*ast.CreateTableStmt), hf)
	err = rst.Valid()
	return rst, err
}
