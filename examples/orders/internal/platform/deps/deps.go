// Package deps validates required constructor dependencies once, at
// construction time, so request paths never repeat nil checks.
package deps

import (
	"fmt"
	"reflect"
)

// Require returns an error naming the dependency when value is nil. It also
// detects a typed nil pointer, map, slice, func, or channel stored in an
// interface, which a plain value == nil comparison misses.
//
// Require checks presence only. Validate ranges, formats, and cross-field
// configuration rules explicitly.
func Require(name string, value any) error {
	if value == nil {
		return fmt.Errorf("%s is required", name)
	}
	switch v := reflect.ValueOf(value); v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		if v.IsNil() {
			return fmt.Errorf("%s is required", name)
		}
	default:
		// Other kinds cannot be nil, so a value is always present.
	}
	return nil
}
