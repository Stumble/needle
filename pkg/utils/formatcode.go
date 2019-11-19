package utils

import (
	// "bytes"
	"go/format"
	// "go/parser"
	// "go/token"
)

// FormatGoCode - syntax check and format go code.
func FormatGoCode(code string) (string, error) {
	// node, err := parser.Parse(code)
	// if err != nil {
	// 	return "", err
	// }

	// // Create a FileSet for node. Since the node does not come
	// // from a real source file, fset will be empty.
	// fset := token.NewFileSet()

	rst, err := format.Source([]byte(code))
	if err != nil {
		return "", err
	}
	return string(rst), nil
}
