//go:build !bindings

package main

// isolateBindings does nothing in the real program.
func isolateBindings() (cleanup func(), err error) { return func() {}, nil }
