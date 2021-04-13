package template

import (
	_ "embed"
)

//go:embed templates/queryfunc.tmpl
var queryFuncTemplate string

// GetQueryFuncTemplate - return a query func template
func GetQueryFuncTemplate() string {
	return queryFuncTemplate
}

//go:embed templates/mutationfunc.tmpl
var mutationFuncTemplate string

// GetMutationFuncTemplate - return a mutation func template
func GetMutationFuncTemplate() string {
	return mutationFuncTemplate
}

//go:embed templates/repo.tmpl
var repoTemplate string

// GetRepoTemplate - return a query func template
func GetRepoTemplate() string {
	return repoTemplate
}
