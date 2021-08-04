package binding

import (
	"errors"
	"fmt"
	"mime/multipart"
	"reflect"
	"strings"

	"github.com/tidwall/gjson"
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

	err = checkFields(sm)
	if err != nil {
		return err
	}

	return nil
}

func bindStruct(r *request, recv reflect.Value, structMeta *StructMetadata) (set bool) {
	set = false
	for i := 0; i < structMeta.FieldNum; i++ {
		fieldMeta := (structMeta.FieldList)[i]
		if fieldMeta.isExported {
			resolveField(r, fieldMeta)
			if !fieldMeta.isUnset {
				set = true
			}
			fieldMeta.setValue(recv.Elem().Field(i))
		}
	}
	return
}

func resolveField(r *request, fieldMeta *fieldMetadata) {
	var value reflect.Value
	fieldMeta.value = &value

	value = getFieldValue(r, fieldMeta)
	if fieldMeta.hasValue {
		return
	}

	fieldMeta.isUnset = false

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
		fieldMeta.hasValue = true
	} else if fieldMeta.isStruct {
		value = reflect.New(fieldMeta.structMeta.StructType)
		fieldMeta.isUnset = !bindStruct(r, value, fieldMeta.structMeta)
		fieldMeta.hasValue = fieldMeta.isUnset
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
		fieldMeta.hasValue = true
		if length == 0 {
			fieldMeta.isUnset = true
		}
	} else {
		fieldMeta.isUnset = true
	}

	if !fieldMeta.isUnset {
		fieldMeta.hasConversionError = false
		fieldMeta.hasValue = true
	}
}

func getFieldValue(r *request, fieldMeta *fieldMetadata) (value reflect.Value) {
	// 获取原始的 string 数据
	originValues, ok := getValue(r, fieldMeta)
	if !ok {
		fieldMeta.isUnset = true
		return
	}

	var after []string
	var processed bool
	for _, name := range fieldMeta.preprocessor {
		processor, ok := getPreprocessor(name)
		if ok {
			processed = true
			for _, v := range originValues {
				res, err := processor(v)
				if err != nil {
					fieldMeta.errs = append(fieldMeta.errs, err)
				}
				after = append(after, res...)
			}
		}
	}
	if processed {
		originValues = after
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
		v := reflect.ValueOf(convertedValue)
		if err != nil {
			fieldMeta.hasConversionError = true
		}

		// 如果不是 slice，直接返回第一个
		if !fieldMeta.isSlice {
			value = v
			fieldMeta.hasValue = true
			return
		}

		if fieldMeta.sliceMeta.isPtr {
			ptr := reflect.New(v.Type())
			ptr.Elem().Set(v)
			v = ptr
		}

		value.Index(i).Set(v)
	}
	fieldMeta.hasValue = true
	return
}

func checkFields(structMeta *StructMetadata) error {
	notFoundError := make([]string, 0)
	conversionError := make([]string, 0)
	errs := make([]string, 0)
	for _, field := range structMeta.FieldList {
		if field.isUnset && field.isRequired {
			notFoundError = append(notFoundError, field.fieldJsonName)
		}
		if field.hasConversionError {
			conversionError = append(conversionError, field.fieldJsonName)
		}
		for _, err := range field.errs {
			errs = append(errs, err.Error())
		}

		if field.isFile {
			continue
		}
		if field.isStruct {
			notFound, conversion, es := checkFields2(field.structMeta)
			notFoundError = append(notFoundError, notFound...)
			conversionError = append(conversionError, conversion...)
			errs = append(errs, es...)
		} else if field.isSlice {
			for _, sd := range field.sliceMeta.structData {
				notFound, conversion, es := checkFields2(sd)
				notFoundError = append(notFoundError, notFound...)
				conversionError = append(conversionError, conversion...)
				errs = append(errs, es...)
			}
		}
	}

	sb := strings.Builder{}
	if len(notFoundError) != 0 {
		sb.WriteString(fmt.Sprintf("parameter required but not found: [%v]", strings.Join(notFoundError, ", ")))
	}
	if len(conversionError) != 0 {
		if sb.Len() != 0 {
			sb.WriteString("; ")
		}

		sb.WriteString(fmt.Sprintf("parameter type cannot be converted from string: [%v]", strings.Join(conversionError, ", ")))
	}
	for _, err := range errs {
		sb.WriteString(err)
	}

	if sb.Len() == 0 {
		return nil
	}

	return errors.New(sb.String())
}

func checkFields2(structMeta *StructMetadata) ([]string, []string, []string) {
	notFoundError := make([]string, 0)
	conversionError := make([]string, 0)
	errs := make([]string, 0)
	for _, field := range structMeta.FieldList {
		if field.isUnset && field.isRequired {
			notFoundError = append(notFoundError, field.fieldJsonName)
		}
		if field.hasConversionError {
			conversionError = append(conversionError, field.fieldJsonName)
		}
		for _, err := range field.errs {
			errs = append(errs, err.Error())
		}

		if field.isFile {
			continue
		}
		if field.isStruct {
			notFound, conversion, es := checkFields2(field.structMeta)
			notFoundError = append(notFoundError, notFound...)
			conversionError = append(conversionError, conversion...)
			errs = append(errs, es...)
		} else if field.isSlice {
			for _, sd := range field.sliceMeta.structData {
				notFound, conversion, es := checkFields2(sd)
				notFoundError = append(notFoundError, notFound...)
				conversionError = append(conversionError, conversion...)
				errs = append(errs, es...)
			}
		}
	}
	return notFoundError, conversionError, errs
}

func getValue(r *request, fieldMeta *fieldMetadata) (originValue []string, present bool) {
	if hasTag(fieldMeta.source, header) {
		key := fieldMeta.fieldName
		key = strings.ToLower(key)
		key = strings.ReplaceAll(key, "-", " ")
		key = strings.Title(key)
		key = strings.ReplaceAll(key, " ", "-")

		originValue, present = r.GetHeader(key)
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
