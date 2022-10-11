package utils

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func Title(s string) string {
	return cases.Title(language.Und, cases.NoLower).String(s)
}
