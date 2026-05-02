package keyenc

import (
	"reflect"
	"strings"
)

// Schema defines a key structure from a struct type. Fields become colon-separated
// key segments in order. Use `key:"name"` tags to override field names. Use `key:"-"`
// to skip a field.
//
// Example:
//
//	type StreamKey struct {
//	    Kind       string `key:"streams"`   // literal prefix
//	    SourceType string
//	    SourceID   string
//	    StreamID   string
//	}
//
//	schema := keyenc.NewSchema[StreamKey]()
//	key := schema.Key(StreamKey{Kind: "streams", SourceType: "m3u", SourceID: "src-1", StreamID: "abc"})
//	// => "streams:m3u:src-1:abc"
//
//	prefix := schema.Prefix(StreamKey{Kind: "streams", SourceType: "m3u", SourceID: "src-1"})
//	// => "streams:m3u:src-1:"
//
//	prefix2 := schema.Prefix(StreamKey{Kind: "streams", SourceType: "m3u"})
//	// => "streams:m3u:"

type Schema[T any] struct {
	fields []fieldInfo
}

type fieldInfo struct {
	index    int
	literal  string // non-empty = always use this value
}

func NewSchema[T any]() Schema[T] {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var fields []fieldInfo
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("key")
		if tag == "-" {
			continue
		}
		fi := fieldInfo{index: i}
		if tag != "" && tag != f.Name {
			fi.literal = tag
		}
		fields = append(fields, fi)
	}
	return Schema[T]{fields: fields}
}

// Key returns the full key for a populated struct. All fields must be non-empty.
func (s Schema[T]) Key(v T) []byte {
	return s.buildKey(v, false)
}

// Prefix returns a key prefix from a partially populated struct. Stops at the
// first empty field and appends a trailing colon for cursor seek.
func (s Schema[T]) Prefix(v T) []byte {
	return s.buildKey(v, true)
}

func (s Schema[T]) buildKey(v T, prefixMode bool) []byte {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	var parts []string
	for _, f := range s.fields {
		if f.literal != "" {
			parts = append(parts, f.literal)
			continue
		}
		val := rv.Field(f.index).String()
		if val == "" {
			break
		}
		parts = append(parts, val)
	}

	result := strings.Join(parts, ":")
	if prefixMode {
		result += ":"
	}
	return []byte(result)
}

// Parse extracts struct field values from a key.
func (s Schema[T]) Parse(key []byte) T {
	var v T
	rv := reflect.ValueOf(&v).Elem()
	parts := strings.Split(string(key), ":")

	pi := 0
	for _, f := range s.fields {
		if pi >= len(parts) {
			break
		}
		if f.literal != "" {
			pi++
			continue
		}
		rv.Field(f.index).SetString(parts[pi])
		pi++
	}
	return v
}

// Reverse builds a reverse-index key: "{prefix}:{lookupField}"
func Reverse(prefix, value string) []byte {
	return []byte(prefix + ":" + value)
}

// ReversePrefix returns the prefix for scanning all reverse index entries.
func ReversePrefix(prefix string) []byte {
	return []byte(prefix + ":")
}
