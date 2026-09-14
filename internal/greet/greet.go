// Package greet builds greetings.
package greet

import (
	"cmp"
	"fmt"
)

// Greeting returns a greeting for name, or for "world" when name is empty.
func Greeting(name string) string {
	return fmt.Sprintf("Hello, %s!", cmp.Or(name, "world"))
}
