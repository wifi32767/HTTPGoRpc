package gorpc

import "reflect"

type Method struct {
	method   reflect.Method
	ArgType  reflect.Type
	RetType  reflect.Type
	Receiver reflect.Value // 结构体的实例对象，用于作为call的参数
}

func (m *Method) newArgv() reflect.Value {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *Method) newRetv() reflect.Value {
	if m.RetType.Kind() == reflect.Ptr {
		return reflect.New(m.RetType.Elem())
	}
	return reflect.New(m.RetType).Elem()
}
