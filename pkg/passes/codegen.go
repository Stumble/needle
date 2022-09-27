package passes

import (
	"fmt"
	"strings"

	"github.com/stumble/needle/pkg/codegen"
	"github.com/stumble/needle/pkg/driver"
	"github.com/stumble/needle/pkg/schema"
	"github.com/stumble/needle/pkg/utils"
	"github.com/stumble/needle/pkg/visitors"
	"github.com/stumble/needle/pkg/vcs"
)

// GoParam -
type GoParam = visitors.GoParam

// GoVar -
type GoVar = visitors.GoVar

// QuerySocket the gateway between go code and sql query.
type QuerySocket struct {
	Query  *driver.Query
	Params []GoParam
	Output []GoVar
}

// MutationSocket the gateway between go code and sql mutation.
type MutationSocket struct {
	Mutation *driver.Mutation
	Params   []GoParam
}

// CodegenPass - prepare for codegen.
type CodegenPass struct {
	Code string
}

// GenQuerySockets for queries.
func (c *CodegenPass) GenQuerySockets(queries []*driver.Query) ([]QuerySocket, error) {
	querySockets := make([]QuerySocket, 0)
	for i, q := range queries {
		paramExtract := visitors.NewParamExtractVisitor()
		q.Node.Accept(paramExtract)
		if paramExtract.Errors() != nil {
			return nil, mergeErrors(paramExtract.Errors())
		}

		outputExtract := visitors.NewOutputExtractVisitor()
		q.Node.Accept(outputExtract)
		if outputExtract.Errors() != nil {
			return nil, mergeErrors(outputExtract.Errors())
		}

		querySockets = append(querySockets, QuerySocket{
			Query:  queries[i],
			Params: paramExtract.Params,
			Output: outputExtract.Output,
		})
	}
	return querySockets, nil
}

// GenMutationSockets for mutations.
func (c *CodegenPass) GenMutationSockets(mutations []*driver.Mutation) ([]MutationSocket, error) {
	sockets := make([]MutationSocket, 0)
	for i, q := range mutations {
		paramExtract := visitors.NewParamExtractVisitor()
		q.Node.Accept(paramExtract)
		if paramExtract.Errors() != nil {
			return nil, mergeErrors(paramExtract.Errors())
		}

		sockets = append(sockets, MutationSocket{
			Mutation: mutations[i],
			Params:   paramExtract.Params,
		})
	}
	return sockets, nil
}

// GenQueryFuncs from query sockets.
func (c *CodegenPass) GenQueryFuncs(mainStruct *codegen.GoStruct, mainTable schema.SQLTable,
	querySockets []QuerySocket) (queryFuncs []*codegen.QueryFunc) {
	for _, query := range querySockets {
		queryName := query.Query.Config.Name
		var outputStruct *codegen.GoStruct
		if canStarCoverOutput(mainTable, query.Output) {
			outputStruct = mainStruct
		} else {
			outputStruct = GenOutputStruct(queryName+"Rst", query.Output)
		}
		// XXX(yumin): MYSQL does not allow value = NULL, must use 'is NULL'.
		// so arguments cannot be null.
		inputStruct := GenInputStruct(queryName+"Args", query.Params)
		queryFuncs = append(queryFuncs, &codegen.QueryFunc{
			Name:          queryName,
			SQL:           utils.RestoreNode(query.Query.Node),
			CacheDuration: query.Query.Config.CacheDuration(),
			Input:         inputStruct,
			Output:        outputStruct,
			IsList:        !query.Query.Config.IsSingleRow(),
		})
	}
	return
}

// GenMutationFuncs from mutation sockets.
func (c *CodegenPass) GenMutationFuncs(
	mainStruct *codegen.GoStruct, mainTable schema.SQLTable,
	mutationSockets []MutationSocket,
	queryFuncs []*codegen.QueryFunc) (rst []*codegen.MutationFunc) {
	queryMap := make(map[string]*codegen.QueryFunc)
	for i, query := range queryFuncs {
		queryMap[query.Name] = queryFuncs[i]
	}

	for _, mutation := range mutationSockets {
		name := mutation.Mutation.Config.Name

		// XXX(yumin): add this part for insert.
		var params *codegen.GoStruct
		if canStarCoverInput(mainTable, mutation.Params) {
			params = mainStruct
		} else {
			params = GenInputStruct(name+"Args", mutation.Params)
		}

		invalidateParams := make([]*codegen.QueryFunc, 0)
		for _, invalidate := range mutation.Mutation.Invalidates {
			query, ok := queryMap[invalidate.Config.Name]
			if !ok {
				panic("compiler error: invalidate not exists")
			}
			invalidateParams = append(invalidateParams, query)
		}
		rst = append(rst, &codegen.MutationFunc{
			Name:        name,
			SQL:         utils.RestoreNode(mutation.Mutation.Node),
			Input:       params,
			Invalidates: invalidateParams,
		})
	}
	return
}

// Run -
func (c *CodegenPass) Run(repo *driver.Repo) error {
	querySockets, err := c.GenQuerySockets(repo.Queries)
	if err != nil {
		return err
	}
	mutationSockets, err := c.GenMutationSockets(repo.Mutations)
	if err != nil {
		return err
	}

	// the main struct
	mainStruct := GenMainStruct(repo.Tables[0], repo.Config.Schema.MainObj)
	queryFuncs := c.GenQueryFuncs(mainStruct, repo.Tables[0], querySockets)
	mutationFuncs := c.GenMutationFuncs(mainStruct, repo.Tables[0], mutationSockets, queryFuncs)

	// building templates.
	mainName := repo.Config.Schema.Name
	pkgName := strings.ToLower(mainName) + "repo"
	interfaceName := mainName
	repoName := strings.ToLower(mainName[0:1]) + mainName[1:]

	signatures := make([]string, 0)
	for _, query := range queryFuncs {
		signatures = append(signatures, query.Name+query.Signature())
	}
	for _, mutation := range mutationFuncs {
		signatures = append(signatures, mutation.Name+mutation.Signature())
	}

	sqlStmtDecls := make([]codegen.SQLStatementDecl, 0)

	queriesStr := make([]string, 0)
	for _, query := range queryFuncs {
		var builder strings.Builder
		builder.WriteString(query.Input.String() + "\n")
		builder.WriteString(query.Input.KeyFunc(query.Name) + "\n")
		builder.WriteString(query.Input.ArglistFunc() + "\n")
		if query.Output != mainStruct {
			builder.WriteString(query.Output.String() + "\n")
			builder.WriteString(query.Output.ScanFunc() + "\n")
		}
		decl := codegen.SQLStatementDecl{
			VarName:    query.Name + "Stmt",
			EscapedSQL: query.SQL,
		}
		sqlStmtDecls = append(sqlStmtDecls, decl)
		tmpl := codegen.QueryFuncTemplate{
			RepoName:        repoName,
			QueryName:       query.Name,
			QuerySig:        query.Signature(),
			QueryInnerSig:   query.SignatureInnerFunc(),
			HiddenQueryName: strings.ToLower(query.Name[0:1]) + query.Name[1:],
			RstTypeName:     query.Output.Name,
			CacheDuration:   query.CacheDuration,
			SQLVarName:      decl.VarName,
			IsList:          query.IsList,
		}
		if query.Input.IsEmpty() {
			tmpl.InitArgsType = query.Input.Name
		}
		funcs, err := tmpl.Generate()
		if err != nil {
			panic(err)
		}
		builder.WriteString(funcs)
		queriesStr = append(queriesStr, builder.String())
	}

	mutationsStr := make([]string, 0)
	for _, mutation := range mutationFuncs {
		var builder strings.Builder

		invTemps := make([]codegen.InvalidateTemplate, 0)
		for i, inv := range mutation.Invalidates {
			invTemps = append(invTemps, codegen.InvalidateTemplate{
				ArgName:       fmt.Sprintf("key%d", i),
				ValName:       fmt.Sprintf("val%d", i),
				CacheDuration: *inv.CacheDuration,
			})
		}

		// XXX(yumin): support insert main object case.
		if mutation.Input != mainStruct {
			builder.WriteString(mutation.Input.String() + "\n")
			builder.WriteString(mutation.Input.ArglistFunc() + "\n")
		}
		decl := codegen.SQLStatementDecl{
			VarName:    mutation.Name + "Stmt",
			EscapedSQL: mutation.SQL,
		}
		sqlStmtDecls = append(sqlStmtDecls, decl)

		tmpl := codegen.MutationFuncTemplate{
			RepoName:     repoName,
			MutationName: mutation.Name,
			MutationSig:  mutation.Signature(),
			SQLVarName:   decl.VarName,
			Invalidates:  invTemps,
		}
		funcs, err := tmpl.Generate()
		if err != nil {
			panic(err)
		}
		builder.WriteString(funcs)
		mutationsStr = append(mutationsStr, builder.String())
	}

	mainStructStr := mainStruct.String() + "\n" +
		"// nolint: unused\n" + mainStruct.ScanFunc() + "\n" +
		"// nolint: unused\n" + mainStruct.ArglistFunc() + "\n"

	template := codegen.RepoTemplate{
		NeedleVersion:       vcs.Commit,
		TableSchema:         repo.Tables[0].SQL(),
		PkgName:             pkgName,
		InterfaceName:       interfaceName,
		InterfaceSignatures: signatures,
		RepoName:            repoName,
		Statements:          sqlStmtDecls,
		MainStruct:          mainStructStr,
		Queries:             queriesStr,
		Mutations:           mutationsStr,
	}

	code, err := template.Generate()
	if err != nil {
		panic(err)
	}
	code, err = utils.FormatGoCode(code)
	if err != nil {
		// fmt.Println(code)
		panic("[CompilerError] code syntax error: " + err.Error())
	}
	c.Code = code
	return nil
}

// GenMainStruct - generate main struct.
func GenMainStruct(tb schema.SQLTable, name string) *codegen.GoStruct {
	rst := codegen.GoStruct{Name: name}
	for _, col := range tb.StarColumns() {
		ft := calcFieldType(col.GoType(), false)
		rst.Fields = append(rst.Fields, codegen.NewGoField(
			strings.Title(col.Name()), ft, ""))
	}
	rst.Comments = "the main struct."
	return &rst
}

// GenOutputStruct generate output structs
// use main object when output is it.
// introduce table name if fields have name conflicts.
func GenOutputStruct(outputName string, output []GoVar) *codegen.GoStruct {
	rst := codegen.GoStruct{Name: outputName}
	names := make(map[string]int)
	for _, v := range output {
		names[v.Name]++
	}
	for _, v := range output {
		ft := calcFieldType(v.Type, false)
		nm := strings.Title(v.Name)
		if names[v.Name] > 1 {
			nm = strings.Title(v.TableName) + nm
		}
		rst.Fields = append(rst.Fields, codegen.NewGoField(nm, ft, ""))
	}
	return &rst
}

// GenInputStruct - generate input struct
// 1. introduce table name when name conflicts.
// 2. append numbers on fields when they still conflict.
// 3. add "List" suffix on lists params.
func GenInputStruct(inputName string, params []GoParam) *codegen.GoStruct {
	rst := codegen.GoStruct{Name: inputName}
	names := make(map[string]int)
	for _, v := range params {
		nm := strings.Title(v.Name)
		names[nm] = names[nm] + 1
	}

	tablenames := make(map[string]int)
	for _, v := range params {
		nm := strings.Title(v.TableName) + strings.Title(v.Name)
		tablenames[nm] = tablenames[nm] + 1
	}

	nameUsed := make(map[string]int)
	for _, v := range params {
		ft := calcFieldType(v.GoType(), v.InPattern)
		nm := strings.Title(v.Name)
		if v.InPattern {
			nm += "List"
		}
		if names[nm] > 1 {
			tnm := strings.Title(v.TableName) + strings.Title(v.Name)
			if tablenames[tnm] > 1 {
				nm = fmt.Sprintf("%s%d", tnm, nameUsed[tnm])
				nameUsed[tnm]++
			} else {
				nm = tnm
			}
		}
		nameUsed[nm] = nameUsed[nm] + 1
		rst.Fields = append(rst.Fields, codegen.NewGoField(nm, ft, ""))
	}
	return &rst
}

func calcFieldType(t schema.GoType, list bool) codegen.GoType {
	if t.Type == schema.GoTypeTime {
		return codegen.GoType{
			Pkg:       "time",
			ID:        strings.Title(string(t.Type)),
			IsList:    list,
			IsPointer: !t.NotNull,
		}
	}
	if t.Type == schema.GoTypeJson {
		return codegen.GoType{
			Pkg:       "json",
			ID:        strings.Title(string(t.Type)),
			IsList:    list,
			IsPointer: !t.NotNull,
		}
	}
	return codegen.GoType{
		Pkg:       "",
		ID:        string(t.Type),
		IsList:    list,
		IsPointer: !t.NotNull,
	}
}

func canStarCoverOutput(tb schema.SQLTable, output []GoVar) bool {
	starCols := tb.StarColumns()
	if len(starCols) != len(output) {
		return false
	}
	for i := range starCols {
		if starCols[i].Name() != output[i].Name || output[i].TableName != tb.Name() {
			return false
		}
	}
	return true
}

func canStarCoverInput(tb schema.SQLTable, input []GoParam) bool {
	starCols := tb.StarColumns()
	if len(starCols) != len(input) {
		return false
	}
	for i := range starCols {
		if starCols[i].Name() != input[i].Name || input[i].TableName != tb.Name() {
			return false
		}
	}
	return true
}
