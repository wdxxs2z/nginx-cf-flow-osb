package route

import (
	"text/template"
	"io/ioutil"
	"os"
)

type NginxService struct {
	ServiceId       string				`json:"service_id"`
	Host            string                          `json:"host"`
	Domain          string                          `json:"domain"`
	Nginxs		[]Nginx				`json:"nginxs"`
}

type Nginx struct {
	Name		string				`json:"name"`
	Url		string				`json:"url"`
	Weight          int64				`json:"weight"`
	Port            int				`json:"port"`
}

func ParseNginxTemplate(nginxTemplFile string, nginsService NginxService, destinationFile string) (error){
	input, ioErr := ioutil.ReadFile(nginxTemplFile)
	if ioErr != nil {
		return ioErr
	}
	nginxTemplate, err := template.New("nginx").Parse(string(input))
	if err != nil {
		return err
	}
	desfile, err := os.Create(destinationFile)
	if err != nil {
		return err
	}
	defer desfile.Close()
	return nginxTemplate.Execute(desfile, nginsService)
}