// Package ctxutil wraps some of the context package's functions to
// improve cache locality of context value access.
package ctxutil

import (
	"context"
	"unsafe"
)

const Unsafe = true

// Background calls context.Background, but adds a cache of the keys
// that have been added to the context so that the keys can be
// "flattened" later.
func Background() context.Context {
	return context.Background()
	// cks := &contextKeys{&emptyInterfaces}
	// return context.WithValue(context.Background(), (*contextKeys)(nil), cks)
}

// WithValue calls context.WithValue but also allows the values to be
// walked.
func WithValue(ctx context.Context, k, v interface{}) context.Context {
	ctx = context.WithValue(ctx, k, v)
	if !Unsafe {
		it := &item{keyValue: keyValue{k, v}}
		if prev, ok := Value(ctx, (*item)(nil)).(*item); ok {
			it.prev = prev
		}
		ctx = context.WithValue(ctx, (*item)(nil), it)
	}
	return ctx
}

// Value is a wrapper around context.Context.Value that supports
// retrieving values from flattened values.
func Value(ctx context.Context, k interface{}) interface{} {
	var foundValue interface{}
	_ = WalkValues(ctx, func(ctx context.Context, key, value interface{}) error {
		switch v := value.(type) {
		case *flattened:
			keycount := len(v.keyValues) / 2
			for i, k2 := range v.keyValues[:keycount] {
				if eq(k, k2) {
					foundValue = v.keyValues[keycount+i]
					// Doesn't matter what we return here;
					// just need to break out of WalkValues.
					return ErrUnknownContextImplementation
				}
			}
		default:
			if eq(key, k) {
				foundValue = value
				return ErrUnknownContextImplementation
			}
		}
		return nil
	})
	return foundValue
}

// eq tries to just compare a == b but if that panics, compare the actual
// interface{} variable values.
func eq(a, b interface{}) (Ok bool) {
	// Note:  This implementation is only meant to handle comparisons
	// of the same interface{} value assigned to different variables
	// and to *not* panic if the comparison cannot be made; just
	// return false.
	//
	// e.g.:
	//
	//	var a interface{} = 1
	//	b := a
	//	eq(a, b) // is expected to return true
	//
	// but:
	//
	//	var a interface{} = 1
	//	var b interface{} = 1
	//	eq(a, b) // might be true or it might be false
	//
	defer func(a, b interface{}, ok *bool) {
		v := recover()
		if v == nil {
			return
		}
		type interfaceData struct {
			data [2]uintptr
		}
		// Just compare the interface values:
		ad := *((*interfaceData)(unsafe.Pointer(&a)))
		bd := *((*interfaceData)(unsafe.Pointer(&b)))
		*ok = ad == bd
	}(a, b, &Ok)
	return a == b
}

type keyValue struct {
	key   interface{}
	value interface{}
}

type flattened struct {
	keyValues []keyValue
}

// Flatten values in the context that this package knows about
// (i.e. added with this package's WithValue function):  Go value
// contexts are essentially a linked list of key-value pairs and
// looking up a value consists of following the linked list until you
// get to the right value.  Flattened values are more efficient to
// scan through on modern hardware because instead of "chasing"
// pointers in a linked list of parent contexts, a flat array can
// instead be scanned resulting in fewer cache misses and more
// predictable memory access patterns so that the CPU is more likely
// to prefetch the right data.
func Flatten(ctx context.Context) context.Context {
	f := &flattened{keyValues: make([]keyValue, 0, 4)}
	_ = WalkValues(ctx, func(ctx context.Context, key, value interface{}) error {
		f.keyValues = append(f.keyValues, keyValue{key, value})
		return nil
	})
	return WithValue(ctx, (*flattened)(nil), f)
}
