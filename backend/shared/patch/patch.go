package patch

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var ErrSkip = errors.New("patch: skip field")

type TransformFunc func(name string, value any) (any, error)
type ManualHandlerFunc func(name string, value any) error

type config struct {
	ignore        map[string]struct{}
	manual        map[string]struct{}
	transforms    map[string]TransformFunc
	before        []TransformFunc
	manualHandler ManualHandlerFunc
}

type Option func(*config)

func WithIgnore(fields ...string) Option {
	return func(c *config) {
		for _, f := range fields {
			c.ignore[f] = struct{}{}
		}
	}
}

func WithManual(fields ...string) Option {
	return func(c *config) {
		for _, f := range fields {
			c.manual[f] = struct{}{}
		}
	}
}

func WithTransform(field string, fn TransformFunc) Option {
	return func(c *config) {
		c.transforms[field] = fn
	}
}

func WithBefore(fn TransformFunc) Option {
	return func(c *config) {
		c.before = append(c.before, fn)
	}
}

func WithManualHandler(fn ManualHandlerFunc) Option {
	return func(c *config) {
		c.manualHandler = fn
	}
}

type ManualField struct {
	Name  string
	Value any
}

type Result struct {
	Updated []string
	Manual  []ManualField
}

type fieldMeta struct {
	index    int
	name     string
	exported bool
	anon     bool
	byTag    fieldTag
}

type fieldTag struct {
	ignore bool
	manual bool
}

var metaCache sync.Map

func getCachedMeta(t reflect.Type) []fieldMeta {
	if v, ok := metaCache.Load(t); ok {
		return v.([]fieldMeta)
	}
	metas := make([]fieldMeta, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("patch")
		metas[i] = fieldMeta{
			index:    i,
			name:     f.Name,
			exported: f.PkgPath == "",
			anon:     f.Anonymous,
			byTag: fieldTag{
				ignore: tag == "ignore",
				manual: tag == "manual",
			},
		}
	}
	metaCache.Store(t, metas)
	return metas
}

func buildPatchIndex(patchVal reflect.Value) map[string]int {
	t := patchVal.Type()
	idx := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		idx[t.Field(i).Name] = i
	}
	return idx
}

func Patch(dst, p any, opts ...Option) (*Result, error) {
	dstPtr := reflect.ValueOf(dst)
	if dstPtr.Kind() != reflect.Ptr || dstPtr.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("patch.Patch: dst must be a pointer to a struct, got %T", dst)
	}

	patchVal := reflect.ValueOf(p)
	if patchVal.Kind() == reflect.Ptr {
		patchVal = patchVal.Elem()
	}
	if patchVal.Kind() != reflect.Struct {
		return nil, fmt.Errorf("patch.Patch: patch must be a struct or pointer to a struct, got %T", p)
	}

	cfg := &config{
		ignore:     make(map[string]struct{}),
		manual:     make(map[string]struct{}),
		transforms: make(map[string]TransformFunc),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	for name := range cfg.ignore {
		if _, ok := cfg.manual[name]; ok {
			return nil, fmt.Errorf("patch.Patch: field %q is in both ignore and manual", name)
		}
	}

	patchIndex := buildPatchIndex(patchVal)
	dstType := dstPtr.Elem().Type()
	metas := getCachedMeta(dstType)
	result := &Result{}

	for _, meta := range metas {
		if !meta.exported || meta.anon {
			continue
		}

		name := meta.name

		if _, ignored := cfg.ignore[name]; ignored || meta.byTag.ignore {
			continue
		}

		if _, isManual := cfg.manual[name]; isManual || meta.byTag.manual {
			pi, ok := patchIndex[name]
			if !ok {
				continue
			}
			patchField := patchVal.Field(pi)
			if patchField.Kind() != reflect.Ptr || patchField.IsNil() {
				continue
			}
			val := patchField.Elem().Interface()

			if cfg.manualHandler != nil {
				if err := cfg.manualHandler(name, val); err != nil {
					return nil, fmt.Errorf("field %s: manual handler error: %w", name, err)
				}
			}

			result.Manual = append(result.Manual, ManualField{Name: name, Value: val})
			continue
		}

		pi, ok := patchIndex[name]
		if !ok {
			continue
		}
		patchField := patchVal.Field(pi)

		if patchField.Kind() != reflect.Ptr {
			return nil, fmt.Errorf("field %s: patch field must be a pointer, got %s", name, patchField.Kind())
		}

		if patchField.IsNil() {
			continue
		}

		patchElem := patchField.Elem()
		dstFieldVal := dstPtr.Elem().Field(meta.index)

		if !patchElem.Type().AssignableTo(dstFieldVal.Type()) {
			return nil, fmt.Errorf("field %s: type mismatch (%s -> %s)", name, patchElem.Type(), dstFieldVal.Type())
		}

		if !dstFieldVal.CanSet() {
			return nil, fmt.Errorf("field %s: cannot be set", name)
		}

		var val any = patchElem.Interface()

		if fn, ok := cfg.transforms[name]; ok {
			var err error
			val, err = fn(name, val)
			if err != nil {
				if errors.Is(err, ErrSkip) {
					continue
				}
				return nil, fmt.Errorf("field %s: transform error: %w", name, err)
			}
			if val == nil {
				return nil, fmt.Errorf("field %s: transform returned nil", name)
			}
		} else {
			skipped := false
			for _, fn := range cfg.before {
				var err error
				val, err = fn(name, val)
				if err != nil {
					if errors.Is(err, ErrSkip) {
						skipped = true
						break
					}
					return nil, fmt.Errorf("field %s: transform error: %w", name, err)
				}
				if val == nil {
					return nil, fmt.Errorf("field %s: transform returned nil", name)
				}
			}
			if skipped {
				continue
			}
		}

		rv := reflect.ValueOf(val)
		if !rv.Type().AssignableTo(dstFieldVal.Type()) {
			return nil, fmt.Errorf("field %s: value type mismatch (%s -> %s)", name, rv.Type(), dstFieldVal.Type())
		}

		dstFieldVal.Set(rv)
		result.Updated = append(result.Updated, name)
	}

	return result, nil
}
