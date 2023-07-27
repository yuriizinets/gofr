package component

import (
	"reflect"
	"runtime"
	"strings"
)

// Component represents a component state builder, defined as a function.
type Component func(ctx *Context) State

// GetName returns the name of the component.
func (c Component) GetName() string {
	functionPath := runtime.FuncForPC(reflect.ValueOf(c).Pointer()).Name()
	tokens := strings.Split(functionPath, ".")
	if tokens[len(tokens)-1] == "func1" || tokens[len(tokens)-1] == "func2" {
		return tokens[len(tokens)-2]
	} else {
		return tokens[len(tokens)-1]
	}
}
