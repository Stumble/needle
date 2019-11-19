package schema

import (
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/types"
)

// EvalTypeToGoType - eval
func EvalTypeToGoType(t *types.FieldType) GoType {
	return GoType{
		Type:    EvalTypeToGoTypeName(t),
		NotNull: mysql.HasNotNullFlag(t.Flag),
	}
}

// EvalTypeToGoTypeName - eval
func EvalTypeToGoTypeName(t *types.FieldType) GoTypeName {
	// XXX(yumin): the first cond is a kind of hack, should visit this part later.
	if (t.Tp == mysql.TypeTiny && t.Flen == 1) || mysql.HasIsBooleanFlag(t.Flag) {
		return GoTypeBool
	}
	et := t.EvalType()
	switch et {
	case types.ETInt:
		return GoTypeInt
	case types.ETReal:
		return GoTypeFloat64
	case types.ETDatetime:
		return GoTypeTime
	case types.ETString:
		return GoTypeString
	case types.ETJson:
		return GoTypeJson
	}
	panic("unsupported type: " + t.String())
}
