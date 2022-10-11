package schema

import (
	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/types"
)

// SQLTable represents a sql table schema, immutable.
type SQLTable interface {
	Name() string
	Valid() error
	StarColumns() []SQLColumn
	Columns() []SQLColumn
	Indexes() []SQLIndex
	SQL() string
}

// SQLColumn is a sql defined column, immutable.
type SQLColumn interface {
	Name() string
	NameExpr() *ast.ColumnNameExpr
	NotNull() bool
	Type() *types.FieldType
	GoType() GoType
}

// SQLIndex - index only, immutable.
type SQLIndex interface {
	Name() string
	IsPrimaryKey() bool
	KeyNames() []string
}

// GoTypeName is the name of the type that can be used in golang.
type GoTypeName = string

const (
	// GoTypeInt int64
	GoTypeInt GoTypeName = "int64"
	// GoTypeFloat64 float64
	GoTypeFloat64 GoTypeName = "float64"
	// GoTypeString string
	GoTypeString GoTypeName = "string"
	// GoTypeTime time
	GoTypeTime GoTypeName = "time"
	// GoTypeBool bool
	GoTypeBool GoTypeName = "bool"
	// GoTypeJson json.RawMessage
	GoTypeJson GoTypeName = "RawMessage"
)

// GoType is the type that can be used in golang.
type GoType struct {
	Type    GoTypeName
	NotNull bool
}

func (g GoType) String() string {
	if g.NotNull {
		return g.Type
	}
	return "*" + g.Type
}
