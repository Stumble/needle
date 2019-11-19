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
func GetInitTemplate() string {
	bytes, err := openFile("init.tmpl")
	if err != nil {
		panic(err)
	}
	return string(bytes)
}
