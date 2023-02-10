package schema

import (
	"geborm/dialect"
	"go/ast"
	"reflect"
)

type Field struct {
	Name string
	Type string
	Tag  string
}

type Schema struct {
	Model      interface{}
	Name       string
	fieldMap   map[string]*Field
	Fields     []*Field
	FieldNames []string
}

func (s *Schema) GetField(name string) *Field {
	return s.fieldMap[name]
}

func Parse(dest interface{}, d dialect.Dialect) *Schema {
	modelT := reflect.Indirect(reflect.ValueOf(dest)).Type()
	schema := &Schema{
		Model:    dest,
		Name:     modelT.Name(),
		fieldMap: make(map[string]*Field),
	}
	for i := 0; i < modelT.NumField(); i++ {
		p := modelT.Field(i)
		if !p.Anonymous && ast.IsExported(p.Name) {
			field := &Field{
				Name: p.Name,
				Type: d.DataTypeOf(reflect.Indirect(reflect.New(p.Type))),
			}
			if v, ok := p.Tag.Lookup("geborm"); ok {
				field.Tag = v
			}
			schema.fieldMap[p.Name] = field
			schema.Fields = append(schema.Fields, field)
			schema.FieldNames = append(schema.FieldNames, p.Name)
		}
	}
	return schema
}
