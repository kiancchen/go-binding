package binding

import (
	"errors"
	"fmt"
	"mime/multipart"
	"reflect"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	// tagBind 的选项
	bindAuto     = "auto"
	bindHeader   = "header"
	bindQuery    = "query"
	bindForm     = "form"
	bindPath     = "path"
	bindJson     = "json"
	bindRequired = "required"
	bindReq      = "req"
)

func Bind(r Request, recvPtr interface{}) error {
	recvType := reflect.TypeOf(recvPtr)
	if recvType.Kind() != reflect.Ptr {
		return fmt.Errorf("A pointer is required but [%v] provided", recvType)
	}
	recvType = recvType.Elem()
	if recvType.Kind() != reflect.Struct {
		return fmt.Errorf("A struct is required but [%v] provided", recvType)
	}

	structMeta := ParseStruct(reflect.ValueOf(recvPtr).Elem().Interface())

	err := BindWithStructMeta(r, recvPtr, structMeta)
	if err != nil {
		return err
	}

	return nil
}

func BindWithStructMeta(r Request, recvPtr interface{}, structMeta *StructMetadata) error {
	recvType := reflect.TypeOf(recvPtr)
	if recvType.Kind() != reflect.Ptr {
		return fmt.Errorf("A pointer is required but [%v] provided", recvType)
	}
	recvType = recvType.Elem()
	if recvType.Kind() != reflect.Struct {
		return nil
	}

	req, err := newRequest(r)
	if err != nil {
		return err
	}

	sm := structMeta.clone()
	bindStruct(req, reflect.ValueOf(recvPtr), sm)

	failedFields := checkFields(sm)
	if len(*failedFields) > 0 {
		var msg strings.Builder
		for _, e := range *failedFields {
			msg.WriteString(e.Error() + "\n")
		}
		return errors.New(msg.String())
	}

	return nil
}

func bindStruct(r *request, recv reflect.Value, structMeta *StructMetadata) {
	for i := 0; i < structMeta.FieldNum; i++ {
		fieldMeta := (structMeta.FieldList)[i]
		resolveField(r, fieldMeta)
		fieldMeta.setValue(recv.Elem().Field(i))
	}
}

func resolveField(r *request, fieldMeta *fieldMetadata) {
	var value reflect.Value
	fieldMeta.value = &value

	if fieldMeta.isFile {
		files, ok := r.GetFormFile(fieldMeta.fieldName)
		if !ok {
			fieldMeta.isUnset = true
			return
		}
		if fieldMeta.isSlice {
			if fieldMeta.sliceMeta.isPtr {
				value = reflect.ValueOf(files)
			} else {
				tempFiles := make([]multipart.FileHeader, len(files))
				for i, file := range files {
					tempFiles[i] = *file
				}
				value = reflect.ValueOf(tempFiles)
			}
		} else {
			value = reflect.ValueOf(*files[0])
		}

	} else if fieldMeta.isStruct {
		value = reflect.New(fieldMeta.structMeta.StructType)
		bindStruct(r, value, fieldMeta.structMeta)
		value = value.Elem()

	} else if fieldMeta.isSlice && fieldMeta.sliceMeta.isStruct {
		sliceMeta := fieldMeta.sliceMeta
		array := gjson.GetBytes(r.GetBody(), sliceMeta.fieldJsonName).Array()
		length := len(array)
		value = reflect.MakeSlice(sliceMeta.sliceType, length, length)
		sliceMeta.structData = make([]*StructMetadata, length)
		for j := range sliceMeta.structData {
			sliceMeta.structData[j] = sliceMeta.structMeta.clone()
			sliceMeta.structData[j].attachLayerNum(j)

			receiver := reflect.New(sliceMeta.elemType)
			bindStruct(r, receiver, sliceMeta.structData[j])

			if !sliceMeta.isPtr {
				receiver = receiver.Elem()
			}

			value.Index(j).Set(receiver)
		}

		if length == 0 {
			fieldMeta.isUnset = true
		}

	} else {
		// 获取原始的 string 数据
		originValues, ok := getValue(r, fieldMeta)
		if !ok {
			fieldMeta.isUnset = true
			return
		}

		// 根据注册的 convertor 转化为对应的类型
		elemType := fieldMeta.elemType
		length := len(originValues)

		if fieldMeta.isSlice {
			sliceMeta := fieldMeta.sliceMeta
			tempValues := make([]string, 0, length)
			for _, originValue := range originValues {
				if gjson.Valid(originValue) {
					array := gjson.Parse(originValue).Array()
					for _, result := range array {
						tempValues = append(tempValues, result.String())
					}
				} else {
					tempValues = append(tempValues, originValue)
				}
			}
			length = len(tempValues)
			originValues = tempValues

			value = reflect.MakeSlice(sliceMeta.sliceType, length, length)
			elemType = sliceMeta.elemType
		}

		convertor := getConvertor(elemType)
		if convertor == nil {
			fieldMeta.hasConversionError = true
			return
		}

		for i, originValue := range originValues {
			var convertedValue interface{}
			convertedValue, err := convertor(originValue)
			if err != nil {
				value.Index(i).Set(reflect.Zero(fieldMeta.sliceMeta.originalType))
				continue
			}

			v := reflect.ValueOf(convertedValue)

			// 如果不是 slice，直接返回第一个
			if !fieldMeta.isSlice {
				value = v
				return
			}

			if fieldMeta.sliceMeta.isPtr {
				ptr := reflect.New(v.Type())
				ptr.Elem().Set(v)
				v = ptr
			}

			value.Index(i).Set(v)
		}
	}
}

func checkFields(structMeta *StructMetadata) *[]*Error {
	failedFields := make([]*Error, 0)
	for _, field := range structMeta.FieldList {
		if field.isUnset && field.isRequired {
			failedFields = append(failedFields, FieldNotFound.setField(field.fieldJsonName))
		}
		if field.hasConversionError {
			failedFields = append(failedFields, FieldConversionError.setField(field.fieldJsonName))
		}

		if field.isFile {
			continue
		}
		if field.isStruct {
			failedFields = append(failedFields, *checkFields(field.structMeta)...)
		} else if field.isSlice {
			for _, sd := range field.sliceMeta.structData {
				failedFields = append(failedFields, *checkFields(sd)...)
			}
		}
	}
	return &failedFields
}

func getValue(r *request, fieldMeta *fieldMetadata) (originValue []string, present bool) {
	if hasTag(fieldMeta.source, header) {
		originValue, present = r.GetHeader(fieldMeta.fieldName)
		if present {
			return
		}
	}

	if hasTag(fieldMeta.source, query) {
		originValue, present = r.GetQuery(fieldMeta.fieldName)
		if present {
			return
		}
	}

	// if hasTag(fieldMeta.source, path) {
	// 	value, present = r.GetPath(fieldMeta.fieldName)
	// 	if present {
	// 		originValue = []string{value}
	// 		return
	// 	}
	// }

	if hasTag(fieldMeta.source, form) {
		originValue, present = r.GetPostForm(fieldMeta.fieldName)
		if present {
			return
		}
	}

	if hasTag(fieldMeta.source, json) {
		v := gjson.GetBytes(r.GetBody(), fieldMeta.fieldJsonName)
		present = v.Exists()
		if present {
			originValue = []string{v.String()}
			return
		}
	}

	if fieldMeta.hasDefault {
		present = true
		originValue = []string{fieldMeta.defaultVal}
	}

	return
}
