package binding

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryString(t *testing.T) {
	type metric string
	type count int32

	type Recv struct {
		X *struct {
			A []string  `bind:"a,query"`
			B string    `bind:"b,query"`
			C *[]string `bind:"c,query,req"`
			D *string   `bind:"d,query"`
			E *[]*int   `bind:"e,query"`
			F metric    `bind:"f,query"`
			G count     `bind:"g,query"`
			I metric    `bind:"i,query" default:"def"`
		}
		Y string  `bind:"y,query,req"`
		Z *string `bind:"z,query"`
		H string  `bind:"h,query,req"`
	}
	req, _ := http.NewRequest("POST", "http://localhost:8080?a=a1&a=a2&b=b1&c=c1&c=c2&d=d1&d=d&f=qps&g=1002&e=&e=2&y=y1", nil)
	recv := new(Recv)
	err := Bind(WrapHTTPRequest(req), recv)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "field=h, cause=field required but not found"))
	assert.Equal(t, []string{"a1", "a2"}, recv.X.A)
	assert.Equal(t, "y1", recv.Y)
	assert.Equal(t, (*string)(nil), recv.Z)

	assert.Equal(t, (*int)(nil), (*recv.X.E)[0])
	assert.Equal(t, 2, *((*recv.X.E)[1]))
	assert.Equal(t, "b1", (recv.X).B)
	assert.Equal(t, []string{"c1", "c2"}, *(recv.X.C))
	assert.Equal(t, "d1", *((*recv.X).D))
	assert.Equal(t, metric("qps"), recv.X.F)
	assert.Equal(t, count(1002), recv.X.G)
	assert.Equal(t, metric("def"), recv.X.I)

}

func TestAutoNum(t *testing.T) {
	type Recv struct {
		A  int8    `bind:"auto"`
		A2 int16   `bind:"A,auto"`
		B  int16   `bind:"auto"`
		C  int32   `bind:"auto"`
		D  int64   `bind:"auto"`
		E  uint8   `bind:"auto"`
		F  uint16  `bind:"auto"`
		G  uint32  `bind:"auto"`
		H  uint64  `bind:"auto"`
		I  float32 `bind:"auto"`
		J  float64 `bind:"auto"`
		K  string  `bind:"auto"`
		L  []int32 `bind:"auto"`
		M  int     `bind:"auto" default:"99"`
	}
	req, _ := http.NewRequest("POST", "http://localhost:8080?A=1&B=2&C=3&D=4&E=5&F=6&G=7&H=8&I=1.123&J=2.11&K=abc&L=1&L=2", nil)
	recv := new(Recv)
	sm := ParseStruct(recv)
	err := BindWithStructMeta(WrapHTTPRequest(req), recv, sm)
	assert.NoError(t, err)
	assert.Equal(t, int8(1), recv.A)
	assert.Equal(t, int16(1), recv.A2)
	assert.Equal(t, int16(2), recv.B)
	assert.Equal(t, int32(3), recv.C)
	assert.Equal(t, int64(4), recv.D)
	assert.Equal(t, uint8(5), recv.E)
	assert.Equal(t, uint16(6), recv.F)
	assert.Equal(t, uint32(7), recv.G)
	assert.Equal(t, uint64(8), recv.H)
	assert.Equal(t, float32(1.123), recv.I)
	assert.Equal(t, 2.11, recv.J)
	assert.Equal(t, "abc", recv.K)
	assert.Equal(t, int32(1), recv.L[0])
	assert.Equal(t, int32(2), recv.L[1])
	assert.Equal(t, 99, recv.M)
}

func TestJson(t *testing.T) {
	type TestJsonT struct {
		A struct {
			A1 int `bind:"auto"`
			B  []struct {
				B1 string `bind:"auto"`
				B2 string `bind:"auto" default:"def123"`
				C  []*struct {
					C1 string `bind:"auto,req"`
				} `bind:"auto,req"`
			} `bind:"auto"`
		} `bind:"auto"`
	}

	req, _ := http.NewRequest("POST", "http://localhost:8080/?a=a1&a=a2&b=b1&c=c1&c=c2&d=d1&d=d&f=qps&g=1002&e=&e=2&y=y1",
		strings.NewReader("{\"A\":{\"A1\":1,\"B\":[{\"B1\":\"A.B.1.B1\",\"B2\":\"A.B.1.B2\"},{\"B1\":\"A.B.2.B1\"},{\"B1\":\"A.B.3.B1\",\"C\":[{\"C1\":\"A.B.3.C.1.C1\"},{\"C1\":\"A.B.3.C.2.C1\"}]}]}}"))
	recv := new(TestJsonT)
	err := Bind(WrapHTTPRequest(req), recv)
	assert.Error(t, err)
	assert.Equal(t, 1, recv.A.A1)
	assert.Equal(t, "A.B.1.B1", recv.A.B[0].B1)
	assert.Equal(t, "A.B.1.B2", recv.A.B[0].B2)
	assert.Equal(t, "def123", recv.A.B[1].B2)
	assert.Equal(t, "def123", recv.A.B[2].B2)
	assert.Equal(t, "A.B.2.B1", recv.A.B[1].B1)
	assert.Equal(t, "A.B.3.B1", recv.A.B[2].B1)
	assert.Equal(t, "A.B.3.C.1.C1", recv.A.B[2].C[0].C1)
	assert.Equal(t, "A.B.3.C.2.C1", recv.A.B[2].C[1].C1)
}

func TestQueryNum(t *testing.T) {
	type Recv struct {
		X *struct {
			A []int     `bind:"a,query"`
			B int32     `bind:"b,query"`
			C *[]uint16 `bind:"c,query,req"`
			D *float32  `bind:"d,query"`
		}
		Y bool   `bind:"y,query,req"`
		Z *int64 `bind:"z,query"`
	}
	req, _ := http.NewRequest("POST", "http://localhost:8080/?a=11&a=12&b=21&c=31&c=32&d=41&d=42&y=true", nil)
	recv := new(Recv)
	err := Bind(WrapHTTPRequest(req), recv)
	assert.NoError(t, err)
	assert.Equal(t, []int{11, 12}, (*recv.X).A)
	assert.Equal(t, int32(21), (*recv.X).B)
	assert.Equal(t, &[]uint16{31, 32}, (*recv.X).C)
	assert.Equal(t, float32(41), *(*recv.X).D)
	assert.Equal(t, true, recv.Y)
	assert.Equal(t, (*int64)(nil), recv.Z)
}

func TestHeaderString(t *testing.T) {
	type Recv struct {
		X *struct {
			A []string  `bind:"X-A,header"`
			B string    `bind:"X-B,header"`
			C *[]string `bind:"X-C,header,req"`
			D *string   `bind:"X-D,header"`
		}
		Y string  `bind:"X-Y,header,req"`
		Z *string `bind:"X-Z,header"`
	}
	header := make(http.Header)
	header.Add("X-A", "a1")
	header.Add("X-A", "a2")
	header.Add("X-B", "b1")
	header.Add("X-C", "c1")
	header.Add("X-C", "c2")
	header.Add("X-D", "d1")
	header.Add("X-D", "d2")
	header.Add("X-Y", "y1")

	req, _ := http.NewRequest("POST", "http://localhost:8080", nil)
	req.Header = header
	recv := new(Recv)
	err := Bind(WrapHTTPRequest(req), recv)
	assert.NoError(t, err)
	assert.Equal(t, []string{"a1", "a2"}, (*recv.X).A)
	assert.Equal(t, "b1", (*recv.X).B)
	assert.Equal(t, []string{"c1", "c2"}, *(*recv.X).C)
	assert.Equal(t, "d1", *(*recv.X).D)
	assert.Equal(t, "y1", recv.Y)
	assert.Equal(t, (*string)(nil), recv.Z)
}

func TestHeaderNum(t *testing.T) {
	type Recv struct {
		X *struct {
			A []int     `bind:"X-A,header"`
			B int32     `bind:"X-B,header"`
			C *[]uint16 `bind:"X-C,header,req"`
			D *float32  `bind:"X-D,header"`
		}
		Y bool   `bind:"X-Y,header,req"`
		Z *int64 `bind:"X-Z,header"`
	}
	header := make(http.Header)
	header.Add("X-A", "11")
	header.Add("X-A", "12")
	header.Add("X-B", "21")
	header.Add("X-C", "31")
	header.Add("X-C", "32")
	header.Add("X-D", "41")
	header.Add("X-D", "42")
	header.Add("X-Y", "true")

	req, _ := http.NewRequest("POST", "http://localhost:8080", nil)
	req.Header = header
	recv := new(Recv)
	err := Bind(WrapHTTPRequest(req), recv)
	assert.NoError(t, err)
	assert.Equal(t, []int{11, 12}, (*recv.X).A)
	assert.Equal(t, int32(21), (*recv.X).B)
	assert.Equal(t, &[]uint16{31, 32}, (*recv.X).C)
	assert.Equal(t, float32(41), *(*recv.X).D)
	assert.Equal(t, true, recv.Y)
	assert.Equal(t, (*int64)(nil), recv.Z)
}

func TestFormString(t *testing.T) {
	type Recv struct {
		X *struct {
			A []string  `bind:"a,form"`
			B string    `bind:"b,form"`
			C *[]string `bind:"c,form,req"`
			D *string   `bind:"d,form"`
		}
		Y string  `bind:"y,form,req"`
		Z *string `bind:"z,form"`
	}
	values := make(url.Values)
	values.Add("a", "a1")
	values.Add("a", "a2")
	values.Add("b", "b1")
	values.Add("c", "c1")
	values.Add("c", "c2")
	values.Add("d", "d1")
	values.Add("d", "d2")
	values.Add("y", "y1")

	header := make(http.Header)
	header.Set("Content-Type", "application/x-www-form-urlencoded")
	req, _ := http.NewRequest("POST", "http://localhost:8080", strings.NewReader(values.Encode()))
	req.Header = header
	req.PostForm = values

	recv := new(Recv)
	err := Bind(WrapHTTPRequest(req), recv)
	assert.NoError(t, err)
	assert.Equal(t, []string{"a1", "a2"}, (*recv.X).A)
	assert.Equal(t, "b1", (*recv.X).B)
	assert.Equal(t, []string{"c1", "c2"}, *(*recv.X).C)
	assert.Equal(t, "d1", *(*recv.X).D)
	assert.Equal(t, "y1", recv.Y)
	assert.Equal(t, (*string)(nil), recv.Z)

}

func TestFormNum(t *testing.T) {
	type Recv struct {
		X *struct {
			A []int     `bind:"a,form"`
			B int32     `bind:"b,form"`
			C *[]uint16 `bind:"c,form,req"`
			D *float32  `bind:"d,form"`
		}
		Y bool   `bind:"y,form,req"`
		Z *int64 `bind:"z,form"`
	}
	values := make(url.Values)
	values.Add("a", "11")
	values.Add("a", "12")
	values.Add("b", "-21")
	values.Add("c", "31")
	values.Add("c", "32")
	values.Add("d", "41")
	values.Add("d", "42")
	values.Add("y", "1")

	header := make(http.Header)
	header.Set("Content-Type", "application/x-www-form-urlencoded")
	req, _ := http.NewRequest("POST", "http://localhost:8080", strings.NewReader(values.Encode()))
	req.Header = header
	req.PostForm = values

	recv := new(Recv)
	err := Bind(WrapHTTPRequest(req), recv)
	assert.NoError(t, err)
	assert.Equal(t, []int{11, 12}, (*recv.X).A)
	assert.Equal(t, int32(-21), (*recv.X).B)
	assert.Equal(t, &[]uint16{31, 32}, (*recv.X).C)
	assert.Equal(t, float32(41), *(*recv.X).D)
	assert.Equal(t, true, recv.Y)
	assert.Equal(t, (*int64)(nil), recv.Z)
}

func TestJSON(t *testing.T) {
	// binding.ResetJSONUnmarshaler(false, json.Unmarshal)
	type metric string
	type count int32
	type ZS struct {
		Z *int64 `bind:"json"`
	}
	type Recv struct {
		X *struct {
			A []string  `bind:"a,json"`
			B int32     `bind:"json"`
			C *[]uint16 `bind:"json,req"`
			D *float32  `bind:"d,json"`
			E metric    `bind:"e,json"`
			F count     `bind:"f,json"`
			// M map[string]string `bind:"m,json"`
		} `bind:"X,json"`
		Y  string `bind:"y,json,req"`
		ZS `bind:"auto"`
	}

	bodyReader := strings.NewReader(`{
		"X": {
			"a": ["a1","a2"],
			"B": 21,
			"C": [31,32],
			"d": 41,
			"e": "qps",
			"f": 100,
			"m": {"a":"x"}
		},
		"Z": 6
	}`)

	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	req, _ := http.NewRequest("POST", "http://localhost:8080", bodyReader)
	req.Header = header

	recv := new(Recv)
	err := Bind(WrapHTTPRequest(req), recv)
	assert.Error(t, err)
	assert.Equal(t, []string{"a1", "a2"}, (*recv.X).A)
	assert.Equal(t, int32(21), (*recv.X).B)
	assert.Equal(t, &[]uint16{31, 32}, (*recv.X).C)
	assert.Equal(t, float32(41), *(*recv.X).D)
	assert.Equal(t, metric("qps"), (*recv.X).E)
	assert.Equal(t, count(100), (*recv.X).F)
	// assert.Equal(t, map[string]string{"a": "x"}, (*recv.X).M)
	assert.Equal(t, "", recv.Y)
	assert.Equal(t, (int64)(6), *recv.Z)
}
