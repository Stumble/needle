package utils

import (
	"strings"

	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/format"
)

// RestoreNode return a node to string.
func RestoreNode(n ast.Node) string {
	var sb strings.Builder
	formatCtx := format.NewRestoreCtx(
		format.RestoreKeyWordUppercase|format.RestoreStringSingleQuotes, &sb)
	err := n.Restore(formatCtx)
	if err != nil {
		panic(err)
	}
	return sb.String()
}
