package web

import (
	"errors"
	"logger"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

var (
	ErrDispathNotFound         = errors.New("web: not found")
	ErrDispathMethodNotAllowed = errors.New("web: method not allowed")
)

const (
	_X_FORWARDED_FOR = "X-Forwarded-For"
	_X_REAL_IP       = "X-Real-Ip"
)

type ServeFunc func(w http.ResponseWriter, r *http.Request) (httpstatus int)

type Dispath func(path, method string) (f ServeFunc, err error)

type BasicServer struct {
	dispath Dispath
	address string
}

func GetRemoteAddr(r *http.Request) (remoteIP string) {
	remoteAddr := strings.Split(r.RemoteAddr, ":")
	return remoteAddr[0]
}

func GetHeaderIP(r *http.Request, header string) (remoteIP string) {
	remoteIP = r.Header.Get(header)
	if remoteIP != "" {
		remoteIPS := strings.Split(remoteIP, ",")
		remoteIP = strings.TrimSpace(remoteIPS[len(remoteIPS)-1])
	}
	return
}

func GetRemoteIP(r *http.Request) (remoteIP string) {
	remoteIP = GetHeaderIP(r, _X_FORWARDED_FOR)
	if remoteIP == "" {
		remoteIP = GetHeaderIP(r, _X_REAL_IP)
	}
	if remoteIP == "" {
		remoteIP = GetRemoteAddr(r)
	}
	return
}

func NewBasicServer(dispath Dispath, address string) *BasicServer {
	return &BasicServer{
		dispath: dispath,
		address: address,
	}
}

func (s *BasicServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var httpStatus int
	f, err := s.dispath(r.URL.Path, r.Method)
	if err == nil {
		httpStatus = safeRun(f, w, r)
	} else if err == ErrDispathNotFound {
		httpStatus = http.StatusNotFound
	} else if err == ErrDispathMethodNotAllowed {
		httpStatus = http.StatusMethodNotAllowed
	} else {
		httpStatus = http.StatusInternalServerError
	}
	timeCost := (time.Now().UnixNano() - start.UnixNano()) / 1000000
	remoteIP := GetRemoteIP(r)
	logger.Info(httpStatus, r.Method, r.URL.Path, remoteIP, timeCost)
}

func safeRun(f ServeFunc, w http.ResponseWriter, r *http.Request) (
	httpStatus int) {
	httpStatus = http.StatusOK
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%v %v\n%s", r.URL.Path, err,
				string(debug.Stack()))
			httpStatus = http.StatusInternalServerError
		}
	}()
	return f(w, r)
}

func (s *BasicServer) Run() {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		logger.Error(err)
		return
	}
	defer listener.Close()
	http.Serve(listener, s)
}
