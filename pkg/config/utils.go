package config

import (
	"bytes"
	"errors"
	"text/template"

	data "github.com/stumble/needle/pkg/config/template"
)

func GenTemplate(objName string) (string, error) {
	if !validName(objName) {
		return "", errors.New("Invalid objName, length < 2 or not start with Uppercase: " + objName)
	}
	t, err := template.New("initTmpl").Parse(data.GetInitTemplate())
	if err != nil {
		return "", err
	}
	buf := bytes.NewBufferString("")
	err = t.Execute(buf, struct {
		TableName string
		ObjName   string
	}{
		objName + "s",
		objName,
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil

}
