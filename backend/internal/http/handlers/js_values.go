package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// jsString implementa String(value) para valores JSON; não usa fmt.Sprint
// para objetos/arrays, cujo comportamento difere de JavaScript.
func jsString(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			if item != nil {
				parts[i] = jsString(item)
			}
		}
		return strings.Join(parts, ",")
	case map[string]any:
		return "[object Object]"
	default:
		return fmt.Sprint(value)
	}
}
func jsSpread(value any) map[string]any {
	result := map[string]any{}
	switch v := value.(type) {
	case map[string]any:
		for k, item := range v {
			result[k] = item
		}
	case []any:
		for i, item := range v {
			result[strconv.Itoa(i)] = item
		}
	case string:
		for i, unit := range utf16.Encode([]rune(v)) {
			result[strconv.Itoa(i)] = string(rune(unit))
		}
	}
	return result
}
