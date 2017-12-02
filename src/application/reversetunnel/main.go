package reversetunnel

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
)

func Main() {
	confFile := flag.String("f", "", "config file")
	flag.Parse()

	conf := new(config)
	if data, err := ioutil.ReadFile(*confFile); err != nil {
		panic(err)
	} else if err = json.Unmarshal(data, conf); err != nil {
		panic(err)
	}

	if conf.Role == "master" {
		masterServer(conf)
	} else {
		panic(fmt.Sprintf("unknown role %s", conf.Role))
	}
}
