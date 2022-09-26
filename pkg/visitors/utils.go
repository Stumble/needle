package visitors

// import (
// 	"github.com/pingcap/tidb/parser/ast"
// )

// // GetTableName - get table name from table refs
// // TODO(yumin): to correctly resolve the name, we need to go through
// // all joined tables, check that the unqualified name is not ambiguous.
// func GetTableName(ref *ast.TableRefsClause) (string, error) {
// 	left := ref.TableRefs.Left
// 	for {
// 		switch v := left.(type) {
// 		case *ast.Join:
// 			left = v.Left
// 		case *ast.TableSource:
// 			name, simple := v.Source.(*ast.TableName)
// 			if !simple {
// 				// not a simple table, not supported for now
// 				return "", NewError(ErrNotSupported, v.Text())
// 			}
// 			return name.Name.String(), nil
// 		}
// 	}
// }
