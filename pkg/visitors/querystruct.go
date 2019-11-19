package visitors

// import (
// 	"github.com/pingcap/parser/ast"
// 	"github.com/pingcap/parser/types"
// )

// // Variable - a name, a type.
// type Variable struct {
// 	Name      string
// 	Type      *types.FieldType
// 	MaybeNull bool
// 	Order     int
// }

// // QueryStructVisitor -
// type QueryStructVisitor struct {
// 	*baseVisitor
// 	Input  []Variable
// 	Output []Variable
// }

// // NewQueryStructVisitor -
// func NewQueryStructVisitor() *QueryStructVisitor {
// 	return &QueryStructVisitor{
// 		baseVisitor: newBaseVisitor("QueryStruct"),
// 	}
// }

// var _ ast.Visitor = &QueryStructVisitor{}

// // Enter - Implements Visitor
// func (s *QueryStructVisitor) Enter(n ast.Node) (ast.Node, bool) {
// 	s.baseVisitor.Enter(n)
// 	switch v := n.(type) {
// 	case *ast.:
// 	}
// 	return n, false
// }

// // Leave - Implements Visitor
// func (s *QueryStructVisitor) Leave(n ast.Node) (ast.Node, bool) {
// 	s.baseVisitor.Leave(n)
// 	return n, true
// }
