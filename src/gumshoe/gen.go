// +build ignore

// This file generates Go source code for various types and type/query-function combinations.
//
// Invoke as
//
//		go run gen.go | gofmt > types_gen.go
//
package main

import (
	"log"
	"os"
	"strings"
	"text/template"
)

var types = []string{
	"uint8",
	"int8",
	"uint16",
	"int16",
	"uint32",
	"int32",
	"float32",
}

var filters = []FilterType{
	{"FilterEqual", "=", "=="},
	{"FilterNotEqual", "!=", "!="},
	{"FilterGreaterThan", ">", ">"},
	{"FilterGreaterThenOrEqual", ">=", ">="},
	{"FilterLessThan", "<", "<"},
	{"FilterLessThanOrEqual", "<=", "<="},
	{"FilterIn", "in", ""},
}

type Type struct {
	GoName          string // uint8
	TitleName       string // Uint8
	GumshoeTypeName string // TypeUint8
}

// Simple filters have a corresponding Go operator
type FilterType struct {
	GumshoeTypeName string // FilterEqual
	Symbol          string // =
	GoOperator      string // ==
}

func main() {
	elements := struct {
		Types             []Type
		IntTypes          []Type
		UintTypes         []Type // TODO(caleb): unneeded?
		FloatTypes        []Type
		FilterTypes       []FilterType
		SimpleFilterTypes []FilterType // Binary op filters
		Bools             []bool
	}{
		FilterTypes: filters,
		Bools:       []bool{true, false},
	}

	for _, name := range types {
		typ := Type{
			GoName:          name,
			TitleName:       strings.Title(name),
			GumshoeTypeName: "Type" + strings.Title(name),
		}
		if strings.HasPrefix(name, "int") || strings.HasPrefix(name, "uint") {
			elements.IntTypes = append(elements.IntTypes, typ)
		}
		if strings.HasPrefix(name, "uint") {
			elements.UintTypes = append(elements.UintTypes, typ)
		}
		if strings.HasPrefix(name, "float") {
			elements.FloatTypes = append(elements.FloatTypes, typ)
		}
		elements.Types = append(elements.Types, typ)
	}

	for _, filter := range filters {
		if filter.GoOperator != "" {
			elements.SimpleFilterTypes = append(elements.SimpleFilterTypes, filter)
		}
	}

	if err := tmpl.Execute(os.Stdout, elements); err != nil {
		log.Fatal(err)
	}
}

var tmpl = template.Must(template.New("source").Parse(sourceTemplate))

const sourceTemplate = `
// WARNING: AUTOGENERATED CODE
// Do not edit by hand (see gen.go).

package gumshoe

import (
	"math"
	"unsafe"
)

type Type int

const ( {{range .Types}}
{{.GumshoeTypeName}} Type = iota{{end}}
)

var typeWidths = []int{ {{range .Types}}
{{.GumshoeTypeName}}: int(unsafe.Sizeof({{.GoName}}(0))),{{end}}
}

var typeMaxes = []float64{ {{range .Types}}
{{.GumshoeTypeName}}: math.Max{{.TitleName}},{{end}}
}

var typeNames = []string{ {{range .Types}}
{{.GumshoeTypeName}}: "{{.GoName}}",{{end}}
}

var NameToType = map[string]Type{ {{range .Types}}
"{{.GoName}}": {{.GumshoeTypeName}},{{end}}
}

// add adds other to m (only m is modified).
func (m MetricBytes) add(s *Schema, other MetricBytes) {
	p1 := uintptr(unsafe.Pointer(&m[0]))
	p2 := uintptr(unsafe.Pointer(&other[0]))
	for i, column := range s.MetricColumns {
		offset := uintptr(s.MetricOffsets[i])
		col1 := unsafe.Pointer(p1 + offset)
		col2 := unsafe.Pointer(p2 + offset)
		switch column.Type { {{range .Types}}
		case {{.GumshoeTypeName}}:
			*(*{{.GoName}})(col1) = *(*{{.GoName}})(col1) + (*(*{{.GoName}})(col2)){{end}}
		}
	}
}

func setRowValue(pos unsafe.Pointer, typ Type, value float64) {
	switch typ { {{range .Types}}
	case {{.GumshoeTypeName}}:
		*(*{{.GoName}})(pos) = {{.GoName}}(value){{end}}
	}
}

// numericCellValue decodes a numeric value from cell based on typ. It does not look into any dimension
// tables.
func (s *State) numericCellValue(cell unsafe.Pointer, typ Type) Untyped {
	switch typ { {{range .Types}}
	case {{.GumshoeTypeName}}:
		return *(*{{.GoName}})(cell){{end}}
	}
	panic("unexpected type")
}

// Query helper functions

type FilterType int

const ( {{range .FilterTypes}}
{{.GumshoeTypeName}} FilterType = iota{{end}}
)

var filterTypeToName = []string{ {{range .FilterTypes}}
{{.GumshoeTypeName}}: "{{.Symbol}}",{{end}}
}

var filterNameToType = map[string]FilterType{ {{range .FilterTypes}}
"{{.Symbol}}": {{.GumshoeTypeName}},{{end}}
}

func makeSumFuncGen(typ Type) func(offset int) sumFunc {
	{{range .Types}}
	if typ == {{.GumshoeTypeName}} {
		return func(offset int) sumFunc {
			return func(sum UntypedBytes, metrics MetricBytes) {
				*(*{{.GoName}})(unsafe.Pointer(&sum[0])) += *(*{{.GoName}})(unsafe.Pointer(&metrics[offset]))
			}
		}
	}{{end}}
	panic("unreached")
}

func makeGetDimensionValueFuncGen(typ Type) func(cell unsafe.Pointer) Untyped {
	{{range .Types}}
	if typ == {{.GumshoeTypeName}} {
		return func(cell unsafe.Pointer) Untyped { return *(*{{.GoName}})(cell) }
	}{{end}}
	panic("unreached")
}

func makeGetDimensionValueAsIntFuncGen(typ Type) func(cell unsafe.Pointer) int {
	{{range .Types}}
	if typ == {{.GumshoeTypeName}} {
		return func(cell unsafe.Pointer) int { return int(*(*{{.GoName}})(cell)) }
	}{{end}}
	panic("unreached")
}

func makeTimestampFilterFuncSimpleGen(filter FilterType) func(timestamp uint32) timestampFilterFunc {
	{{range $.SimpleFilterTypes}}
	if filter == {{.GumshoeTypeName}} {
		return func(timestamp uint32) timestampFilterFunc {
			return func(t uint32) bool { return t {{.GoOperator}} timestamp }
		}
	}{{end}}
	panic("unreached")
}

func makeNilFilterFuncSimpleGen(typ Type, filter FilterType) func(nilOffset int, mask byte) filterFunc {
	{{range $type := .Types}}{{range $filter := $.SimpleFilterTypes}}
	if typ == {{$type.GumshoeTypeName}} && filter == {{$filter.GumshoeTypeName}} {
		return func(nilOffset int, mask byte) filterFunc {
			return func(row RowBytes) bool {
				// See comparison truth table
				if row[nilOffset] & mask > 0 {
					return {{if eq $filter.Symbol "="}} true {{else}} false {{end}}
				}
				return {{if eq $filter.Symbol "!="}} true {{else}} false {{end}}
			}
		}
	}{{end}}{{end}}
	panic("unreached")
}

func makeDimensionFilterFuncSimpleGen(typ Type, filter FilterType, isString bool) func(interface{}, int, byte, int) filterFunc {
	{{range $type := .Types}}{{range $filter := $.SimpleFilterTypes}}{{range $str := $.Bools}}
	if typ == {{$type.GumshoeTypeName}} && filter == {{$filter.GumshoeTypeName}} && isString == {{$str}} {
		return func(value interface{}, nilOffset int, mask byte, valueOffset int) filterFunc {
			{{if $str}}
			v := {{$type.GoName}}(value.(uint32))
			{{else}}
			v := {{$type.GoName}}(value.(float64))
			{{end}}
			return func(row RowBytes) bool {
				if row[nilOffset] & mask > 0 {
					return {{if eq $filter.Symbol "!="}} true {{else}} false {{end}}
				}
				return *(*{{$type.GoName}})(unsafe.Pointer(&row[valueOffset])) {{$filter.GoOperator}} v
			}
		}
	}{{end}}{{end}}{{end}}
	panic("unreached")
}

func makeDimensionFilterFuncInGen(typ Type, isString bool) func(interface{}, bool, int, byte, int) filterFunc {
	{{range $type := .Types}}{{range $str := $.Bools}}
	if typ == {{$type.GumshoeTypeName}} && isString == {{$str}} {
		return func(values interface{}, acceptNil bool, nilOffset int, mask byte, valueOffset int) filterFunc {
			var typedValues []{{$type.GoName}}
			{{if $str}}
			for _, v := range values.([]uint32) {
			{{else}}
			for _, v := range values.([]float64) {
			{{end}}
				typedValues = append(typedValues, {{$type.GoName}}(v))
			}
			return func(row RowBytes) bool {
				if row[nilOffset] & mask > 0 {
					return acceptNil
				}
				value := *(*{{$type.GoName}})(unsafe.Pointer(&row[valueOffset]))
				for _, v := range typedValues {
					if value == v {
						return true
					}
				}
				return false
			}
		}
	}{{end}}{{end}}
	panic("unreached")
}

func makeMetricFilterFuncSimpleGen(typ Type, filter FilterType) func(value float64, offset int) filterFunc {
	{{range $type := .Types}}{{range $filter := $.SimpleFilterTypes}}
	if typ == {{$type.GumshoeTypeName}} && filter == {{$filter.GumshoeTypeName}} {
		return func(value float64, offset int) filterFunc {
			v := {{$type.GoName}}(value)
			return func(row RowBytes) bool {
				return *(*{{$type.GoName}})(unsafe.Pointer(&row[offset])) {{$filter.GoOperator}} v
			}
		}
	}{{end}}{{end}}
	panic("unreached")
}

func makeMetricFilterFuncInGen(typ Type) func(floats []float64, offset int) filterFunc {
	{{range .Types}}
	if typ == {{.GumshoeTypeName}} {
		return func(floats []float64, offset int) filterFunc {
			typedValues := make([]{{.GoName}}, len(floats))
			for i, f := range floats {
				typedValues[i] = {{.GoName}}(f)
			}
			return func(row RowBytes) bool {
				value := *(*{{.GoName}})(unsafe.Pointer(&row[offset]))
				for _, v := range typedValues {
					if value == v {
						return true
					}
				}
				return false
			}
		}
	}{{end}}
	panic("unreached")
}
`
