package handlers

import "strings"

func quoted(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
