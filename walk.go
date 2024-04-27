package ctxutil

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
	"unsafe"
)

// Walk the chain of contexts and call the each function on each one
// until we get to an "empty" context (context.Background() or
// context.TODO()) or the each function returns a non-nil result.
func Walk(ctx context.Context, each func(context.Context) error) error {
	for ctx != nil {
		if err := each(ctx); err != nil {
			return err
		}
		ctxReflectType := reflect.TypeOf(ctx)
		ctxData, ok := ctxTypes[ctxReflectType]
		if !ok {
			return fmt.Errorf(
				"%[1]w: %[2]v (type: %[2]T)",
				ErrUnknownContextImplementation,
				ctx,
			)
		}
		ctx = ctxData.parent(ctx)
	}
	return nil
}

func WalkValues(ctx context.Context, each func(ctx context.Context, key, value interface{}) error) error {
	return Walk(ctx, func(ctx context.Context) error {
		ct := reflect.TypeOf(ctx)
		if ct != valueCtxType {
			return nil
		}
		type valueCtx struct {
			context.Context
			key, value interface{}
		}
		id := (*ifaceData)(unsafe.Pointer(&ctx))
		vc := (*valueCtx)(id.Data)
		return each(ctx, vc.key, vc.value)
	})
}

var (
	ErrUnknownContextImplementation = errors.New(
		"unknown context implementation",
	)

	emptyCtxType  = reflect.TypeOf(context.Background())
	cancelCtxType = reflect.TypeOf(func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}())
	deadlineCtxType = reflect.TypeOf(func() context.Context {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Minute))
		cancel()
		return ctx
	}())
	valueCtxType = reflect.TypeOf(context.WithValue(context.Background(), (*interface{})(nil), nil))

	ctxTypes = map[reflect.Type]*ctxType{
		emptyCtxType: {
			parent: func(context.Context) context.Context { return nil },
		},
		cancelCtxType: {
			parent: getCtxParentFirstField,
		},
		deadlineCtxType: {
			parent: func(ctx context.Context) context.Context {
				id := (*ifaceData)(unsafe.Pointer(&ctx))
				cancelCtxPtr := ((*ctxFirstFieldCancelCtxParent)(id.Data)).Pointer
				cancelCtxIface := reflect.NewAt(cancelCtxType.Elem(), cancelCtxPtr).Interface().(context.Context)
				return getCtxParentFirstField(cancelCtxIface)
			},
		},
		valueCtxType: {
			parent: getCtxParentFirstField,
		},
	}
)

type ifaceData struct {
	Type unsafe.Pointer
	Data unsafe.Pointer
}

type ctxType struct {
	parent func(context.Context) context.Context
}

func getCtxParentFirstField(ctx context.Context) context.Context {
	type ctxParentFirstField struct {
		Context context.Context
	}
	id := (*ifaceData)(unsafe.Pointer(&ctx))
	return ((*ctxParentFirstField)(id.Data)).Context
}

type ctxFirstFieldCancelCtxParent struct {
	Pointer unsafe.Pointer
}
