package db

import (
	"code.cloudfoundry.org/lager"
	"github.com/wdxxs2z/nginx-flow-osb/config"
	_"github.com/go-sql-driver/mysql"
	"database/sql"
	"fmt"
	"time"
	"encoding/json"
	"github.com/wdxxs2z/nginx-flow-osb/route"
	"os"
)

type DBClient struct {
	client		*sql.DB
	logger          lager.Logger
}

func NewDBClient(config config.Config, logger lager.Logger) (*DBClient, error) {
	dbName := os.Getenv("DATABASE_NAME")
	dbUsername := os.Getenv("DATABASE_USERNAME")
	dbPassword := os.Getenv("DATABASE_PASSWORD")
	dbHost := os.Getenv("DATABASE_HOST")
	dbPort := os.Getenv("DATABASE_PORT")
	logger.Debug("init-database", lager.Data{
		"host": 	dbHost,
		"port":		dbPort,
		"username":    dbUsername,
	})
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&timeout=%ds", dbUsername, dbPassword, dbHost, dbPort, dbName, config.DatabaseConfig.DialTimeout)
	dbClient, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to log mysql: %s", err)
	}
	err = dbClient.Ping()
	if err != nil {
		return nil, fmt.Errorf("Failed to ping mysql: %s", err)
	}
	dbClient.SetMaxOpenConns(config.DatabaseConfig.MaxOpenConns)
	dbClient.SetMaxIdleConns(config.DatabaseConfig.MaxIdleConns)
	dbClient.SetConnMaxLifetime(time.Duration(config.DatabaseConfig.ConnMaxLifetime) * time.Hour)
	return &DBClient{
		client:		dbClient,
		logger:         logger,
	}, nil
}

func (c *DBClient) MigrateServiceInstanceTable() error {
	baseCreateTable := "CREATE TABLE IF NOT EXISTS service_instance (" +
		"id int NOT NULL AUTO_INCREMENT, PRIMARY KEY (id)" +
		", service_instance_id varchar(42) NOT NULL" +
		", service_instance_details BLOB NOT NULL" +
		", space_id varchar(42) NOT NULL" +
                ");"
	_, err := c.client.Exec(baseCreateTable)
	return err
}

func (c *DBClient) ExistServiceInstance(serviceInstanceId string) (bool, error){
	c.logger.Debug("check-db-instance-exist", lager.Data{
		"instance_id":		serviceInstanceId,
	})
	exist, err := c.rowExists("SELECT 1 FROM service_instance WHERE service_instance_id = ?", serviceInstanceId)
	if err != nil {
		return false, err
	}
	return exist, nil
}

func (c *DBClient) CreateServiceInstance(serviceInstanceId string, serviceDetails []byte, spaceGuid string) (error) {
	c.logger.Debug("create-db-instance", lager.Data{
		"instance_id":		serviceInstanceId,
	})
	_, err := c.client.Exec("INSERT INTO service_instance(service_instance_id,service_instance_details,space_id) VALUES(?,?,?)", serviceInstanceId, serviceDetails, spaceGuid)
	if err != nil {
		return err
	}
	return nil
}

func (c *DBClient) DeleteServiceInstance(serviceInstanceId string) (error) {
	c.logger.Debug("delete-db-instance", lager.Data{
		"instance_id":		serviceInstanceId,
	})
	_, err := c.client.Exec("DELETE FROM service_instance WHERE service_instance_id = ?", serviceInstanceId)
	if err != nil {
		return err
	}
	return nil
}

func (c *DBClient) GetSpaceWithServiceId(serviceInstanceId string) (string, error) {
	c.logger.Debug("get-db-space-with-service", lager.Data{
		"instance_id":		serviceInstanceId,
	})
	var spaceId string
	if err := c.client.QueryRow("SELECT space_id FROM service_instance WHERE service_instance_id = ?", serviceInstanceId).Scan(&spaceId); err != nil {
		return "", err
	}
	return spaceId, nil
}

func (c *DBClient) UpdateServiceInstance(serviceInstanceId string, serviceDetails []byte) (error){
	c.logger.Debug("update-db-instance", lager.Data{
		"instance_id":		serviceInstanceId,
	})
	stmt , err := c.client.Prepare("UPDATE service_instance SET service_instance_details = ? WHERE service_instance_id = ?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(serviceDetails, serviceInstanceId)
	if err != nil {
		return err
	}
	return nil
}

func (c *DBClient) GetServiceInstance(serviceInstanceId string) (route.NginxService, error){
	c.logger.Debug("get-db-instance", lager.Data{
		"instance_id":		serviceInstanceId,
	})
	var serviceDetailsBlob []byte
	var serviceDetails route.NginxService
	err := c.client.QueryRow("SELECT service_instance_details FROM service_instance WHERE service_instance_id = ?", serviceInstanceId).Scan(&serviceDetailsBlob)
	if err != nil {
		return route.NginxService{}, err
	}
	jsonErr := json.Unmarshal(serviceDetailsBlob, &serviceDetails)
	if jsonErr != nil {
		return route.NginxService{}, jsonErr
	}
	return serviceDetails, nil
}

func (c *DBClient)rowExists(query string, args ...interface{}) (bool , error) {
	var exists bool
	query = fmt.Sprintf("SELECT exists (%s)", query)
	err := c.client.QueryRow(query, args...).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("error checking if row exists '%s' %v", args, err)
	}
	return exists, nil
}