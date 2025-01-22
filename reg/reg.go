package reg

import "reflect"

var TypeRegistry = make(map[string]reflect.Type)

func registerType(typedNil interface{}) {
	t := reflect.TypeOf(typedNil).Elem()
	TypeRegistry[t.Name()] = t
}
