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
	// split tag的key，value用英文逗号分隔
	split      = ","
	tagBind    = "bind"
	tagDefault = "default"
	tagPre     = "pre"

	// 参数来源
	header = 1 << 0
	query  = 1 << 1
	form   = 1 << 2
	path   = 1 << 3
	json   = 1 << 4
	auto   = math.MaxInt32

	// tagBind 的选项
	bindIgnore   = "-"
	bindAuto     = "auto"
	bindHeader   = "header"
	bindQuery    = "query"
	bindForm     = "form"
	bindPath     = "path"
	bindJson     = "json"
	bindRequired = "required"
	bindReq      = "req"
)

var sourceMap = map[string]int{
	bindAuto:   auto,
	bindHeader: header,
	bindQuery:  query,
	bindForm:   form,
	bindPath:   path,
	bindJson:   json,
}

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
		f.parentSliceIdx = num
		if f.isSlice {
			f.sliceMeta.fieldJsonName = strings.Replace(f.sliceMeta.fieldJsonName, "#", strconv.Itoa(num), 1)
			f.sliceMeta.structMeta.attachLayerNum(num)
			for _, sd := range f.sliceMeta.structData {
				sd.attachLayerNum(num)
			}
		} else if f.isStruct {
			f.structMeta.attachLayerNum(num)
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
	isIgnored bool

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

	parentSliceIdx int

	// Field来源，Query,Body,Header
	source int

	// 是否是必传的参数
	isRequired bool

	// 有没有设置default值
	hasDefault bool

	preprocessor []string

	// default值
	defaultVal string

	value *reflect.Value

	hasValue bool

	isUnset bool

	hasConversionError bool

	errs []error
}

func (field *fieldMetadata) setValue(recv reflect.Value) {
	if field.hasValue {
		v := *field.value
		if field.isPtr {
			ptr := reflect.New(v.Type())
			ptr.Elem().Set(v)
			recv.Set(ptr)
		} else {
			recv.Set(v.Convert(field.elemType))
		}
	} else {
		v := reflect.Zero(field.originalType)
		recv.Set(v)
	}
}

func (field *fieldMetadata) clone() *fieldMetadata {
	clone := *field
	if field.structMeta != nil {
		clone.structMeta = field.structMeta.clone()
	}

	if field.sliceMeta != nil {
		clone.sliceMeta = field.sliceMeta.clone()
	}

	return &clone
}

func (field *fieldMetadata) parseTag() {
	tagInfo := field.tagInfo

	// parse default tag
	defaultStr, ok := tagInfo.Lookup(tagDefault)
	if ok {
		field.hasDefault = true
		field.defaultVal = defaultStr
	}

	// parse preprocessor tag
	field.preprocessor = strings.Split(tagInfo.Get(tagPre), split)

	// parse bind tag
	bindTag := tagInfo.Get(tagBind)
	bindTags := strings.Split(bindTag, split)
	isSourceSet := false
	for _, value := range bindTags {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		if source, ok := sourceMap[value]; ok {
			field.source |= source
			isSourceSet = true
		} else {
			switch value {
			case bindIgnore:
				field.isIgnored = true
				isSourceSet = true
			case bindRequired, bindReq:
				field.isRequired = true
			default:
				field.fieldJsonName = strings.TrimSuffix(field.fieldJsonName, field.fieldName)
				field.fieldJsonName = field.fieldJsonName + value
				field.fieldName = value
			}
		}
	}

	if !isSourceSet {
		field.source |= auto
	}
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

		fieldMetaList[i] = fieldMeta

		fieldMeta.parseTag()
		if fieldMeta.isIgnored {
			continue
		}

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
	}

	return &StructMetadata{
		StructType: t,
		FieldNum:   numField,
		StructName: t.Name(),
		FieldList:  fieldMetaList,
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
