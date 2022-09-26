package visitors

import (
	"github.com/pingcap/tidb/parser/ast"
)

// PrintFunc - print to.
type PrintFunc = func(format string, a ...interface{})

// PrinterVisitor -
type PrinterVisitor struct {
	print  PrintFunc
	format string
}

var _ ast.Visitor = &PrinterVisitor{}

// NewPrinterVisitor -
func NewPrinterVisitor(print PrintFunc, format string) *PrinterVisitor {
	return &PrinterVisitor{
		print:  print,
		format: format,
	}
}

// Enter - Implements Visitor
func (c *PrinterVisitor) Enter(n ast.Node) (ast.Node, bool) {
	c.print(c.format, n)
	return n, false
}

// Leave - Implements Visitor
func (c *PrinterVisitor) Leave(n ast.Node) (ast.Node, bool) {
	return n, true
}
