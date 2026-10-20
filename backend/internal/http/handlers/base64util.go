package handlers

import "encoding/base64"

func encodeBase64Std(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
