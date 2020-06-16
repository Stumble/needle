package passes

import (
	"github.com/pingcap/parser/ast"

	"github.com/stumble/needle/pkg/driver"
	"github.com/stumble/needle/pkg/schema"
	"github.com/stumble/needle/pkg/visitors"
	// "github.com/stumble/needle/pkg/utils"
)

// NormalizePass - replace colName with fully qualified names.
type NormalizePass struct {
}

// Run -
func (n NormalizePass) Run(repo *driver.Repo) error {
	// tableNames := collectTableNames(repo.Tables)

	asts := make([]ast.Node, 0)

	for _, q := range repo.Queries {
		asts = append(asts, q.Node)
	}
	for _, m := range repo.Mutations {
		asts = append(asts, m.Node)
	}

	// normalize all statements.
	for _, node := range asts {
		starElim := visitors.NewStarElimVisitor(repo.Tables[0])
		node.Accept(starElim)
		if starElim.Errors() != nil {
			return mergeErrors(starElim.Errors())
		}

		// XXX(yumin): tableAs pass is no longer useful.
		// tableAs := visitors.NewTableAsVisitor(tableNames)
		// node.Accept(tableAs)
		// if tableAs.Errors() != nil {
		// 	return mergeErrors(tableAs.Errors())
		// }

		nameResolve := visitors.NewNameResolveVisitor(repo.Tables)
		node.Accept(nameResolve)
		if nameResolve.Errors() != nil {
			return mergeErrors(nameResolve.Errors())
		}

		typeInference := visitors.NewTypeInferenceVisitor(repo.Tables)
		node.Accept(typeInference)
		if typeInference.Errors() != nil {
			return mergeErrors(typeInference.Errors())
		}
	}
	return nil
}

func collectTableNames(tables []schema.SQLTable) (rst []string) {
	for _, t := range tables {
		rst = append(rst, t.Name())
	}
	return
}
