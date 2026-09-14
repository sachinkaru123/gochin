package logs

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
)

// Dump logs a variable at debug level along with the file and line that asked
// for it, which is what makes it usable for quick inspection:
//
//	logs.Dump(order)
//	logs.Dump(user, "after update")
func Dump(value any, label ...string) {
	name := "dump"
	if len(label) > 0 && label[0] != "" {
		name = label[0]
	}

	Logger().Debug(name,
		"caller", caller(2),
		"type", fmt.Sprintf("%T", value),
		"value", Format(value),
	)
}

// DumpTo is Dump against a named channel.
func DumpTo(channel string, value any, label ...string) {
	name := "dump"
	if len(label) > 0 && label[0] != "" {
		name = label[0]
	}

	Channel(channel).Debug(name,
		"caller", caller(2),
		"type", fmt.Sprintf("%T", value),
		"value", Format(value),
	)
}

// Format renders a value for human reading: indented JSON when it marshals,
// Go syntax otherwise. Channels, funcs and cyclic structures do not marshal,
// hence the fallback.
func Format(value any) string {
	if value == nil {
		return "nil"
	}

	switch v := value.(type) {
	case string:
		return v
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	}

	if b, err := json.MarshalIndent(value, "", "  "); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%+v", value)
}

func caller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}
