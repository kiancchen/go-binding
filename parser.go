package binding

import (
	"math"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

const (
	// bindSplit tag的key，value用英文逗号分隔
	bindSplit  = ","
	tagBind    = "bind"
	tagDefault = "default"

	// 参数来源
	header = 1 << 0
	query  = 1 << 1
	form   = 1 << 2
	path   = 1 << 3
	json   = 1 << 4
	Auto   = math.MaxInt32
)

var fileType = reflect.TypeOf(multipart.FileHeader{})

func hasTag(target int, flag int) bool {
	return (target & flag) == flag
}

// StructMetadata 结构体的结构化信息
type StructMetadata struct {
	StructName string

	// 非指针的结构体类型
	StructType reflect.Type

	// 结构体field数量
	FieldNum int

	// FieldMetaList
	FieldList []*fieldMetadata
}

func (s *StructMetadata) attachLayerNum(num int) {
	for i := range s.FieldList {
		f := s.FieldList[i]
		f.fieldJsonName = strings.Replace(f.fieldJsonName, "#", strconv.Itoa(num), 1)
		if f.isSlice {
			f.sliceMeta.fieldJsonName = strings.Replace(f.sliceMeta.fieldJsonName, "#", strconv.Itoa(num), 1)
			f.sliceMeta.structMeta.attachLayerNum(num)
			for _, sd := range f.sliceMeta.structData {
				sd.attachLayerNum(num)
			}
		}
	}
}

func (s *StructMetadata) clone() *StructMetadata {
	clone := *s
	clone.FieldList = make([]*fieldMetadata, len(s.FieldList))
	for i, field := range s.FieldList {
		clone.FieldList[i] = field.clone()
	}
	return &clone
}

type sliceMetadata struct {
	// slice 原始类型 []*struct
	sliceType reflect.Type

	// 解slice后的类型 *struct
	originalType reflect.Type

	// 解指针后的类型 struct
	elemType reflect.Type

	// originalType 是否是指针
	isPtr bool

	// elemType 是否是 struct
	isStruct bool

	// 如果是 struct, elemType 的信息
	structMeta *StructMetadata

	// 解析 json 后存储的信息
	structData []*StructMetadata

	// 用来在 json 中查询的名字
	fieldJsonName string
}

func (s *sliceMetadata) clone() *sliceMetadata {
	clone := *s
	if s.isStruct && s.structMeta != nil {
		clone.structMeta = s.structMeta.clone()
		clone.structData = make([]*StructMetadata, len(s.structData))
		for i, data := range s.structData {
			clone.structData[i] = data.clone()
		}
	}

	return &clone
}

// Field的结构化信息
type fieldMetadata struct {
	// Field原始信息
	fieldType *reflect.StructField

	// 原始类型  *struct / *[]*struct
	originalType reflect.Type

	// 原始类型是否是指针
	isPtr bool

	isFile bool

	// 解指针后的类型 struct / []*struct
	elemType reflect.Type

	// 是否是 struct
	isStruct bool

	// elemType 的信息
	structMeta *StructMetadata

	// 是否是 Slice
	isSlice bool

	// elemType 的信息
	sliceMeta *sliceMetadata

	isExported bool

	// Tag原始信息
	tagInfo reflect.StructTag

	// Field的名字，用于从Query、Body、Header中找值
	fieldName string

	// Field的名字，用于从Json中找值
	fieldJsonName string

	// Field来源，Query,Body,Header
	source int

	// 是否是必传的参数
	isRequired bool

	// 有没有设置default值
	hasDefault bool

	// default值
	defaultVal string

	value *reflect.Value

	hasValue bool

	isUnset bool

	hasConversionError bool
}

func (f *fieldMetadata) setValue(recv reflect.Value) {
	// 如果有错误，那设为零值
	v := *f.value
	if !f.hasValue {
		v = reflect.Zero(f.elemType)
	}

	if f.isPtr {
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		recv.Set(ptr)
	} else {
		recv.Set(v.Convert(f.elemType))
	}
}

func (f *fieldMetadata) clone() *fieldMetadata {
	clone := *f
	if f.structMeta != nil {
		clone.structMeta = f.structMeta.clone()
	}

	if f.sliceMeta != nil {
		clone.sliceMeta = f.sliceMeta.clone()
	}

	return &clone
}

func ParseStruct(structType interface{}) *StructMetadata {
	typ := reflect.TypeOf(structType)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return parseStruct(&typ, "")
}

func parseStruct(structType *reflect.Type, parentFieldJsonName string) *StructMetadata {
	t := *structType
	if t == fileType {
		return nil
	}

	numField := t.NumField()
	fieldMetaList := make([]*fieldMetadata, numField)
	for i := 0; i < numField; i++ {
		var fieldJsonName string
		if parentFieldJsonName != "" {
			fieldJsonName = parentFieldJsonName + "."
		}

		field := t.Field(i)
		tagInfo := field.Tag

		fieldMeta := &fieldMetadata{
			fieldType:     &field,
			tagInfo:       tagInfo,
			fieldName:     field.Name,
			isExported:    unicode.IsUpper([]rune(field.Name)[0]),
			fieldJsonName: fieldJsonName + field.Name,
		}
		if field.Anonymous {
			fieldMeta.fieldJsonName = fieldJsonName
		}

		parseTag(fieldMeta, fieldJsonName)

		// 最原始的类型，如 *struct
		fieldType := field.Type
		fieldMeta.originalType = field.Type

		// 解指针
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
			fieldMeta.isPtr = true
		}
		fieldMeta.elemType = fieldType

		// 如果 field 是 struct
		if fieldType.Kind() == reflect.Struct {
			fieldMeta.isStruct = true
			fieldMeta.structMeta = parseStruct(&fieldType, fieldMeta.fieldJsonName)
			if fieldType == fileType {
				fieldMeta.isFile = true
			}
		} else if fieldType.Kind() == reflect.Slice {
			fieldMeta.isSlice = true
			fieldMeta.sliceMeta = parseSlice(&fieldType, fieldMeta.fieldJsonName)
			if fieldMeta.sliceMeta.elemType == fileType {
				fieldMeta.isFile = true
			}
		}

		fieldMetaList[i] = fieldMeta
	}

	return &StructMetadata{
		StructType: t,
		FieldNum:   numField,
		StructName: t.Name(),
		FieldList:  fieldMetaList,
	}
}

func parseTag(fieldMeta *fieldMetadata, fieldJsonName string) {
	tagInfo := fieldMeta.tagInfo
	tags := strings.Split(tagInfo.Get(tagBind), bindSplit)
	for _, value := range tags {
		if value == "" {
			continue
		}
		switch value {
		case bindQuery:
			fieldMeta.source |= query
		case bindForm:
			fieldMeta.source |= form
		case bindHeader:
			fieldMeta.source |= header
			// header 首字母自动大写
			name := fieldMeta.fieldName
			for i, v := range name {
				fieldMeta.fieldName = string(unicode.ToUpper(v)) + name[i+1:]
				break
			}
		case bindPath:
			fieldMeta.source |= path
		case bindAuto:
			fieldMeta.source |= Auto
		case bindJson:
			fieldMeta.source |= json
		case bindRequired, bindReq:
			fieldMeta.isRequired = true
		default:
			fieldMeta.fieldName = value
			fieldMeta.fieldJsonName = fieldJsonName + fieldMeta.fieldName
		}
	}

	// default 解析
	defaultStr, ok := tagInfo.Lookup(tagDefault)
	if ok {
		fieldMeta.hasDefault = true
		fieldMeta.defaultVal = defaultStr
	}
}

func parseSlice(sliceType *reflect.Type, parentFieldJsonName string) *sliceMetadata {
	t := *sliceType
	sliceMeta := &sliceMetadata{
		sliceType:     t,
		fieldJsonName: parentFieldJsonName,
	}
	// 从[]*struct 转为 *struct
	sliceElementType := t.Elem()
	sliceMeta.originalType = sliceElementType

	if sliceElementType.Kind() == reflect.Ptr {
		// 从 *struct 变为 struct
		sliceElementType = sliceElementType.Elem()
		sliceMeta.isPtr = true
	}
	sliceMeta.elemType = sliceElementType

	sliceMeta.isStruct = sliceElementType.Kind() == reflect.Struct
	if sliceMeta.isStruct {
		sliceMeta.structMeta = parseStruct(&sliceElementType, parentFieldJsonName+".#")
	}

	return sliceMeta
}
