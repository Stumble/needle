package schema

import (
	"github.com/pingcap/tidb/parser/ast"
	// "github.com/pingcap/tidb/parser/types"
)

// IndexInfo holds index info of SQL table.
type IndexInfo struct {
	cons *ast.Constraint
}

// NewIndexInfo -
func NewIndexInfo(cons *ast.Constraint) SQLIndex {
	return &IndexInfo{
		cons: cons,
	}
}

func (s *IndexInfo) IsPrimaryKey() bool {
	return s.cons.Tp == ast.ConstraintPrimaryKey
}

// Name - implements SQLIndex
func (s *IndexInfo) Name() string {
	return s.cons.Name
}

// Keys - implements SQLIndex
func (s *IndexInfo) KeyNames() []string {
	rst := make([]string, 0)
	for _, key := range s.cons.Keys {
		rst = append(rst, key.Column.Name.String())
	}
	return rst
}
