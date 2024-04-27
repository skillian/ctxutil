package ctxutil_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/skillian/ctxutil"
)

func TestWalk(t *testing.T) {
	expect := make([]context.Context, 4)
	emptyCtx := context.Background()
	expect[3] = emptyCtx
	cancelCtx, cancelCancel := context.WithCancel(emptyCtx)
	defer cancelCancel()
	expect[2] = cancelCtx
	timerCtx, timerCancel := context.WithTimeout(cancelCtx, 1*time.Minute)
	defer timerCancel()
	expect[1] = timerCtx
	type contextKeyValue string
	valueCtx := context.WithValue(timerCtx, contextKeyValue("Hello"), contextKeyValue("World!"))
	expect[0] = valueCtx

	actual := make([]context.Context, 0, 4)

	if err := ctxutil.Walk(valueCtx, func(ctx context.Context) error {
		actual = append(actual, ctx)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for i, e := range expect {
		et := reflect.TypeOf(e)
		at := reflect.TypeOf(actual[i])
		if et != at {
			t.Errorf(
				"Expected %T at index %d, but actual: %T",
				e, i, actual[i],
			)
		}
	}
}

func TestWalkValues(t *testing.T) {
	p1, p2 := new(int), new(int)
	expect := [][2]interface{}{
		{123, 456},
		{"Hello", "World!"},
		{p1, p2},
	}
	ctx := context.Background()
	cancels := make([]context.CancelFunc, len(expect))
	for i := range expect {
		kv := expect[len(expect)-1-i]
		ctx = context.WithValue(ctx, kv[0], kv[1])
		// add non-value contexts into the chain, like real
		// context chains:
		ctx, cancels[i] = context.WithTimeout(ctx, 1*time.Minute)
	}
	actual := make([][2]interface{}, 0, len(expect))
	if err := ctxutil.WalkValues(ctx, func(ctx context.Context, key, value interface{}) error {
		actual = append(actual, [2]interface{}{key, value})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for i, e := range expect {
		if actual[i] != e {
			t.Errorf(
				"expected [%[1]d] = %[2]v (type: %[2]T), "+
					"actual = %[3]v (type: %[3]T)",
				i, e, actual[i],
			)
		}
	}
}
