package reversetunnel

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"logger"
	"net"
	"time"
)

func setTimeout(conn net.Conn, d time.Duration) error {
	return conn.SetDeadline(time.Now().Add(d))
}

func Main() {
	logger.AddLogger("default", nil)

	confFile := flag.String("f", "", "config file")
	flag.Parse()

	conf := new(config)
	if data, err := ioutil.ReadFile(*confFile); err != nil {
		panic(err)
	} else if err = json.Unmarshal(data, conf); err != nil {
		panic(err)
	}

	if conf.Role == "master" {
		m := newMasterServer(conf)
		m.run()
	} else {
		panic(fmt.Sprintf("unknown role %s", conf.Role))
	}

	// fmt.Println([]byte(net.ParseIP("fe80::ba97:5aff:fe9e:4abf")))
	// fmt.Println([]byte(net.ParseIP("10.0.0.128")))
}
