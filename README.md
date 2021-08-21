# go-binding

A library that bind http request to a struct.

[中文](README-zh.md)

# Minimal example

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

# Syntax

## Bind

```go
type S struct{
    A  int `bind:"auto"` // get A form header, query, form, json in order
    A2 int               // field with no tag will be bind, same as 'auto'
    B  int `bind:"b,auto"` // get b from .... 
    C  int `bind:"c,query"` // get c from query
    D  int `bind:"d,query,header,form,required"` // get d from header, query, form and 													return an error if not provided
    E  int `bind:"auto,required"` // get E from ..... and return an error if not provided
    F int `bind:"-"` // ignore this field
    File *multipart.FileHeader `bind:"auto"` // get File from multipart form
}
```

The library supports get value from `header`, `query`, `form`, `json`. If you specify `auto` or multiple sources, it will get value in that order until the value obtained, regardless of the order you specify.

## Preprocessor

You can register a preprocessor that process the value obtained. This process is done after obtain the value immediately, make sure the processed value can be converted to the corresponding field type.

```go
// register the preprocessor when the program starts up
RegisterPreprocessor("split", func(origin string) ([]string, error) {
		return strings.Split(origin, ","), nil
	})

// bind
type Recv struct {
    A []int `bind:"auto" pre:"split"`
}
req, _ := http.NewRequest("POST", "/?A=1,2&A=3,4", nil)
recv := new(Recv)
err := Bind(WrapHTTPRequest(req), recv)
assert.NoError(t, err)
assert.Equal(t, []int{1,2,3,4}, recv.X.A)
```

**some usage scenario:** 

- split a string into an array
- add an area code prefix to a phone number
- do some checks like a validator, just return an error

## Custom convertor

```go
// register the convertor
RegisterTypeConvertor(time.Time{}, func(s string) (interface{}, error) {
    return time.Parse("2006-01-02", s)
})

// bind
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

# Why use this but not others

## support pointer, array and struct well

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

## solve required and default fields better

> This reason is also why I write this library, because I found other existing binding libraries can't handle required and default fields well in an array.

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

And return an error: `parameter required but not found: [Peoples.1.Name, Peoples.2.Name]`, which specifies the field `Name` is missing in the 1th and 2rd elements in the array `Peoples`.

