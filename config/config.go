package config

type Config struct {
	AllowUserProvisionParameters bool    		`yaml:"allow_user_provision_parameters"`
	AllowUserUpdateParameters    bool    		`yaml:"allow_user_update_parameters"`
	AllowUserBindParameters      bool               `yaml:"allow_user_bind_parameters"`
	DatabaseConfig               DB                 `yaml:"db"`
	NginxBackendInstanceNum      int                `yaml:"per_nginx_backend_instance_num"`
	StoreDataDir		     string		`yaml:"store_data_dir"`
	TemplateDir                  string             `yaml:"template_dir"`
	ServiceSpace                 string             `yaml:"service_space"`
	Services                     []Service 		`yaml:"services"`
}

type DB struct {
	Host			string			`yaml:"host"`
	Port			int			`yaml:"port"`
	Username                string			`yaml:"username"`
	Password 		string 			`yaml:"password"`
	DbName                  string                  `yaml:"db_name"`
	DialTimeout		int64			`yaml:"dial_timeout"`
	ConnMaxLifetime         int64			`yaml:"conn_max_lifetime"`
	MaxIdleConns		int			`yaml:"max_idle_conns"`
	MaxOpenConns		int			`yaml:"max_open_conns"`
}

type Service struct {
	Id          		string 			`yaml:"id"`
	Name        		string 			`yaml:"name"`
	Description 		string 			`yaml:"description"`
	Tags        	 	[]string		`yaml:"tags"`
	Requires    	 	[]string		`yaml:"requires"`
	Bindable    	 	bool			`yaml:"bindable"`
	Metadata    	 	ServiceMetadata		`yaml:"metadata"`
	DashboardClient  	map[string]string	`yaml:"dashboard_client"`
	PlanUpdateable   	bool			`yaml:"plan_updateable"`
	Plans 			[]Plan 			`yaml:"plans"`
}

type ServiceMetadata struct {
	DisplayName         	string 			`yaml:"displayName"`
	ImageUrl            	string 			`yaml:"imageUrl"`
	LongDescription     	string 			`yaml:"longDescription"`
	ProviderDisplayName 	string 			`yaml:"providerDisplayName"`
	DocumentationUrl    	string 			`yaml:"documentationUrl"`
	SupportUrl          	string 			`yaml:"supportUrl"`
}

type Plan struct {
	Id          		string 			`yaml:"id"`
	Name        		string 			`yaml:"name"`
	Description 		string 			`yaml:"description"`
	Free        		*bool 			`yaml:"free"`
	Bindable    		*bool			`yaml:"bindable"`
	EnableSystemSpace       bool                    `yaml:"use_system_space"`
	Metadata    		PlanMetadata		`yaml:"metadata"`
}

type PlanMetadata struct {
	Costs    		[]Cost			`yaml:"costs"`
	Bullets  		[]string		`yaml:"bullets"`
}

type Cost struct {
	Amount    		map[string]float64	`yaml:"amount"`
	Unit      		string			`yaml:"unit"`
}