package binding

import (
	"reflect"
	"strconv"
)

type Convertor func(string) (interface{}, error)

var (
	convertMap     = map[reflect.Type]Convertor{}
	kindConvertMap = map[reflect.Kind]Convertor{
		reflect.String: func(originValue string) (interface{}, error) {
			return originValue, nil
		},
		reflect.Bool: func(originValue string) (interface{}, error) {
			if originValue == "false" || originValue == "False" || originValue == "" || originValue == "0" {
				return false, nil
			}

			return true, nil
		},
		reflect.Int: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseInt(originValue, 10, 32)
			if err != nil {
				return 0, err
			}

			return int(value), nil
		},
		reflect.Int8: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseInt(originValue, 10, 8)
			if err != nil {
				return int8(0), err
			}

			return int8(value), nil
		},
		reflect.Int16: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseInt(originValue, 10, 16)
			if err != nil {
				return int16(0), err
			}

			return int16(value), nil
		},
		reflect.Int32: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseInt(originValue, 10, 32)
			if err != nil {
				return int32(0), err
			}

			return int32(value), nil
		},
		reflect.Int64: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseInt(originValue, 10, 64)
			if err != nil {
				return int64(0), err
			}

			return int64(value), nil
		},
		reflect.Uint: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseUint(originValue, 10, 32)
			if err != nil {
				return uint(0), err
			}

			return uint(value), nil
		},
		reflect.Uint8: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseUint(originValue, 10, 8)
			if err != nil {
				return uint8(0), err
			}

			return uint8(value), nil
		},
		reflect.Uint16: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseUint(originValue, 10, 16)
			if err != nil {
				return uint16(0), err
			}

			return uint16(value), nil
		},
		reflect.Uint32: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseUint(originValue, 10, 32)
			if err != nil {
				return uint32(0), err
			}

			return uint32(value), nil
		},
		reflect.Uint64: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseUint(originValue, 10, 64)
			if err != nil {
				return uint64(0), err
			}

			return uint64(value), nil
		},
		reflect.Float32: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseFloat(originValue, 32)
			if err != nil {
				return float32(0), err
			}

			return float32(value), nil
		},
		reflect.Float64: func(originValue string) (interface{}, error) {
			value, err := strconv.ParseFloat(originValue, 64)
			if err != nil {
				return float64(0), err
			}

			return float64(value), nil
		},
	}
)

func RegisterTypeConvertor(target interface{}, convertor Convertor) {
	t := reflect.TypeOf(target)
	convertMap[t] = convertor
}

func getConvertor(t reflect.Type) Convertor {
	if t == nil {
		return nil
	}

	if c, ok := kindConvertMap[t.Kind()]; ok {
		return c
	}

	if c, ok := convertMap[t]; ok {
		return c
	}
	for tp, convertor := range convertMap {
		if tp.ConvertibleTo(t) && t.ConvertibleTo(tp) && t.Kind() == tp.Kind() {
			return convertor
		}
	}
	return nil
}
