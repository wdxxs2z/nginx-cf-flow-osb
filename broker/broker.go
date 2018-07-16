package broker

import (
	"os"
	"log"
	"fmt"
	"context"
	"strings"
	"net/http"
	"encoding/json"

	"github.com/gorilla/mux"
	"github.com/gorilla/handlers"
	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"

	"github.com/wdxxs2z/nginx-flow-osb/config"
	"github.com/wdxxs2z/nginx-flow-osb/route"
	"github.com/wdxxs2z/nginx-flow-osb/utils"
	cfClient "github.com/wdxxs2z/nginx-flow-osb/client"
	"github.com/wdxxs2z/nginx-flow-osb/db"
)

type ProvisionParameters map[string]interface{}

type BindParameters map[string]interface{}

type NginxDataflowServiceBroker struct {
	allowUserProvisionParameters 	bool
	allowUserUpdateParameters    	bool
	allowUserBindParameters      	bool
	logger                  	lager.Logger
	brokerRouter			*mux.Router
	databaseClient                  *db.DBClient
	config                          config.Config
}

func New(config config.Config, logger lager.Logger) *NginxDataflowServiceBroker{
	brokerRouter := mux.NewRouter()
	dbClient, err := db.NewDBClient(config, logger)
	if err != nil {
		logger.Error("Error-create-db-client", err, lager.Data{})
		return nil
	}
	if err := dbClient.MigrateServiceInstanceTable(); err != nil {
		logger.Error("Error-migrate-servicetable", err, lager.Data{})
		return nil
	}
	broker := &NginxDataflowServiceBroker{
		allowUserBindParameters:	config.AllowUserBindParameters,
		allowUserProvisionParameters:   config.AllowUserProvisionParameters,
		allowUserUpdateParameters:      config.AllowUserUpdateParameters,
		logger:				logger.Session("osb-api"),
		brokerRouter:                   brokerRouter,
		databaseClient:                 dbClient,
		config:                         config,
	}
	brokerapi.AttachRoutes(broker.brokerRouter, broker, logger)
	liveness := broker.brokerRouter.HandleFunc("/liveness", livenessHandler).Methods(http.MethodGet)

	broker.brokerRouter.Use(authHandler(config, map[*mux.Route]bool{liveness: true}))
	broker.brokerRouter.Use(handlers.ProxyHeaders)
	broker.brokerRouter.Use(handlers.CompressHandler)
	broker.brokerRouter.Use(handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{http.MethodHead, http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions}),
		handlers.AllowCredentials(),
	))

	return broker
}

func (nsb  *NginxDataflowServiceBroker)Run(address string)  {
	log.Println("Nginx dataflow service broker started on port " + strings.TrimPrefix(address, ":"))
	log.Fatal(http.ListenAndServe(address, nsb.brokerRouter))
}

func (nsb *NginxDataflowServiceBroker)Services(context context.Context) ([]brokerapi.Service, error){
	nsb.logger.Debug("service-catalog",lager.Data{})

	nginxDataflowServices := nsb.config.Services

	var services []brokerapi.Service

	for _, nginxService := range nginxDataflowServices {

		services = append(services, brokerapi.Service{
			ID:			nginxService.Id,
			Name:           	nginxService.Name,
			Description:    	nginxService.Description,
			Bindable:       	nginxService.Bindable,
			Tags:           	nginxService.Tags,
			PlanUpdatable:  	nginxService.PlanUpdateable,
			Metadata:       	&brokerapi.ServiceMetadata{
				DisplayName:		nginxService.Metadata.DisplayName,
				ImageUrl:               nginxService.Metadata.ImageUrl,
				LongDescription:	nginxService.Metadata.LongDescription,
				ProviderDisplayName:    nginxService.Metadata.ProviderDisplayName,
				DocumentationUrl:	nginxService.Metadata.DocumentationUrl,
				SupportUrl:		nginxService.Metadata.SupportUrl,
			},
			Plans:          	servicePlans(nginxService.Plans),
		})
	}
	return services,nil
}

func (nsb *NginxDataflowServiceBroker)Provision(context context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	nsb.logger.Debug("provision-service-instance", lager.Data{
		"instanceId": instanceID,
	})
	service, _ := nsb.GetService(details.ServiceID)
	if service.Name == "" {
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("service (%s) not found in catalog", details.ServiceID)
	}
	exist, err := nsb.databaseClient.ExistServiceInstance(instanceID)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}
	if exist == true {
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("service instance (%s) already created", instanceID)
	}
	if nsb.allowUserProvisionParameters && len(details.GetRawParameters()) >0 {
		provisionParameters := ProvisionParameters{}
		sourceDir := nsb.config.StoreDataDir + instanceID
		destinationDir := nsb.config.StoreDataDir + instanceID + "/" + instanceID + ".zip"
		if jsonErr := json.Unmarshal(details.RawParameters, &provisionParameters); jsonErr != nil {
			return brokerapi.ProvisionedServiceSpec{}, jsonErr
		}
		ns , err := nsb.ParseParameters(instanceID, provisionParameters)
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("parse parameter error: %s", err)
		}
		err = nsb.PreparePushDir(instanceID, ns)
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("prepare push director err: %s", err)
		}
		_, err = cfClient.CreateApplicationWorkflow("nginx-flow-" + instanceID, nsb.config.ServiceSpace, ns.Host, ns.Domain, sourceDir, destinationDir)
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("create application err: %s", err)
		}
		if err := nsb.databaseClient.CreateServiceInstance(instanceID, details.RawParameters); err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}
	} else {
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("user provision parameter must be open, now is %s", nsb.allowUserProvisionParameters)
	}
	return brokerapi.ProvisionedServiceSpec{}, nil
}

func (nsb *NginxDataflowServiceBroker)Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error){
	nsb.logger.Debug("deprovision-service-instance", lager.Data{
		"instanceId": instanceID,
	})
	service, _ := nsb.GetService(details.ServiceID)
	if service.Name == "" {
		return brokerapi.DeprovisionServiceSpec{}, fmt.Errorf("service (%s) not found in catalog", details.ServiceID)
	}
	instanceDir := nsb.config.StoreDataDir + instanceID
	exist, err := nsb.databaseClient.ExistServiceInstance(instanceID)
	if err != nil {
		return brokerapi.DeprovisionServiceSpec{}, err
	}
	app, err := cfClient.GetApplicationWorkflow("nginx-flow-" + instanceID)
	if err != nil {
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	if app.Name == "" && exist == false {
		return brokerapi.DeprovisionServiceSpec{}, brokerapi.ErrInstanceDoesNotExist
	} else if exist && app.Name == ""{
		if err := nsb.databaseClient.DeleteServiceInstance(instanceID); err != nil {
			return brokerapi.DeprovisionServiceSpec{}, err
		}
	} else if exist == false && app.Name != "" {
		if err := cfClient.DeleteApplcationWorkflow("nginx-flow-" + instanceID, instanceDir); err != nil {
			return brokerapi.DeprovisionServiceSpec{}, err
		}
	} else {
		if err := nsb.databaseClient.DeleteServiceInstance(instanceID); err != nil {
			return brokerapi.DeprovisionServiceSpec{}, err
		}
		if err := cfClient.DeleteApplcationWorkflow("nginx-flow-" + instanceID, instanceDir); err != nil {
			return brokerapi.DeprovisionServiceSpec{}, err
		}
	}
	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (nsb *NginxDataflowServiceBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	nsb.logger.Debug("last-operation", lager.Data{
		"instanceId": instanceID,
	})
	state, err := cfClient.CheckApplicationStateWorkflow("nginx-flow-" + instanceID)
	if err != nil {
		return brokerapi.LastOperation{}, err
	}
	return brokerapi.LastOperation{
		State:		brokerapi.LastOperationState(state),
		Description:    "Normal application state",
	}, nil
}

func (nsb *NginxDataflowServiceBroker)Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	nsb.logger.Debug("update", lager.Data{
		"instance_id":        	instanceID,
	})
	exist, err := nsb.databaseClient.ExistServiceInstance(instanceID)
	if err != nil {
		return brokerapi.UpdateServiceSpec{}, err
	}
	if exist == false {
		return brokerapi.UpdateServiceSpec{}, fmt.Errorf("service instance (%s) already delete", instanceID)
	}
	//update
	if nsb.allowUserUpdateParameters && len(details.GetRawParameters()) >0 {
		provisionParameters := ProvisionParameters{}
		sourceDir := nsb.config.StoreDataDir + instanceID
		destinationDir := nsb.config.StoreDataDir + instanceID + "/" + instanceID + ".zip"
		if jsonErr := json.Unmarshal(details.RawParameters, &provisionParameters); jsonErr != nil {
			return brokerapi.UpdateServiceSpec{}, jsonErr
		}
		ns , err := nsb.ParseParameters(instanceID, provisionParameters)
		if err != nil {
			return brokerapi.UpdateServiceSpec{}, err
		}
		err = nsb.PreparePushDir(instanceID, ns)
		if err != nil {
			return brokerapi.UpdateServiceSpec{}, err
		}
		_, err = cfClient.UpdateApplicationWorkflow("nginx-flow-" + instanceID, nsb.config.ServiceSpace, ns.Host, ns.Domain, sourceDir, destinationDir)
		if err != nil {
			return brokerapi.UpdateServiceSpec{}, err
		}
		if err := nsb.databaseClient.UpdateServiceInstance(instanceID, details.RawParameters); err != nil {
			return brokerapi.UpdateServiceSpec{}, err
		}
	} else {
		return brokerapi.UpdateServiceSpec{}, fmt.Errorf("user provision parameter must be open, now is %s", nsb.allowUserUpdateParameters)
	}
	return brokerapi.UpdateServiceSpec{}, nil
}

func (nsb *NginxDataflowServiceBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error){
	nsb.logger.Debug("bind", lager.Data{
		"instance_id":        	instanceID,
	})
	credentials := make(map[string]interface{})
	service, _ := nsb.GetService(details.ServiceID)
	if service.Name == "" {
		return brokerapi.Binding{}, fmt.Errorf("service (%s) not found in catalog", details.ServiceID)
	}
	//random port generate
	ports := make([]int, 0)
	for i := 8001; i <= 8010; i++ {
		ports = append(ports, i)
	}
	//check service instance exist
	exist, err := nsb.databaseClient.ExistServiceInstance(instanceID)
	if err != nil {
		return brokerapi.Binding{}, err
	}
	if exist == false {
		return brokerapi.Binding{}, fmt.Errorf("service instance (%s) already delete", instanceID)
	}
	//get bind service's application
	bindAppGuid := details.AppGUID
	bindApp, err := cfClient.GetApplicationWithGuidWorkflow(bindAppGuid)
	if err != nil {
		return brokerapi.Binding{}, err
	}
	//get service instance details form db
	ns, err := nsb.databaseClient.GetServiceInstance(instanceID)
	if err != nil {
		return brokerapi.Binding{}, err
	}
	//get raw params |{\"name\":\"fakec\",\"url\":\"fakec.local.pcfdev.io\",\"weight\":4,\"port\":8001}|
	if nsb.allowUserBindParameters && len(details.GetRawParameters()) >0 {
		bindParameters := BindParameters{}
		sourceDir := nsb.config.StoreDataDir + instanceID
		destinationDir := nsb.config.StoreDataDir + instanceID + "/" + instanceID + ".zip"
		if jsonErr := json.Unmarshal(details.RawParameters, &bindParameters); jsonErr != nil {
			return brokerapi.Binding{}, jsonErr
		}
		bindNginx := nsb.ParseBindParameters(instanceID, bindingID, bindParameters)

		//when bind url param is null
		if bindNginx.Url == "" {
			routes, err := cfClient.GetApplicationRouteWorkflow(bindApp.Guid)
			if err != nil {
				return brokerapi.Binding{}, err
			}
			//pick one route from application
			if len(routes) >0 {
				host := routes[0].Host
				domain, err := cfClient.GetDomainWorkflow(routes[0].DomainGuid)
				if err != nil {
					return brokerapi.Binding{}, err
				}
				bindNginx.Url = host + "." + domain.Name
				if bindNginx.Weight == 0 {
					bindNginx.Weight = 5
				}
			}else {
				return brokerapi.Binding{}, fmt.Errorf("the bind application %s has no route, and bind parameter has not set url parameter", bindApp.Name)
			}
		}
		//check the origin url exist
		for _, originNginx := range ns.Nginxs {
			if originNginx.Url == bindNginx.Url {
				return brokerapi.Binding{}, fmt.Errorf("the bind url(%s) has already exist in origin nginxs(%s)", bindNginx.Url, ns.Nginxs)
			}
		}
		//set a port
		for _, n := range ns.Nginxs {
			for index, p := range ports {
				if n.Port == p {
					ports = append(ports[:index], ports[index+1:]...)
					break
				}
			}
		}
		bindNginx.Port = ports[1]
		//set weight
		if bindNginx.Weight == 0 {
			bindNginx.Weight = 5
		}
		//revert origin nginxs
		ns.Nginxs = append(ns.Nginxs, bindNginx)
		ns.ServiceId = instanceID
		newRawParameters, err := json.Marshal(ns)
		if err != nil {
			return brokerapi.Binding{}, err
		}
		err = nsb.PreparePushDir(instanceID, ns)
		if err != nil {
			return brokerapi.Binding{}, err
		}
		_, err = cfClient.UpdateApplicationWorkflow("nginx-flow-" + instanceID, nsb.config.ServiceSpace, ns.Host, ns.Domain, sourceDir, destinationDir)
		if err != nil {
			return brokerapi.Binding{}, err
		}
		if err := nsb.databaseClient.UpdateServiceInstance(instanceID, newRawParameters); err != nil {
			return brokerapi.Binding{}, err
		}
		credentials["host"] = ns.Host
		credentials["domain"] = ns.Domain
		credentials["nginxs"] = ns.Nginxs
	}else {
		return brokerapi.Binding{}, fmt.Errorf("user bind parameter must be open, now is %s", nsb.allowUserUpdateParameters)
	}
	return brokerapi.Binding{
		Credentials:    credentials,
	}, nil
}

func (nsb *NginxDataflowServiceBroker) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	nsb.logger.Debug("unbind", lager.Data{
		"instance_id":        	instanceID,
	})
	sourceDir := nsb.config.StoreDataDir + instanceID
	destinationDir := nsb.config.StoreDataDir + instanceID + "/" + instanceID + ".zip"
	exist, err := nsb.databaseClient.ExistServiceInstance(instanceID)
	if err != nil {
		return err
	}
	if exist == false {
		return fmt.Errorf("service instance (%s) already delete", instanceID)
	}
	//get service instance details form db
	ns, err := nsb.databaseClient.GetServiceInstance(instanceID)
	if err != nil {
		return err
	}
	//check the bindId and url exist
	for index, n := range ns.Nginxs {
		if n.Name == bindingID {
			ns.Nginxs = append(ns.Nginxs[:index], ns.Nginxs[index+1:]...)
			break
		}
	}
	//update instance
	err = nsb.PreparePushDir(instanceID, ns)
	if err != nil {
		return err
	}
	_, err = cfClient.UpdateApplicationWorkflow("nginx-flow-" + instanceID, nsb.config.ServiceSpace, ns.Host, ns.Domain, sourceDir, destinationDir)
	if err != nil {
		return err
	}
	//revert database
	newNginxParameters, err := json.Marshal(ns)
	if err != nil {
		return err
	}
	if err = nsb.databaseClient.UpdateServiceInstance(instanceID, newNginxParameters); err != nil {
		return err
	}
	return nil
}

// [{"name":"fakea","url":"fakea.dcos.os","weight":4,"port":8001},{"name":"fakeb","url":"fakeb.dcos.os","weight":6,"port":8002}]
// -> {"service_id":"64e82332-b919-4188-bb3e-14103ff0e1bd","nginxs":[{"name":"fakea","url":"fakea.dcos.os","weight":4,"port":8001},{"name":"fakeb","url":"fakeb.dcos.os","weight":6,"port":8002}]}
func (nsb *NginxDataflowServiceBroker)ParseParameters(instanceId string, parameters map[string]interface{}) (route.NginxService, error){
	ns := route.NginxService{
		ServiceId:	instanceId,
	}
	for serviceKey, serviceValue := range parameters {
		if serviceKey == "nginxs" {
			var nginxs []route.Nginx
			nginsString := serviceValue.(string)
			err := json.Unmarshal([]byte(nginsString), &nginxs)
			if err != nil {
				return route.NginxService{}, err
			}
			ns.Nginxs = nginxs
		}
		if serviceKey == "host" {
			ns.Host = serviceValue.(string)
		}
		if serviceKey == "domain" {
			ns.Domain = serviceValue.(string)
		}
	}
	return ns, nil
}

func (nsb *NginxDataflowServiceBroker)ParseBindParameters(instanceId string, bindId string,parameters map[string]interface{}) (route.Nginx) {
	nb := route.Nginx{
		Name:        bindId,
	}
	for bindKey, bindValue := range parameters {
		if bindKey == "url" {
			nb.Url = bindValue.(string)
		}
		if bindKey == "weight" {
			nb.Weight = int(bindValue.(float64))
		}
	}
	return nb
}

func (nsb *NginxDataflowServiceBroker)GetService(serviceId string) (config.Service, error) {
	for _,s := range nsb.config.Services {
		if strings.EqualFold(s.Id, serviceId) {
			return s, nil
		}
	}
	return *new(config.Service), nil
}

func (nsb *NginxDataflowServiceBroker)GetPlan(serviceId, planId string) (config.Plan, error) {
	for _,s := range nsb.config.Services {
		if strings.EqualFold(s.Id, serviceId) {
			for _,p := range s.Plans {
				if strings.EqualFold(p.Id, planId){
					return p, nil
				}
			}
		}
	}
	return *new(config.Plan), nil
}
// dir data prepare
func (nsb *NginxDataflowServiceBroker)PreparePushDir(instanceID string, ns route.NginxService) error{
	pushDir := nsb.config.StoreDataDir + instanceID
	pushDirExist, pushDirErr := utils.PathExists(pushDir)
	if pushDirErr != nil {
		return pushDirErr
	}
	if pushDirExist {
		err := os.RemoveAll(pushDir)
		if err != nil{
			return err
		}
	}
	mkdirErr := os.Mkdir(pushDir, os.FileMode(666))
	if mkdirErr != nil {
		return mkdirErr
	}
	_, err := utils.CopyFile(pushDir + "/" + "index.html", nsb.config.TemplateDir + "index.html")
	if err != nil {
		return err
	}
	err = route.ParseNginxTemplate(nsb.config.TemplateDir + "nginx.conf.templ", ns, pushDir + "/" + "nginx.conf")
	if err != nil {
		return err
	}
	return nil
}

func servicePlans(plans []config.Plan) []brokerapi.ServicePlan {
	servicePlans := make([]brokerapi.ServicePlan, len(plans)-1)
	for _,servicePlan := range plans {
		servicePlans = append(servicePlans, brokerapi.ServicePlan{
			ID:		servicePlan.Id,
			Name:		servicePlan.Name,
			Description:	servicePlan.Description,
			Free:		servicePlan.Free,
			Bindable:	servicePlan.Bindable,
			Metadata:	&brokerapi.ServicePlanMetadata{
				Bullets: 	servicePlan.Metadata.Bullets,
			},
		})
	}
	return servicePlans
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

func authHandler(config config.Config, noAuthRequired map[*mux.Route]bool) mux.MiddlewareFunc{
	validCredentials := func(r *http.Request) bool {
		if noAuthRequired[mux.CurrentRoute(r)] {
			return true
		}
		user := os.Getenv("USERNAME")
		pass := os.Getenv("PASSWORD")
		username, password, ok := r.BasicAuth()
		if ok && username == user && password == pass {
			return true
		}
		return false
	}

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validCredentials(r) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(w, r)
		})
	}
}