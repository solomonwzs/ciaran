package reversetunnel

type config struct {
	Role       string `json:"role,omitempty"`
	Name       string `json:"name,omitempty"`
	ClientAddr string `json:"client_addr,omitempty"`
	CtrlAddr   string `json:"ctrl_addr,omitempty"`
	TunnelAddr string `json:"tunnel_addr,omitempty"`
	JoinAddr   string `json:"join,omitempty"`
}
