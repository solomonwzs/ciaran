package reversetunnel

import (
	"encoding/json"
	"io/ioutil"
	"logger"
	"net/http"
	"runtime/debug"
)

type buildTunnelReq struct {
	MAddr      string `json:"m_addr"`
	SAddr      string `json:"s_addr"`
	SlaverName string `json:"s_name"`
}

func (m *masterServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%v %v\n%s", r.URL.Path, err,
				string(debug.Stack()))
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	if r.URL.Path == "/tunnel" {
		if r.Method == "POST" {
			m.buildTunnelHandler(w, r)
			return
		}
	}

	m.indexHandler(w, r)
}

func (m *masterServer) indexHandler(w http.ResponseWriter, r *http.Request) (
	httpStatus int) {
	w.Write([]byte("server running"))
	return http.StatusOK
}

func (m *masterServer) buildTunnelHandler(w http.ResponseWriter,
	r *http.Request) (httpStatus int) {

	httpStatus = http.StatusOK
	rep := []byte{}
	defer func() {
		w.WriteHeader(httpStatus)
		w.Write(rep)
	}()

	req := new(buildTunnelReq)
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Error(err)
		httpStatus = http.StatusInternalServerError
		return
	}

	if err = json.Unmarshal(body, req); err != nil {
		logger.Error(err)
		httpStatus = http.StatusBadRequest
		return
	}
	logger.Debugf("%+v\n", req)

	return
}
