package visitors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/mysql"
	"github.com/pingcap/tidb/parser/opcode"
	"github.com/pingcap/tidb/parser/types"
	drivertypes "github.com/pingcap/tidb/types"
	driver "github.com/pingcap/tidb/types/parser_driver"

	// "github.com/stumble/needle/pkg/parser"
	"github.com/stumble/needle/pkg/schema"
	"github.com/stumble/needle/pkg/utils"
)

type columnRef struct {
	name       string // name may be aliased by as clause.
	nullable   bool   // additional nullable may be applied by left/right/outer join.
	col        schema.SQLColumn
	tmpVarType *types.FieldType
}

func (r columnRef) t() *types.FieldType {
	var tp *types.FieldType
	if r.col != nil {
		tp = r.col.Type()
	}
	if r.tmpVarType != nil {
		tp = r.tmpVarType
	}
	if tp == nil {
		panic(fmt.Errorf("type nil: %+v", r))
	}
	if r.nullable {
		return nullClone(tp)
	}
	return tp.Clone()
}

type refStack struct {
	stack [][]columnRef
	dict  map[string]([]columnRef)
}

func newRefStack() refStack {
	return refStack{
		stack: nil,
		dict:  make(map[string]([]columnRef)),
	}
}

func (r *refStack) PushNames(refs ...columnRef) {
	r.stack = append(r.stack, refs)
	for _, ref := range refs {
		if ref.col == nil && ref.tmpVarType == nil {
			panic(fmt.Errorf("invalid push: %+v", ref))
		}
		r.dict[ref.name] = append(r.dict[ref.name], ref)
	}
}

func (r *refStack) PopNames() {
	if len(r.stack) == 0 {
		panic("pop on empty stack")
	}
	top := r.stack[len(r.stack)-1]
	for _, ref := range top {
		val := r.dict[ref.name]
		r.dict[ref.name] = val[:len(val)-1]
	}
	r.stack = r.stack[:len(r.stack)-1]
}

func (r *refStack) Lookup(name string) (columnRef, bool) {
	if refs, ok := r.dict[name]; ok {
		return refs[len(refs)-1], true
	}
	return columnRef{}, false
}

// TypeInferenceVisitor - assign types to terms.
// premise: all column names are fully qualified.
// after: all ColumnNameExpr will have its type field set to be the type defined
//
//	in the schema file. ColumnNameExpr in select will have its
//	type field set to column's type, function call term's type will resolve to
//	the type of the return value.
//	Terms as input(driver.ParamMarkerExpr), will have its type,
//	inferred from binary operations, assignments...
type TypeInferenceVisitor struct {
	*baseVisitor
	DBInfo []schema.SQLTable

	tableMap map[string]schema.SQLTable
	refStack refStack
}

var _ ast.Visitor = &TypeInferenceVisitor{}

// NewTypeInferenceVisitor - schema is used for column's type.
func NewTypeInferenceVisitor(dbs []schema.SQLTable) *TypeInferenceVisitor {
	tableMap := make(map[string]schema.SQLTable)
	for _, table := range dbs {
		tableMap[table.Name()] = table
	}
	return &TypeInferenceVisitor{
		baseVisitor: newBaseVisitor("TypeInference"),
		DBInfo:      dbs,
		tableMap:    tableMap,
		refStack:    newRefStack(),
	}
}

func makeFQColName(tb string, col string) string {
	return tb + "." + col
}

func (t *TypeInferenceVisitor) typeLookup(nameExpr *ast.ColumnName) (*types.FieldType, bool) {
	tb := nameExpr.Table.String()
	name := makeFQColName(tb, nameExpr.Name.String())
	colRef, ok := t.refStack.Lookup(name)
	if !ok {
		return nil, false
	}
	return colRef.t(), true
}

// Enter - Implements Visitor
func (t *TypeInferenceVisitor) Enter(n ast.Node) (ast.Node, bool) {
	t.baseVisitor.Enter(n)
	switch v := n.(type) {
	case *ast.UpdateStmt:
		err := t.pushColumnRefs(v.TableRefs)
		if err != nil {
			t.AppendErr(err.(Error))
			return n, true
		}
	case *ast.DeleteStmt:
		err := t.pushColumnRefs(v.TableRefs)
		if err != nil {
			t.AppendErr(err.(Error))
			return n, true
		}
	case *ast.InsertStmt:
		err := t.pushColumnRefs(v.Table)
		if err != nil {
			t.AppendErr(err.(Error))
			return n, true
		}
		if v.Lists == nil || len(v.Lists) != 1 {
			t.AppendErr(NewErrorf(ErrNotSupported,
				"insert not supported, multiple values: %s", utils.RestoreNode(n)))
			return n, true
		}
		params := v.Lists[0]
		cols := v.Columns
		for i := range params {
			coltype, ok := t.typeLookup(cols[i])
			if !ok {
				t.AppendErr(NewErrorf(ErrInvalidExpr, "column not defined: %s", cols[i].Name))
				return n, true
			}
			// nullable input parameter.
			params[i].SetType(coltype)
		}
	case *ast.SelectStmt:
		err := t.pushColumnRefs(v.From)
		if err != nil {
			t.AppendErr(err.(Error))
			return n, true
		}
	}
	return n, false
}

func (t *TypeInferenceVisitor) popColumnRefs() {
	t.refStack.PopNames()
}

func (t *TypeInferenceVisitor) pushColumnRefs(tref *ast.TableRefsClause) error {
	columnRefs, ok := t.makeColumnRefs(tref)
	if !ok {
		return NewErrorf(ErrTypeCheck, "failed to find table def: %s", utils.RestoreNode(tref))
	}
	t.refStack.PushNames(columnRefs...)
	return nil
}

func (t *TypeInferenceVisitor) makeColumnRefs(tref *ast.TableRefsClause) ([]columnRef, bool) {
	if tref == nil {
		return nil, true
	}
	join := tref.TableRefs
	left := join.Left
	right := join.Right
	var joinVec = []ast.ResultSetNode{left, right}
	var nullableVec []bool // [left, right]
	switch join.Tp {
	case 0:
		nullableVec = []bool{false, false}
	case ast.CrossJoin:
		nullableVec = []bool{false, false}
	case ast.LeftJoin:
		nullableVec = []bool{false, true}
	case ast.RightJoin:
		nullableVec = []bool{true, false}
	}

	var rst []columnRef
	for i, resultSetNode := range joinVec {
		if resultSetNode == nil {
			continue
		}
		nullable := nullableVec[i]
		switch v := resultSetNode.(type) {
		case *ast.TableSource:
			switch src := v.Source.(type) {
			case *ast.TableName:
				asname := v.AsName.String()
				tablename := src.Name.String()
				if asname == "" {
					asname = tablename
				}
				table, ok := t.tableMap[tablename]
				if !ok {
					t.AppendErr(NewErrorf(ErrInvalidExpr,
						"table definition not found: %s", tablename))
					return nil, false
				}
				for _, col := range table.Columns() {
					rst = append(rst, columnRef{
						name:     makeFQColName(asname, col.Name()),
						nullable: nullable,
						col:      col,
					})
				}
			case *ast.SelectStmt:
				// in this case, actually we need a control-flow-visitor where
				// select * from XXX, XXX is visited before select so that XXX has
				// been type-checked. However, since we don't have it, plus that
				// this IR is mutable, we can hack in the following way.
				asname := v.AsName.String()
				if asname == "" {
					t.AppendErr(NewErrorf(ErrInvalidExpr,
						"subquery not aliased: %s", utils.RestoreNode(src)))
					return nil, false
				}
				subVisitor := NewTypeInferenceVisitor(t.DBInfo)
				subVisitor.DisableLogging(true)
				src.Accept(subVisitor)
				if subVisitor.Errors() != nil {
					for _, e := range subVisitor.Errors() {
						t.AppendErr(e.(Error))
					}
					return nil, false
				}
				for _, field := range src.Fields.Fields {
					if field.Expr.GetType().GetType() == mysql.TypeUnspecified {
						t.AppendErr(NewErrorf(ErrTypeCheck,
							"failed to type-check: %s", utils.RestoreNode(field)))
						return nil, false
					}
					tempRef := columnRef{
						name:       makeFQColName(asname, field.AsName.String()),
						nullable:   nullable,
						tmpVarType: field.Expr.GetType().Clone(),
					}
					t.LogInfo("adding temp ref: %+v", tempRef)
					rst = append(rst, tempRef)
				}
			default:
				// not a simple table, not supported for now
				t.AppendErr(NewError(ErrNotSupported, utils.RestoreNode(v)))
				return nil, false
			}
		case *ast.Join:
			refs, found := t.makeColumnRefs(&ast.TableRefsClause{TableRefs: v})
			if found {
				rst = append(rst, refs...)
			} else {
				return nil, false
			}
		default:
			t.AppendErr(NewErrorf(ErrNotSupported, "%s", utils.RestoreNode(v)))
		}
	}
	return rst, true
}

// Leave - Implements Visitor
// premise: visitors visit left hand size of binary operation first.
func (t *TypeInferenceVisitor) Leave(n ast.Node) (ast.Node, bool) {
	t.baseVisitor.Leave(n)
	switch v := n.(type) {
	case *ast.SelectStmt, *ast.DeleteStmt, *ast.UpdateStmt, *ast.InsertStmt:
		t.popColumnRefs()
	case *ast.ColumnNameExpr:
		coltype, ok := t.typeLookup(v.Name)
		if !ok {
			t.AppendErr(NewErrorf(ErrInvalidExpr, "column not defined: %s", v.Name))
			return n, true
		}
		v.SetType(coltype)
	case ast.ParamMarkerExpr: // interface
		// type inferenced in enter, skipped.
		if v.GetType().GetType() != mysql.TypeUnspecified {
			break
		}
		bop, ok := t.FindInCtxAnyOf(
			(*ast.Limit)(nil),
			(*ast.PatternInExpr)(nil),
			(*ast.PatternLikeExpr)(nil),
			(*ast.BetweenExpr)(nil),
			(*ast.BinaryOperationExpr)(nil),
			(*ast.Assignment)(nil),
		)
		if !ok {
			t.AppendErr(NewErrorf(ErrInvalidExpr, "ParamMarker type cannot be inferred: %s",
				utils.RestoreNode(n)))
			return n, true
		}
		switch op := bop.(type) {
		case *ast.BinaryOperationExpr:
			v.SetType(notNullClone(op.L.GetType()))
		case *ast.PatternInExpr:
			v.SetType(notNullClone(op.Expr.GetType()))
		case *ast.PatternLikeExpr:
			v.SetType(notNullClone(op.Expr.GetType()))
		case *ast.BetweenExpr:
			v.SetType(notNullClone(op.Expr.GetType()))
		case *ast.Limit:
			v.SetType(newNotNullIntType())
		case *ast.Assignment:
			// nullable input parameter.
			coltype, ok := t.typeLookup(op.Column)
			if ok {
				if v.GetType().GetType() == mysql.TypeUnspecified {
					v.SetType(coltype)
				} else {
					// TODO(yumin): this part may be wrong, type.Equal is overly restricted,
					// maybe evaluted type equal is enough.
					if !v.GetType().Equal(coltype) {
						t.AppendErr(NewErrorf(ErrTypeCheck,
							"SET type check failed, lhs = %s, rhs = %s: %s",
							coltype, v.GetType(), utils.RestoreNode(v)))
						return n, true
					}
				}
			}
		}
		if v.GetType().GetType() == mysql.TypeUnspecified {
			t.AppendErr(NewErrorf(ErrInvalidExpr, "ParamMarker type cannot be inferred: %s",
				utils.RestoreNode(n)))
			return n, true
		}
	case *ast.PatternInExpr:
		// XXX(yumin): parser does not parse `in` as a b-op.
		// so we type check it here, not in bop.
		if len(v.List) > 0 {
			if !v.Expr.GetType().Equal(v.List[0].GetType()) {
				t.AppendErr(NewErrorf(ErrTypeCheck, "In type mismatch(%s, %s): %s",
					v.Expr.GetType(),
					v.List[0].GetType(),
					utils.RestoreNode(n)))
			}
		}
		v.SetType(newBoolType())
	case *ast.AggregateFuncExpr:
		atype, err := aggregateFuncTypeInfer(v)
		if err != nil {
			t.AppendErr(err.(Error))
		} else {
			v.SetType(atype)
		}
	case *ast.BinaryOperationExpr:
		target, err := bopTypeCheck(v)
		if err != nil {
			t.AppendErr(NewErrorf(ErrTypeCheck,
				"BinOp %s: %s", err.Error(), utils.RestoreNode(n)))
		} else {
			v.SetType(target.Clone())
		}
	case *ast.FuncCallExpr:
		// TODO(yumin): Support more functions.
		// For function calls we handled some types.
		switch v.FnName.L {
		case ast.Coalesce:
			// user MUST PLACE the type in the order of (nullable, nullable, notnullable)?
			if len(v.Args) == 0 {
				t.AppendErr(NewErrorf(ErrTypeCheck,
					"invoke %s with zero arguments", ast.Coalesce))
			} else {
				lastType := v.Args[len(v.Args)-1].GetType().Clone()
				t.LogWarn("partial support of coalesce function %s, "+
					"type resolve to the last parameter: %s, notnull: %t",
					utils.RestoreNode(v), lastType, (lastType.GetFlag()&mysql.NotNullFlag) != 0)
				v.SetType(lastType)
			}
		case ast.AddDate, ast.DateAdd, ast.Date:
			v.SetType(v.Args[0].GetType().Clone())
		case ast.UTCTimestamp:
			v.SetType(newNotNullDatetimeType())
		case ast.LastInsertId:
			v.SetType(newNotNullIntType())
		case ast.Curdate, ast.Now:
			v.SetType(newNotNullDatetimeType())
		default:
			if len(v.Args) >= 1 {
				defaultType := v.Args[0].GetType()
				v.SetType(defaultType.Clone())
				t.LogWarn("unsupported function: %s, type-check defaults to: %s",
					v.FnName.L, defaultType)
			} else {
				t.AppendErr(NewErrorf(ErrTypeCheck, "cannot infer type of %s", v.FnName.L))
			}
		}
	case *ast.FuncCastExpr:
		switch v.FunctionType {
		case ast.CastFunction:
			v.SetType(v.Tp.Clone())
		case ast.CastConvertFunction:
			v.SetType(v.Expr.GetType().Clone())
		case ast.CastBinaryOperator:
			v.SetType(v.Expr.GetType().Clone())
		}
	case *ast.IsNullExpr:
		v.SetType(newBoolType())
	case *ast.ParenthesesExpr:
		v.SetType(v.Expr.GetType().Clone())
	case ast.ValueExpr:
		// XXX(yumin): parse does not mark consts as nonnull. Need to mark it here.
		if value, ok := v.(*driver.ValueExpr); ok {
			if value.Kind() != drivertypes.KindNull {
				v.SetType(notNullClone(v.GetType()))
			}
		}
	}

	return n, true
}

// // Equal checks whether two FieldType objects are equal.
// func DebugEqual(ft, other *types.FieldType) bool {
// 	// We do not need to compare whole `ft.Flag == other.Flag` when wrapping cast upon an Expression.
// 	// but need compare unsigned_flag of ft.Flag.
// 	partialEqual := ft.Tp == other.Tp &&
// 		ft.Flen == other.Flen &&
// 		ft.Decimal == other.Decimal &&
// 		ft.Charset == other.Charset &&
// 		ft.Collate == other.Collate &&
// 		mysql.HasUnsignedFlag(ft.Flag) == mysql.HasUnsignedFlag(other.Flag)
// 	if !partialEqual || len(ft.Elems) != len(other.Elems) {
// 		fmt.Println(
// 			ft.Decimal, other.Decimal,
// 			ft.Charset, other.Charset,
// 			ft.Collate, other.Collate,
// 		)
// 		return false
// 	}
// 	for i := range ft.Elems {
// 		if ft.Elems[i] != other.Elems[i] {
// 			return false
// 		}
// 	}
// 	return true
// }

func bopTypeCheck(bop *ast.BinaryOperationExpr) (*types.FieldType, error) {
	lt := bop.L.GetType().Clone()
	rt := bop.R.GetType().Clone()
	// remove binary flag.
	if mysql.HasBinaryFlag(rt.GetFlag()) {
		rt.ToggleFlag(mysql.BinaryFlag)
	}
	if lt == nil || rt == nil {
		return nil, errors.New("subterm type not resolved")
	}
	if !lt.Equal(rt) {
		// mysql implicit type conversions.
		etleft := lt.EvalType()
		etright := rt.EvalType()
		// XXX(yumin): introduce a new assumption that on implicit mysql type conversion
		// the return type is converted back to the left hand side expression type. This
		// is not correct but a work around for SET X = X + 1 case for now.
		if etleft != etright {
			return nil, fmt.Errorf("subterm type not equal: (%s, %s)", lt.String(), rt.String())
			// return lt, nil
		}
	}

	if isAnyOf(bop.Op, []opcode.Op{
		opcode.LeftShift,
		opcode.RightShift,
		opcode.Plus,
		opcode.Minus,
		opcode.And,
		opcode.Or,
		opcode.Mod,
		opcode.Xor,
		opcode.Div,
		opcode.Mul,
		opcode.BitNeg,
		opcode.IntDiv,
	}) {
		et := lt.EvalType()
		if !(et == types.ETInt || et == types.ETReal || et == types.ETDecimal) {
			return nil, errors.New("algorithmatic b-op on non-numbers")
		}
		return lt, nil
	}
	// all other b-op produce a boolean variable.
	return newBoolType(), nil
}

// nullClone returns a type with NotNullFlag to be false.
func nullClone(t *types.FieldType) *types.FieldType {
	tp := t.Clone()
	tp.AndFlag(^mysql.NotNullFlag)
	return tp
}

// nullClone returns a type with NotNullFlag to be true.
func notNullClone(t *types.FieldType) *types.FieldType {
	tp := t.Clone()
	tp.AddFlag(mysql.NotNullFlag)
	return tp
}

func newNotNullIntType() *types.FieldType {
	rst := types.NewFieldType(mysql.TypeLong)
	rst.AddFlag(mysql.NotNullFlag)
	return rst
}

func newNotNullDatetimeType() *types.FieldType {
	rst := types.NewFieldType(mysql.TypeDatetime)
	rst.AddFlag(mysql.NotNullFlag)
	return rst
}

func newBoolType() *types.FieldType {
	rst := types.NewFieldType(mysql.TypeTiny)
	rst.AddFlag(mysql.IsBooleanFlag)
	return rst
}

func isAnyOf(op opcode.Op, ops []opcode.Op) bool {
	for i := range ops {
		if op == ops[i] {
			return true
		}
	}
	return false
}

// return cloned return type of aggregate func.
func aggregateFuncTypeInfer(f *ast.AggregateFuncExpr) (*types.FieldType, error) {
	if len(f.Args) < 1 {
		return nil, NewErrorf(ErrInvalidExpr,
			"Arguments missiong in: %s", utils.RestoreNode(f))
	}
	switch strings.ToLower(f.F) {
	case ast.AggFuncCount:
		// count will never return null.
		return newNotNullIntType(), nil
	case ast.AggFuncSum, ast.AggFuncMax, ast.AggFuncMin:
		t := f.Args[0].GetType().Clone()
		// sum, max, and min function will return null when there's no matching rows.
		t.AndFlag(^mysql.NotNullFlag)
		return t, nil
	case ast.AggFuncAvg, ast.AggFuncVarPop, ast.AggFuncVarSamp, ast.AggFuncStddevPop, ast.AggFuncStddevSamp:
		return types.NewFieldType(mysql.TypeFloat), nil
	default:
		return nil, NewErrorf(ErrCompilerError,
			"Unsupported aggregate func: %s", utils.RestoreNode(f))
	}
}
