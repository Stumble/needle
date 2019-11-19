package template

import (
	"io/ioutil"
)

func openFile(path string) ([]byte, error) {
	file, err := templateAssets.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ioutil.ReadAll(file)
}

// GetQueryFuncTemplate - return a query func template
func GetQueryFuncTemplate() string {
	bytes, err := openFile("queryfunc.tmpl")
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

// GetMutationFuncTemplate - return a mutation func template
func GetMutationFuncTemplate() string {
	bytes, err := openFile("mutationfunc.tmpl")
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

// GetRepoTemplate - return a query func template
func GetRepoTemplate() string {
	bytes, err := openFile("repo.tmpl")
	if err != nil {
		panic(err)
	}
	return string(bytes)
}
