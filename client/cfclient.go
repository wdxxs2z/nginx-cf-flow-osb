package client

import (
	"os"
	"fmt"
	"time"
	"strings"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/wdxxs2z/nginx-flow-osb/utils"
	"code.cloudfoundry.org/lager"
	"path/filepath"
)

func targetCFClient() (*cfclient.Client, error){
	cfApi := os.Getenv("CF_API")
	cfUsername := os.Getenv("CF_USERNAME")
	cfPassword := os.Getenv("CF_PASSWORD")
	if cfApi == "" || cfUsername == "" || cfPassword == "" {
		return nil, fmt.Errorf("Cloud Foundry %s, %s, %s must not blank.", "api", "username", "password")
	}
	config := &cfclient.Config{
		ApiAddress:        cfApi,
		Username:          cfUsername,
		Password:          cfPassword,
		SkipSslValidation: true,
	}
	client, err := cfclient.NewClient(config)
	return client, err
}

func GetSpaceWorkflow(spaceGuid string, logger lager.Logger)(cfclient.Space,error){
	logger.Debug("fetch-cloudfoundry-space-workflow", lager.Data{
		"space_guid":    spaceGuid,
	})
	client, err := targetCFClient()
	if err != nil {
		return cfclient.Space{}, err
	}
	return client.GetSpaceByGuid(spaceGuid)
}

func CreateApplicationWorkflow(appName, spaceName, routeName, domain string, sourceDir string, destinationZip string, instanceNum, memory, disk int, buildpack string, logger lager.Logger) (cfclient.App, error){
	logger.Debug("create-cloudfoundry-application-workflow", lager.Data{
		"app_name":    appName,
		"route_name":  routeName,
		"domain_name": domain,
	})
	client, err := targetCFClient()
	if err != nil {
		return cfclient.App{}, err
	}
	//app
	app, err := getApplication(client, appName)
	if err != nil {
		return cfclient.App{}, err
	}
	if app.Name == "" {
		app, err = createApplication(client, appName, spaceName, instanceNum, memory, disk, buildpack)
		if err != nil {
			return cfclient.App{}, err
		}
	}
	//route
	route, err := getRoute(client, routeName, domain)
	if err != nil {
		return cfclient.App{}, err
	}
	if route.Host == "" {
		route, err = createRoute(client, routeName, domain, spaceName)
		if err != nil {
			return cfclient.App{}, err
		}
	}
	//map
	mapping, err := getMappingRoute(client, app.Guid, route.Guid)
	if err != nil {
		return cfclient.App{}, err
	}
	if mapping.Guid == "" {
		_, err = mapRouteToApplication(client, app.Guid, route.Guid)
		if err != nil {
			return cfclient.App{}, err
		}
	}
	//upload app
	err = uploadApplication(client, app.Guid, sourceDir, destinationZip)
	if err != nil {
		return cfclient.App{}, err
	}
	//start app
	_, err = updateApplication(client, app.Guid, "STARTED")
	if err != nil {
		return cfclient.App{}, err
	}
	return app, nil
}

func UpdateApplicationWorkflow(appName, spaceName, routeName, domainName string, sourceDir string, destinationZip string, logger lager.Logger) (cfclient.App, error){
	logger.Debug("update-cloudfoundry-application-workflow", lager.Data{
		"app_name":    appName,
		"route_name":  routeName,
		"domain_name": domainName,
	})
	client, err := targetCFClient()
	if err != nil {
		return cfclient.App{}, err
	}
	//get origin application
	originApp , err := getApplication(client, appName)
	if err != nil {
		return cfclient.App{}, err
	}
	//create a new application blue
	blueApp, err := createApplication(client, appName + "-blue", spaceName, originApp.Instances, originApp.Memory, originApp.DiskQuota, originApp.Buildpack)
	if err != nil {
		return cfclient.App{}, err
	}
	//create blue application route
	blueAppRoute, err := getRoute(client, routeName, domainName)
	if err != nil {
		return cfclient.App{}, err
	}
	if blueAppRoute.Host == "" {
		blueAppRoute, err = createRoute(client, routeName, domainName, spaceName)
		if err != nil {
			return cfclient.App{}, err
		}
	}
	//mapping blue application route to blue application
	blueAppRouteMapping, err := getMappingRoute(client, blueApp.Guid, blueAppRoute.Guid)
	if err != nil {
		return cfclient.App{}, err
	}
	if blueAppRouteMapping.Guid == "" {
		blueAppRouteMapping, err = mapRouteToApplication(client, blueApp.Guid, blueAppRoute.Guid)
		if err != nil {
			return cfclient.App{}, err
		}
	}
	//upload bits to blue application
	err = uploadApplication(client, blueApp.Guid, sourceDir, destinationZip)
	if err != nil {
		return cfclient.App{}, err
	}
	//start blue application
	_, err = updateApplication(client, blueApp.Guid, "STARTED")
	if err != nil {
		return cfclient.App{}, err
	}
	//wait for blue application start up, close async mode
	errChan := make(chan error)
	successChan := make(chan  bool)
	timeout := time.Duration(90 * time.Second)
	go getAppStateAsync(client, blueApp.Guid, errChan, successChan)
	select {
	case <- errChan :
		//delete blue application route mapping, and routes
		if err = cleanApplicationResource(client, blueApp); err != nil {
			return cfclient.App{}, err
		}
		return cfclient.App{}, err
	case <- successChan :
		//delete origin application route mapping and not origin app exist routes
		err = cleanApplicationResource(client, originApp)
		if err != nil {
			return cfclient.App{}, err
		}
		//rename blue application name to origin app name
		err = renameApplication(client, blueApp.Guid, appName)
		if err != nil {
			return cfclient.App{}, err
		}
	case <- time.After(timeout):
		if err = cleanApplicationResource(client, blueApp); err != nil {
			return cfclient.App{}, fmt.Errorf("update blue timeout, and clean blue app cause an error: %s", err)
		}
		return cfclient.App{}, fmt.Errorf("get blue app(%s) state timeout(%d)", blueApp.Name, timeout)
	}
	defer close(successChan)
	defer close(errChan)

	return blueApp, nil
}

func DeleteApplcationWorkflow(appName string, instanceDir string, logger lager.Logger) error{
	logger.Debug("delete-cloudfoundry-application-workflow", lager.Data{
		"app_name":    appName,
	})
	client, err := targetCFClient()
	if err != nil {
		return err
	}
	app , err := getApplication(client, appName)
	if err != nil {
		return err
	}
	routes, err := getApplicationRoutes(client, app.Guid)
	if err != nil {
		return err
	}
	for _,route := range routes {
		err = unmappingRouteWithApplication(client, app.Guid, route.Guid)
		if err != nil {
			return err
		}
		err = deleteAppRoute(client, route.Guid)
		if err != nil {
			return err
		}
	}
	err = removeInstanceDir(instanceDir)
	if err != nil {
		return err
	}
	return deleteApplication(client, app.Name)
}

func CheckApplicationExistWorkflow(appName string, logger lager.Logger) (bool, error) {
	logger.Debug("check-cloudfoundry-application-workflow", lager.Data{
		"app_name":    appName,
	})
	client, err := targetCFClient()
	if err != nil {
		return false, err
	}
	app, err := getApplication(client, appName)
	if err != nil {
		return false, err
	}
	if app.Guid == "" {
		return false, nil
	}
	return true, nil
}

func GetApplicationWorkflow(appName string, logger lager.Logger) (cfclient.App, error){
	logger.Debug("fetch-cloudfoundry-application-workflow", lager.Data{
		"app_name":    appName,
	})
	client, err := targetCFClient()
	if err != nil {
		return cfclient.App{}, err
	}
	app, err := getApplication(client, appName)
	if err != nil {
		return cfclient.App{}, err
	}
	return app, nil
}

func GetApplicationWithGuidWorkflow(appGuid string, logger lager.Logger) (cfclient.App, error){
	logger.Debug("fetch-cloudfoundry-application-guid-workflow", lager.Data{
		"app_guid":    appGuid,
	})
	client, err := targetCFClient()
	if err != nil {
		return cfclient.App{}, err
	}
	return client.GetAppByGuid(appGuid)
}

func GetApplicationRouteWorkflow(appGuid string, logger lager.Logger) ([]cfclient.Route, error) {
	logger.Debug("fetch-cloudfoundry-application--route-workflow", lager.Data{
		"app_guid":	appGuid,
	})
	client, err := targetCFClient()
	if err != nil {
		return []cfclient.Route{}, err
	}
	return getApplicationRoutes(client, appGuid)
}

func GetDomainWorkflow(domainGuid string, logger lager.Logger) (cfclient.SharedDomain, error){
	logger.Debug("fetch-cloudfoundry-domain-workflow", lager.Data{
		"domain_guid":    domainGuid,
	})
	client, err := targetCFClient()
	if err != nil {
		return cfclient.SharedDomain{}, err
	}
	sharedDomains, err := client.ListSharedDomains()
	if err != nil {
		return cfclient.SharedDomain{}, err
	}
	for _,sharedDomain := range sharedDomains {
		if sharedDomain.Guid == domainGuid {
			return sharedDomain, nil
		}
	}
	return cfclient.SharedDomain{}, fmt.Errorf("domain not found with %s", domainGuid)
}

func CheckApplicationStateWorkflow(appName string, logger lager.Logger) (string, error){
	logger.Debug("check-cloudfoundry-application-state-workflow", lager.Data{
		"app_name":    appName,
	})
	client, err := targetCFClient()
	if err != nil {
		return "failed", err
	}
	app, err := getApplication(client, appName)
	if err != nil {
		return "failed", err
	}
	stats, err := client.GetAppStats(app.Guid)
	if err != nil {
		return "failed", err
	}
	status := stats["0"]
	switch status.State {
	case "RUNNING":
		return "succeeded", nil
	case "STARTING":
		return "in progress", nil
	case "CRASHED":
		return "failed", nil
	default:
		return "failed", nil
	}
}

func cleanApplicationResource(client *cfclient.Client, oldApp cfclient.App) error{
	oldRoutes, err := getApplicationRoutes(client, oldApp.Guid)
	if err != nil {
		return err
	}
	if len(oldRoutes) != 0 {
		for _, oldRoute := range oldRoutes {
			err = unmappingRouteWithApplication(client, oldApp.Guid, oldRoute.Guid)
			if err != nil {
				return err
			}
			oldRouteMappings, err := getRouteMappingsWithRoute(client, oldRoute.Guid)
			if err != nil {
				return err
			}
			if len(oldRouteMappings) == 0 {
				err = deleteAppRoute(client, oldRoute.Guid)
				if err != nil {
					return err
				}
			}
		}
	}
	err = deleteApplication(client, oldApp.Name)
	if err != nil {
		return err
	}
	return nil
}

func getAppStateAsync(client *cfclient.Client, appGuid string, errChan chan error, successChan chan bool){
	appStates, err := client.GetAppStats(appGuid)
	if err != nil{
		errChan <- err
	}
	s := appStates["0"].State
	switch s {
	case "RUNNING":
		successChan <- true
	case "STARTING":
		getAppStateAsync(client, appGuid, errChan, successChan)
	case "CRASHED":
		errChan <- fmt.Errorf("app crashed error")
	case "DOWN":
		getAppStateAsync(client, appGuid, errChan, successChan)
	}
}

func getApplicationRoutes(client *cfclient.Client, appGuid string) ([]cfclient.Route, error){
	routes , err := client.GetAppRoutes(appGuid)
	if err != nil {
		return []cfclient.Route{}, err
	}
	return routes, nil
}

func getDomainGuid(client *cfclient.Client, domain string) (string, error) {
	sharedDomain, err := client.GetSharedDomainByName(domain)
	if err != nil {
		return "", err
	}
	return sharedDomain.Guid, nil
}

func getDomain(client *cfclient.Client, domain string) (cfclient.SharedDomain, error){
	sharedDomain, err := client.GetSharedDomainByName(domain)
	if err != nil {
		return cfclient.SharedDomain{}, err
	}
	return sharedDomain, nil
}

func getSpaceGuid(client *cfclient.Client, spaceName string) (string, error) {
	org, err := client.GetOrgByName("system")
	if err != nil {
		return "", err
	}
	space, err := client.GetSpaceByName(spaceName, org.Guid)
	if err != nil {
		return "", err
	}
	return space.Guid, nil
}

func getRoute(client *cfclient.Client, hostName string, domain string) (cfclient.Route , error){
	domainGuid, err := getDomainGuid(client, domain)
	if err != nil {
		return cfclient.Route{}, err
	}
	query := make(map[string][]string)
	hostQuery := fmt.Sprintf("host:%s", hostName)
	domainQuery := fmt.Sprintf("domain_guid:%s", domainGuid)
	query["q"] = []string{hostQuery, domainQuery}
	routers, err := client.ListRoutesByQuery(query)
	if err != nil {
		return cfclient.Route{}, err
	}
	if len(routers) == 0 {
		return cfclient.Route{}, nil
	}
	return routers[0], nil
}

func createRoute(client *cfclient.Client, host, domain, space string) (cfclient.Route, error){
	route , err := getRoute(client, host, domain)
	if err != nil {
		return cfclient.Route{}, err
	}
	if route.Guid != "" {
		return route, nil
	}
	domain_guid, err := getDomainGuid(client, domain)
	if err != nil {
		return cfclient.Route{}, err
	}
	space_guid, err := getSpaceGuid(client, space)
	if err != nil {
		return cfclient.Route{}, err
	}
	routeRequest := cfclient.RouteRequest{
		DomainGuid:       domain_guid,
		SpaceGuid:        space_guid,
		Host:             host,
	}
	route, err = client.CreateRoute(routeRequest)
	if err != nil {
		if strings.Contains(err.Error(), "CF-RoutePortNotEnabledOnApp") {
			return route, nil
		}
		return cfclient.Route{}, err
	}
	return route, nil
}

func deleteAppRoute(client *cfclient.Client, routeGuid string) (error) {
	return client.DeleteRoute(routeGuid)
}

func renameApplication (client *cfclient.Client, appGuid, newName string) error{
	aur := cfclient.AppUpdateResource{
		Name:           newName,
	}
	_, err := client.UpdateApp(appGuid, aur)
	if err != nil {
		return err
	}
	return nil
}

func getRouteMappingsWithRoute(client *cfclient.Client, routeGuid string) ([]*cfclient.RouteMapping, error) {
	query := make(map[string][]string)
	routeQuery := fmt.Sprintf("route_guid:%s", routeGuid)
	query["q"] = []string{routeQuery}
	mappings, err := client.ListRouteMappingsByQuery(query)
	if err != nil {
		return []*cfclient.RouteMapping{}, err
	}
	return mappings, nil
}

func mapRouteToApplication(client *cfclient.Client, appGuid, routeGuid string) (*cfclient.RouteMapping, error){
	mapRequest := cfclient.RouteMappingRequest{
		AppGUID:        appGuid,
		RouteGUID:      routeGuid,
		AppPort:        8080,
	}
	routeMapping, err := client.MappingAppAndRoute(mapRequest)
	return routeMapping, err
}

func unmappingRouteWithApplication(client *cfclient.Client, appGuid, routeGuid string) error{
	query := make(map[string][]string)
	appQuery := fmt.Sprintf("app_guid:%s", appGuid)
	routeQuery := fmt.Sprintf("route_guid:%s", routeGuid)
	query["q"] = []string{appQuery, routeQuery}
	routeMappings, err := client.ListRouteMappingsByQuery(query)
	if err != nil {
		return err
	}
	if len(routeMappings) == 0 {
		return nil
	}
	return client.DeleteRouteMapping(routeMappings[0].Guid)
}

func getMappingRoute(client *cfclient.Client, appGuid, routeGuid string) (*cfclient.RouteMapping, error) {
	query := make(map[string][]string)
	hostQuery := fmt.Sprintf("app_guid:%s", appGuid)
	domainQuery := fmt.Sprintf("route_guid:%s", routeGuid)
	query["q"] = []string{hostQuery, domainQuery}
	mappings, err := client.ListRouteMappingsByQuery(query)
	if err != nil {
		return &cfclient.RouteMapping{}, err
	}
	if len(mappings) == 0 {
		return &cfclient.RouteMapping{}, nil
	}
	return mappings[0], nil
}

func createApplication(client *cfclient.Client, appName, spaceName string, instanceNum, memory, disk int, buildpack string) (cfclient.App, error){
	spaceGuid, err := getSpaceGuid(client, spaceName)
	if err != nil {
		return cfclient.App{}, err
	}
	appRequest := cfclient.AppCreateRequest{
		Name:       appName,
		SpaceGuid:  spaceGuid,
	}
	app, err := client.CreateApp(appRequest)
	if err != nil {
		return cfclient.App{}, err
	}
	aur := cfclient.AppUpdateResource{
		Name:           app.Name,
		SpaceGuid:      app.SpaceGuid,
		Memory:		memory,
		DiskQuota: 	disk,
		Instances:      instanceNum,
		Buildpack:      buildpack,
	}
	_, err = client.UpdateApp(app.Guid, aur)
	if err != nil {
		return cfclient.App{}, fmt.Errorf("update app err: %s", err)
	}
	return app, nil
}

func updateApplication(client *cfclient.Client, appGuid, state string) (cfclient.UpdateResponse, error){
	aur := cfclient.AppUpdateResource{
		State:      state,
	}
	return client.UpdateApp(appGuid, aur)
}

func getApplication(client *cfclient.Client, appName string) (cfclient.App, error){
	query := make(map[string][]string)
	nameQuery := fmt.Sprintf("name:%s", appName)
	query["q"] = []string{nameQuery}
	apps , err := client.ListAppsByQuery(query)
	if err != nil {
		return cfclient.App{}, err
	}
	if len(apps) == 0 {
		return cfclient.App{}, nil
	}
	return apps[0], nil
}

func deleteApplication(client *cfclient.Client, appName string) error{
	app , err := getApplication(client, appName)
	if err != nil {
		return err
	}
	return client.DeleteApp(app.Guid)
}

func uploadApplication(client *cfclient.Client, appGuid, source, des string) error{
	var files []string
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error)error{
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = utils.ZipFiles(des, files)
	if err != nil {
		return fmt.Errorf("zip error: %s", err)
	}
	desZipFile ,err := os.Open(des)
	if err != nil {
		return err
	}
	defer desZipFile.Close()
	return client.UploadAppBits(desZipFile, appGuid)
}

func removeInstanceDir(dir string) error {
	return os.RemoveAll(dir)
}