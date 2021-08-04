# go-binding

支持将 HTTP 请求自动绑定到 struct 上

# 最小例子

```go
type Recv struct {
    X *struct {
        A int `bind:"auto"`
    } `bind:"auto"`
}
req, _ := http.NewRequest("POST", "/?A=1", nil)
recv := new(Recv)
err := Bind(WrapHTTPRequest(req), recv)
assert.NoError(t, err)
assert.Equal(t, 1, recv.X.A)
```

# 语法

## 绑定

```go
type S struct{
    A int `bind:"auto"` // 按顺序从 header, query, form, json 获取 A
    B int `bind:"b,auto"` // 从...获取 b
    C int `bind:"c,query"` // 从 query 获取 c
    D int `bind:"d,query,header,form,required"` // 按顺序从 header, query, form 获取 d，如果													没有获取到, 返回一个错误
    E int `bind:"auto,required"` // 从...获取 E, 如果没有获取到, 返回一个错误
    File *multipart.FileHeader `bind:"auto"` // 从 multipart form 获取 File 文件
}
```

支持从 `header`, `query`, `form`, `json` 获取参数。如果指定 `auto` 或者指定了多个来源，将会按前面这个顺序获取，直到值被取到，你指定的来源顺序将会被忽略。

## 预处理器

你可以注册自己的预处理器来处理获取到的值。这个处理的过程会在获得值之后完成，请确保你处理过的值可以被转换成对应的字段类型。

```go
// 一般在程序启动时注册好所有的处理器
RegisterPreprocessor("split", func(origin string) ([]string, error) {
		return strings.Split(origin, ","), nil
	})

// 绑定
type Recv struct {
    A []int `bind:"auto" pre:"split"`
}
req, _ := http.NewRequest("POST", "/?A=1,2&A=3,4", nil)
recv := new(Recv)
err := Bind(WrapHTTPRequest(req), recv)
assert.NoError(t, err)
assert.Equal(t, []int{1,2,3,4}, recv.X.A)
```

**一些使用场景:** 

- 将 string 分割成数组
- 给手机号码添加前缀
- 返回一个 error，可以当做校验器使用

## 自定义类型转换器

```go
//  一般在程序启动时注册好所有的类型转换器
RegisterTypeConvertor(time.Time{}, func(s string) (interface{}, error) {
    return time.Parse("2006-01-02", s)
})

// 绑定
type Recv struct {
    T time.Time `bind:"auto"`
}
req, _ := http.NewRequest("POST", "/?T=2021-08-04", nil)
recv := new(Recv)
err := Bind(WrapHTTPRequest(req), recv)
assert.NoError(t, err)
expect, _ := time.Parse("2006-01-02", "2021-08-04")
assert.Equal(t, expect, recv.J)
```

# 为什么选择这个库

## 更好地支持指针，数据和结构体

```go
type Recv struct{
    A int `bind:"auto"`            // non-pointer value
    B *int `bind:"auto"`           // pointer
    C []int `bind:"auto"`          // array
    D []*int `bind:"auto"`         // array of pointers
    E *[]*int `bind:"auto"`        // pointer of an array of pointers
    F *[]*struct{                  // dive into struct automatically
        Inner int `bind:"auto"`
        InnerArray []int `bind:"auto"`
    } `bind:"auto"`
    File *multipart.FileHeader `bind:"auto"`
}
```

## 更好地处理 required 或者有默认值(default)的字段

> 这也是为什么我写这个库，我试过现有的一些 binding 库，对于数组的支持都不算好，比如 required 或者 default 的字段都不会被检测到

```go
type People struct {
    Id     int      `bind:"auto" default:"99"`
    Name   string   `bind:"auto,required"`
}

type Recv struct {
    Peoples []*People `bind:"auto"`
}
```

**Input**:

```json
{
	"Peoples":[
		{
			"Name": "name",
			"Id": 1
		},
        {
            "Id": 2
        },
        {
            
        }
    ]
}
```

Result:

```json5
{
	"Peoples":[
		{
			"Name":"name",
			"Id": 1
		},
        {
            "Id": 2
        },
        {
            "Id": 99  // default value
        }
    ]
}
```

返回的错误: `parameter required but not found: [Peoples.1.Name, Peoples.2.Name]`, 字段 `Name` 在数组`Peoples` 的第一个和第二个都缺失了。

