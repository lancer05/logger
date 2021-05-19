package logger

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

func TestFormatterOutput(t *testing.T) {
	entry := &logrus.Entry{
		Time: time.Now(),
		Data: logrus.Fields{},
	}

	query := url.Values{}
	query.Set("getPram", "bar")

	form := url.Values{}
	form.Set("formPram", "baz")

	body := io.NopCloser(strings.NewReader(`{"jsonParam":"brz"}`))

	headers := http.Header{}
	headers.Set("x-test", "1")
	headers.Set("Content-Type", "application/json; charset=UTF-8")

	req := &http.Request{
		RemoteAddr: "1.2.3.4:1234",
		Header:     headers,
		Method:     http.MethodPost,
		URL: &url.URL{
			Path:     "/api",
			RawQuery: query.Encode(),
		},
		PostForm: form,
		Body:     body,
	}

	entry.Data["request"] = req
	entry.Data["user"] = 65535
	entry.Data["status"] = http.StatusAccepted
	entry.Data["duration"] = 100
	entry.Data["error"] = errors.New("error occurred")

	cases := []struct {
		path     []interface{}
		expected string
	}{
		{
			path:     []interface{}{"schema"},
			expected: string(SchemaHTTPRequestV1),
		},
		{
			path:     []interface{}{"u"},
			expected: "65535",
		},
		{
			path:     []interface{}{"request", "ip"},
			expected: "1.2.3.4",
		},
		{
			path:     []interface{}{"request", "method"},
			expected: req.Method,
		},
		{
			path:     []interface{}{"request", "path"},
			expected: req.URL.Path,
		},

		{
			path:     []interface{}{"request", "param", "getPram"},
			expected: "bar",
		},
		{
			path:     []interface{}{"request", "param", "formPram"},
			expected: "baz",
		},
		{
			path:     []interface{}{"request", "param", "jsonParam"},
			expected: "brz",
		},
		{
			path:     []interface{}{"request", "status"},
			expected: fmt.Sprintf("%d", http.StatusAccepted),
		},
		{
			path:     []interface{}{"err"},
			expected: "error occurred",
		},
	}

	f := NewFormatter("test", "test")
	data, err := f.Format(entry)
	if err != nil {
		t.Fatalf("Format() error, Expected=nil, Actual=%q", err.Error())
	}

	// 两次Format是为了校验Format的幂等性
	data1, err := f.Format(entry)
	if err != nil {
		t.Fatalf("Format() error, Expected=nil, Actual=%q", err.Error())
	}

	for _, c := range cases {
		if v := jsoniter.Get(data, c.path...).ToString(); v != c.expected {
			t.Fatalf(`Format() output %q, Expecteded=%q, Actual=%q`, c.path, c.expected, v)
		}

		if v := jsoniter.Get(data1, c.path...).ToString(); v != c.expected {
			t.Fatalf(`Format() output %q, Expecteded=%q, Actual=%q`, c.path, c.expected, v)
		}
	}
}
