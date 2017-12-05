package reversetunnel

import (
	"logger"
	"net/http"
	"runtime/debug"
)

type buildTunnelReq struct {
	MPort      string `json:"m_port"`
	SPort      string `json:"s_port"`
	SlaverName string `json:"s_name"`
}

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

func (m *masterServer) buildTunnelHandler(w http.ResponseWriter,
	r *http.Request) (httpStatus int) {
	return
}
