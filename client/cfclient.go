package client

import (
	"github.com/cloudfoundry-community/go-cfclient"
	"os"
	"fmt"
	"github.com/wdxxs2z/nginx-flow-osb/utils"
	"strings"
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

func CreateApplicationWorkflow(appName, spaceName, routeName, domain string, sourceDir string, destinationZip string) (cfclient.App, error){
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
		app, err = createApplication(client, appName, spaceName)
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

func UpdateApplicationWorkflow(appName, spaceName, routeName, domain string, sourceDir string, destinationZip string) (cfclient.App, error){
	client, err := targetCFClient()
	if err != nil {
		return cfclient.App{}, err
	}
	app , err := getApplication(client, appName)
	if err != nil {
		return cfclient.App{}, err
	}
	route, err := getRoute(client, routeName, domain)
	if err != nil {
		return cfclient.App{}, err
	}

	route, err = createRoute(client, routeName, domain, spaceName)
	if err != nil {
		return cfclient.App{}, err
	}
	_, err = mapRouteToApplication(client, app.Guid, route.Guid)
	if err != nil {
		return cfclient.App{}, err
	}

	err = uploadApplication(client, app.Guid, sourceDir, destinationZip)
	if err != nil {
		return cfclient.App{}, err
	}
	return app, nil
}

func DeleteApplcationWorkflow(appName string, routeName, domain string) error{
	client, err := targetCFClient()
	if err != nil {
		return err
	}
	app , err := getApplication(client, appName)
	if err != nil {
		return err
	}

	route, err := getRoute(client, routeName, domain)
	if err != nil {
		return err
	}
	err = unmappingRouteWithApplication(client, app.Guid, route.Guid)
	if err != nil {
		return err
	}

	err = deleteRoute(client, routeName, domain)
	if err != nil {
		return err
	}

	return deleteApplication(client, app.Name)
}

func CheckApplicationStateWorkflow(appName string) (string, error){
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
	case "DOWN":
		return "failed", nil
	default:
		return "failed", nil
	}
}

func getDomainGuid(client *cfclient.Client, domain string) (string, error) {
	sharedDomain, err := client.GetSharedDomainByName(domain)
	if err != nil {
		return "", err
	}
	return sharedDomain.Guid, nil
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

func deleteRoute(client *cfclient.Client, host, domain string) error{
	route , err := getRoute(client, host, domain)
	if err != nil {
		return err
	}
	return client.DeleteRoute(route.Guid)
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

func createApplication(client *cfclient.Client, appName, spaceName string) (cfclient.App, error){
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
		Memory:		128,
		DiskQuota: 	64,
		Buildpack:      "staticfile_buildpack",
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
	sourceDir, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceDir.Close()
	var files = []*os.File{sourceDir}
	err = utils.Compress(files, des)
	if err != nil {
		return err
	}
	desZipFile ,err := os.Open(des)
	if err != nil {
		return err
	}
	defer desZipFile.Close()
	return client.UploadAppBits(desZipFile, appGuid)
}