package handlers

import "fmt"

func failMessage(prefix string, body []byte) error { return fmt.Errorf("%s: %s", prefix, string(body)) }
func failText(message string) error                { return fmt.Errorf("%s", message) }
