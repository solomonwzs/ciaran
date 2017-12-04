package reversetunnel

import (
	"logger"
	"net/http"
	"runtime/debug"
)

func (m *masterServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%v %v\n%s", r.URL.Path, err,
				string(debug.Stack()))
		}
	}()

	m.indexHandler(w, r)
}

func (m *masterServer) indexHandler(w http.ResponseWriter, r *http.Request) (
	httpStatus int) {
	w.Write([]byte("server running"))
	return http.StatusOK
}
