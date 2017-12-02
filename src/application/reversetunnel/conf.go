package reversetunnel

type config struct {
	Role       string `json:"role,omitempty"`
	ClientAddr string `json:"client_addr,omitempty"`
	CtrlAddr   string `json:"ctrl_addr,omitempty"`
}
