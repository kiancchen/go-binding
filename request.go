package binding

import (
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
)

const (
	defaultMaxMemory = 32 << 20 // 32 MB
)

type Request interface {
	GetMethod() string
	GetQuery() url.Values
	GetPathParam(key string) (string, bool)
	GetContentType() string
	GetHeader() http.Header
	GetCookies() []*http.Cookie
	GetPostForm() (url.Values, error)
	GetFormFile() (map[string][]*multipart.FileHeader, error)
	GetBody() ([]byte, error)
}

func WrapHTTPRequest(req *http.Request) Request {
	req.ParseMultipartForm(defaultMaxMemory)
	return &httpRequest{
		Request: req,
	}
}

type httpRequest struct {
	*http.Request
}

func (r *httpRequest) GetMethod() string {
	return r.Method
}
func (r *httpRequest) GetQuery() url.Values {
	return r.URL.Query()
}

func (r *httpRequest) GetPathParam(key string) (string, bool) {
	return "", false
}

func (r *httpRequest) GetContentType() string {
	return r.GetHeader().Get("Content-Type")
}

func (r *httpRequest) GetHeader() http.Header {
	return r.Header
}

func (r *httpRequest) GetCookies() []*http.Cookie {
	return r.Cookies()
}

func (r *httpRequest) GetPostForm() (url.Values, error) {
	return r.PostForm, nil
}

func (r *httpRequest) GetFormFile() (map[string][]*multipart.FileHeader, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	return r.MultipartForm.File, nil
}

func (r *httpRequest) GetBody() ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	buf, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}

	return buf, nil
}

type request struct {
	header       http.Header
	query        url.Values
	getPathParam func(key string) (string, bool)
	method       string
	contentType  string
	postForm     url.Values
	body         []byte
	cookie       []*http.Cookie
	formFile     map[string][]*multipart.FileHeader
}

func newRequest(r Request) (*request, error) {
	postForm, err := r.GetPostForm()
	if err != nil {
		return nil, err
	}

	body, err := r.GetBody()
	if err != nil {
		return nil, err
	}

	formFile, err := r.GetFormFile()
	if err != nil {
		return nil, err
	}

	return &request{
		header:       r.GetHeader(),
		query:        r.GetQuery(),
		getPathParam: r.GetPathParam,
		method:       r.GetMethod(),
		contentType:  r.GetContentType(),
		postForm:     postForm,
		body:         body,
		cookie:       r.GetCookies(),
		formFile:     formFile,
	}, nil
}

func (r request) GetMethod() string {
	return r.method
}

func (r request) GetQuery(key string) ([]string, bool) {
	v, ok := r.query[key]
	return v, ok
}

func (r request) GetContentType() string {
	return r.contentType
}

func (r request) GetHeader(key string) ([]string, bool) {
	v, ok := r.header[key]
	return v, ok
}

func (r request) GetCookies(key string) *http.Cookie {
	for _, cookie := range r.cookie {
		if cookie.Name == key {
			return cookie
		}
	}
	return nil
}

func (r request) GetFormFile(key string) ([]*multipart.FileHeader, bool) {
	v, ok := r.formFile[key]
	return v, ok
}

func (r request) GetPostForm(key string) ([]string, bool) {
	v, ok := r.postForm[key]
	return v, ok
}

func (r request) GetBody() []byte {
	return r.body
}
