package template

import (
	_ "embed"
)

//go:embed templates/init.tmpl
var configInitTemplate string

// GetInitTemplate return an example XML template
func GetInitTemplate() string {
	return configInitTemplate
}
