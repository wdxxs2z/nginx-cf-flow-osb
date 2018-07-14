package main

import (
	"github.com/wdxxs2z/nginx-flow-osb/route"
	"fmt"
	"os"
	"path/filepath"
	"log"
	"encoding/json"
	"bytes"
)

func main() {

	ns := route.NginxService{
		ServiceId:	"64e82332-b919-4188-bb3e-14103ff0e1bd",
		Nginxs:         make([]route.Nginx,2),
	}
	n1 := route.Nginx{
		Name:		"fakea",
		Url:		"fakea.dcos.os",
		Weight:         4,
		Port:           8001,
	}
	n2 := route.Nginx{
		Name:		"fakeb",
		Url:		"fakeb.dcos.os",
		Weight:         6,
		Port:           8002,
	}
	ns.Nginxs = []route.Nginx{n1, n2}
	j,err := json.Marshal(ns)
	if err != nil {
		log.Fatal(err)
	}
	var out bytes.Buffer
	err = json.Indent(&out, j, "", "\t")
	if err != nil {
		log.Fatalln(err)
	}
	out.WriteTo(os.Stdout)
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(dir)
	err = route.ParseNginxTemplate("f:/nginx.conf.templ", ns, "f:/nginx.conf")
	if err != nil {
		fmt.Println(err)
	}
}
