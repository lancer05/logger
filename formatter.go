package logger

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Schema 日志规范
type Schema string

const (
	// LogsV1 运行日志
	SchemaGeneralLogsV1 Schema = "general.logs.v1"
	// HTTPRequestV1 请求日志
	SchemaHTTPRequestV1 Schema = "http.request.v1"
)

var (
	// MaxStackTrace 记录的错误信息的调用栈最大深度
	MaxStackTrace = 10

	_ logrus.Formatter = (*LogsV1Formatter)(nil)

	emptyStack = make([]string, 0)

	logsV1Pool = sync.Pool{
		New: func() interface{} {
			return &LogsV1{}
		},
	}
)

// NewFormatter 获得日志规范对应的格式化对象
func NewFormatter(service, env string) logrus.Formatter {
	return &LogsV1Formatter{
		TimeLayout:  "2006-01-02T15:04:05.999Z07:00",
		Service:     service,
		Environment: env,
	}
}

// LogsV1 logs.v1 日志输出内容
type LogsV1 struct {
	Schema      string                 `json:"schema"`
	Time        string                 `json:"t"`
	Level       string                 `json:"l"`
	Service     string                 `json:"s"`
	Channel     string                 `json:"c"`
	Environment string                 `json:"e"`
	User        string                 `json:"u"`
	Message     string                 `json:"m"`
	Context     map[string]interface{} `json:"ctx"`
	Request     *RequestData           `json:"request,omitempty"`
}

// LogsV1Formatter 日志格式化
type LogsV1Formatter struct {
	// 时间格式，默认ISO8601，精确到秒
	TimeLayout  string
	Service     string
	Environment string
}

// RequestData 请求相关的参数
type RequestData struct {
	IP       string            `json:"ip"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"header"`
	Status   string            `json:"status"`
	Duration string            `json:"duration"`
	Param    logrus.Fields     `json:"param"`
}

// Format implements logrus.Formatter interface
func (af *LogsV1Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	channel := ""
	uid := ""
	status := ""
	duration := ""
	context := logrus.Fields{}
	schema := SchemaGeneralLogsV1

	// 先处理caller记录，允许entry.Data内的数据覆盖caller
	// 可以实现自行记录caller的目的
	if entry.HasCaller() {
		caller := entry.Caller
		context[logrus.FieldKeyFile] = fmt.Sprintf("%s:%d", caller.File, caller.Line)
		context[logrus.FieldKeyFunc] = caller.Function
	}

	for k, v := range entry.Data {
		switch k {
		case "channel":
			channel, _ = v.(string)
		case "request":
			continue
		case "user":
			uid = fmt.Sprintf("%v", v)
		case "status":
			status = fmt.Sprintf("%v", v)
		case "duration":
			duration = fmt.Sprintf("%v", v)

		default:
			if err, ok := v.(error); !ok {
				context[k] = v
			} else {
				msg, trace := extractError(err)
				errData := logrus.Fields{
					"msg": msg,
				}
				if len(trace) > 0 {
					errData["trace"] = trace
				}
				context[k] = errData
			}
		}
	}

	data := logsV1Pool.Get().(*LogsV1)
	data.Time = entry.Time.Format(af.TimeLayout)
	data.Level = entry.Level.String()
	data.Service = af.Service
	data.Channel = channel
	data.Environment = af.Environment
	data.Message = entry.Message
	data.Context = context
	data.User = uid
	defer logsV1Pool.Put(data)

	if rv, ok := entry.Data["request"]; ok {
		if req, ok := rv.(*http.Request); ok {
			schema = SchemaHTTPRequestV1
			data.Request = richRequest(req, status, duration)
		}
	}

	data.Schema = string(schema)

	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	if err := jsoniter.NewEncoder(b).Encode(data); err != nil {
		return nil, errors.Wrapf(err, "json encode %s log", schema)
	}

	return b.Bytes(), nil
}

func richRequest(req *http.Request, status, duration string) *RequestData {
	request := &RequestData{
		IP:       parseIP(req.RemoteAddr),
		Method:   req.Method,
		Path:     req.URL.Path,
		Status:   status,
		Duration: duration,
		Headers:  map[string]string{},
		Param:    logrus.Fields{},
	}

	// 获取 header信息
	for k, v := range req.Header {
		k = strings.ToLower(k)
		if len(v) > 1 {
			request.Headers[k] = strings.Join(v, ", ")
		} else {
			request.Headers[k] = v[0]
		}
	}

	// get方式参数
	if q := req.URL.Query(); len(q) > 0 {
		for k, v := range q {
			if len(v) > 1 {
				request.Param[k] = v
			} else {
				request.Param[k] = v[0]
			}
		}
	}

	if err := req.ParseForm(); err == nil {
		// postFrom 方式参数
		if postForm := req.PostForm; len(postForm) > 0 {
			for k, v := range postForm {
				if len(v) > 1 {
					request.Param[k] = v
				} else {
					request.Param[k] = v[0]
				}
			}
		}
	}

	// json 方式参数
	if strings.Contains(request.Headers["content-type"], "application/json") {
		if tmpBody, err := ioutil.ReadAll(req.Body); err == nil {
			req.Body = ioutil.NopCloser(bytes.NewReader(tmpBody))

			body := make(map[string]interface{})
			if err := jsoniter.NewDecoder(bytes.NewReader(tmpBody)).Decode(&body); err == nil {
				for k, v := range body {
					request.Param[k] = v
				}
			}
		}
	}

	return request
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

// stackTrace 从错误信息中获取调用栈信息
func stackTrace(err error) []string {
	if err, ok := err.(stackTracer); ok {
		return strings.Split(
			strings.ReplaceAll(
				strings.TrimLeft(
					fmt.Sprintf("%+v", err.StackTrace()),
					"\n",
				),
				"\n\t",
				" ",
			),
			"\n",
		)
	}

	return emptyStack
}

func extractError(err error) (string, []string) {
	var trace []string
	if st := stackTrace(err); len(st) > 0 {
		if len(st) >= MaxStackTrace {
			st = st[:MaxStackTrace]
		}
		trace = st
	}
	return err.Error(), trace
}
