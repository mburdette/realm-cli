package local

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/10gen/realm-cli/internal/cloud/realm"
)

// AppStructureV2 represents the v2 Realm app structure
type AppStructureV2 struct {
	ConfigVersion         realm.AppConfigVersion            `json:"config_version"`
	ID                    string                            `json:"app_id,omitempty"`
	Name                  string                            `json:"name,omitempty"`
	Location              realm.Location                    `json:"location,omitempty"`
	DeploymentModel       realm.DeploymentModel             `json:"deployment_model,omitempty"`
	Environment           string                            `json:"environment,omitempty"`
	Environments          map[string]map[string]interface{} `json:"environments,omitempty"`
	AllowedRequestOrigins []string                          `json:"allowed_request_origins,omitempty"`
	Values                []map[string]interface{}          `json:"values,omitempty"`
	Auth                  *AuthStructure                    `json:"auth,omitempty"`
	Functions             *FunctionsStructure               `json:"functions,omitempty"`
	Triggers              []map[string]interface{}          `json:"triggers,omitempty"`
	DataSources           []DataSourceStructure             `json:"data_sources,omitempty"`
	HTTPEndpoints         []HTTPEndpointStructure           `json:"http_endpoints,omitempty"`
	Services              []ServiceStructure                `json:"services,omitempty"`
	GraphQL               *GraphQLStructure                 `json:"graphql,omitempty"`
	Hosting               map[string]interface{}            `json:"hosting,omitempty"`
	Sync                  *SyncStructure                    `json:"sync,omitempty"`
	Secrets               *SecretsStructure                 `json:"secrets,omitempty"`
}

// AuthStructure represents the v2 Realm app auth structure
type AuthStructure struct {
	CustomUserData map[string]interface{} `json:"custom_user_data,omitempty"`
	Providers      map[string]interface{} `json:"providers,omitempty"`
}

// DataSourceStructure represents the v2 Realm app data source structure
type DataSourceStructure struct {
	Config map[string]interface{}   `json:"config,omitempty"`
	Rules  []map[string]interface{} `json:"rules,omitempty"`
}

// FunctionsStructure represents the v2 Realm app functions structure
type FunctionsStructure struct {
	Configs []map[string]interface{} `json:"config,omitempty"`
	Sources map[string]string        `json:"sources,omitempty"`
}

// HTTPEndpointStructure represents the v2 Realm app http endpoint structure
type HTTPEndpointStructure struct {
	Config           map[string]interface{}   `json:"config,omitempty"`
	IncomingWebhooks []map[string]interface{} `json:"incoming_webhooks,omitempty"`
}

// SyncStructure represents the v2 Realm app sync structure
type SyncStructure struct {
	Config map[string]interface{} `json:"config,omitempty"`
}

// AppDataV2 is the v2 local Realm app data
type AppDataV2 struct {
	AppStructureV2
}

// ConfigVersion returns the local Realm app config version
func (a AppDataV2) ConfigVersion() realm.AppConfigVersion {
	return a.AppStructureV2.ConfigVersion
}

// ID returns the local Realm app id
func (a AppDataV2) ID() string {
	return a.AppStructureV2.ID
}

// Name returns the local Realm app name
func (a AppDataV2) Name() string {
	return a.AppStructureV2.Name
}

// Location returns the local Realm app location
func (a AppDataV2) Location() realm.Location {
	return a.AppStructureV2.Location
}

// DeploymentModel returns the local Realm app deployment model
func (a AppDataV2) DeploymentModel() realm.DeploymentModel {
	return a.AppStructureV2.DeploymentModel
}

// LoadData will load the local Realm app data
func (a *AppDataV2) LoadData(rootDir string) error {
	secrets, err := parseSecrets(rootDir)
	if err != nil {
		return err
	}
	a.Secrets = secrets

	environments, err := parseEnvironments(rootDir)
	if err != nil {
		return err
	}
	a.Environments = environments

	values, err := parseJSONFiles(filepath.Join(rootDir, NameValues))
	if err != nil {
		return err
	}
	a.Values = values

	auth, err := parseAuth(rootDir)
	if err != nil {
		return err
	}
	a.Auth = auth

	sync, err := parseSync(rootDir)
	if err != nil {
		return err
	}
	a.Sync = sync

	functions, err := parseFunctionsV2(rootDir)
	if err != nil {
		return err
	}
	a.Functions = functions

	triggers, err := parseJSONFiles(filepath.Join(rootDir, NameTriggers))
	if err != nil {
		return err
	}
	a.Triggers = triggers

	graphql, ok, err := parseGraphQL(rootDir)
	if err != nil {
		return err
	} else if ok {
		a.GraphQL = &graphql
	}

	services, err := parseServices(rootDir)
	if err != nil {
		return err
	}
	a.Services = services

	dataSources, err := parseDataSources(rootDir)
	if err != nil {
		return err
	}
	a.DataSources = dataSources

	httpEndpoints, err := parseHTTPEndpoints(rootDir)
	if err != nil {
		return err
	}
	a.HTTPEndpoints = httpEndpoints

	return nil
}

func parseAuth(rootDir string) (*AuthStructure, error) {
	dir := filepath.Join(rootDir, NameAuth)

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	customUserData, err := parseJSON(filepath.Join(dir, FileCustomUserData.String()))
	if err != nil {
		return nil, err
	}

	providers, err := parseJSON(filepath.Join(dir, FileProviders.String()))
	if err != nil {
		return nil, err
	}

	return &AuthStructure{customUserData, providers}, nil
}

func parseFunctionsV2(rootDir string) (*FunctionsStructure, error) {
	dir := filepath.Join(rootDir, NameFunctions)

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	configs, err := parseJSONArray(filepath.Join(dir, FileConfig.String()))
	if err != nil {
		return nil, err
	}

	sources := map[string]string{}
	if err := walk(dir, func(file os.FileInfo, path string) error {
		if filepath.Ext(path) != extJS {
			return nil // looking for javascript files
		}

		pathRelative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		sources[pathRelative] = string(data)
		return nil
	}); err != nil {
		return nil, err
	}

	return &FunctionsStructure{configs, sources}, nil
}

func parseDataSources(rootDir string) ([]DataSourceStructure, error) {
	var out []DataSourceStructure

	dw := directoryWalker{
		path:     filepath.Join(rootDir, NameDataSources),
		onlyDirs: true,
	}
	if err := dw.walk(func(file os.FileInfo, path string) error {
		config, err := parseJSON(filepath.Join(path, FileConfig.String()))
		if err != nil {
			return err
		}

		var rules []map[string]interface{}

		dbs := directoryWalker{path: path, onlyDirs: true}
		if err := dbs.walk(func(db os.FileInfo, dbPath string) error {

			colls := directoryWalker{path: dbPath, onlyDirs: true}
			if err := colls.walk(func(coll os.FileInfo, collPath string) error {

				rulePath := filepath.Join(collPath, FileRules.String())
				if _, err := os.Stat(rulePath); err != nil {
					if os.IsNotExist(err) {
						return nil // skip directories that do not contain `rules.json`
					}
					return err
				}

				rule, err := parseJSON(rulePath)
				if err != nil {
					return err
				}

				schema, err := parseJSON(filepath.Join(collPath, FileSchema.String()))
				if err != nil {
					return err
				}
				rule[NameSchema] = schema

				rules = append(rules, rule)
				return nil
			}); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		out = append(out, DataSourceStructure{config, rules})
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func parseHTTPEndpoints(rootDir string) ([]HTTPEndpointStructure, error) {
	var out []HTTPEndpointStructure

	dw := directoryWalker{
		path:     filepath.Join(rootDir, NameHTTPEndpoints),
		onlyDirs: true,
	}
	if err := dw.walk(func(file os.FileInfo, path string) error {
		config, err := parseJSON(filepath.Join(path, FileConfig.String()))
		if err != nil {
			return err
		}

		webhooks, err := parseFunctions(filepath.Join(path))
		if err != nil {
			return err
		}

		out = append(out, HTTPEndpointStructure{config, webhooks})
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func parseSync(rootDir string) (*SyncStructure, error) {
	dir := filepath.Join(rootDir, NameSync)

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	config, err := parseJSON(filepath.Join(dir, FileConfig.String()))
	if err != nil {
		return nil, err
	}
	return &SyncStructure{config}, nil
}

// ConfigData marshals the config data out to JSON
func (a AppDataV2) ConfigData() ([]byte, error) {
	temp := &struct {
		ConfigVersion         realm.AppConfigVersion `json:"config_version"`
		ID                    string                 `json:"app_id,omitempty"`
		Name                  string                 `json:"name,omitempty"`
		Location              realm.Location         `json:"location,omitempty"`
		DeploymentModel       realm.DeploymentModel  `json:"deployment_model,omitempty"`
		Environment           string                 `json:"environment,omitempty"`
		AllowedRequestOrigins []string               `json:"allowed_request_origins,omitempty"`
	}{
		ConfigVersion:         a.ConfigVersion(),
		ID:                    a.ID(),
		Name:                  a.Name(),
		Location:              a.Location(),
		DeploymentModel:       a.DeploymentModel(),
		Environment:           a.Environment,
		AllowedRequestOrigins: a.AllowedRequestOrigins,
	}
	return MarshalJSON(temp)
}

// WriteData will write the local Realm app data to disk
func (a AppDataV2) WriteData(rootDir string) error {
	if err := writeSecrets(rootDir, a.Secrets); err != nil {
		return err
	}
	if err := writeEnvironments(rootDir, a.Environments); err != nil {
		return err
	}
	if err := writeValues(rootDir, a.Values); err != nil {
		return err
	}
	// TODO(REALMC-8395): Revisit the app structure v2 and decide which directories are always present
	graphQL := a.GraphQL
	if a.GraphQL == nil {
		graphQL = &GraphQLStructure{}
	}
	if err := writeGraphQL(rootDir, *graphQL); err != nil {
		return err
	}
	if err := writeServices(rootDir, a.Services); err != nil {
		return err
	}
	if err := writeFunctionsV2(rootDir, a.Functions); err != nil {
		return err
	}
	if err := writeAuth(rootDir, a.Auth); err != nil {
		return err
	}
	if err := writeSync(rootDir, a.Sync); err != nil {
		return err
	}
	if err := writeDataSources(rootDir, a.DataSources); err != nil {
		return err
	}
	if err := writeHTTPEndpoints(rootDir, a.HTTPEndpoints); err != nil {
		return err
	}
	if err := writeHTTPEndpoints(rootDir, a.HTTPEndpoints); err != nil {
		return err
	}
	if err := writeTriggers(rootDir, a.Triggers); err != nil {
		return err
	}
	return nil
}

func writeFunctionsV2(rootDir string, functions *FunctionsStructure) error {
	var sources map[string]string
	configs := []map[string]interface{}{}
	if functions != nil {
		configs = functions.Configs
		sources = functions.Sources
	}
	dir := filepath.Join(rootDir, NameFunctions)
	data, err := MarshalJSON(configs)
	if err != nil {
		return err
	}
	if err = WriteFile(
		filepath.Join(dir, FileConfig.String()),
		0666,
		bytes.NewReader(data),
	); err != nil {
		return err
	}
	for path, src := range sources {
		if err = WriteFile(
			filepath.Join(dir, path),
			0666,
			bytes.NewReader([]byte(src)),
		); err != nil {
			return err
		}
	}
	return nil
}

func writeAuth(rootDir string, auth *AuthStructure) error {
	if auth == nil {
		return nil
	}
	dir := filepath.Join(rootDir, NameAuth)
	if auth.Providers != nil {
		data, err := MarshalJSON(auth.Providers)
		if err != nil {
			return err
		}
		if err = WriteFile(
			filepath.Join(dir, FileProviders.String()),
			0666,
			bytes.NewReader(data),
		); err != nil {
			return err
		}
	}
	if auth.CustomUserData != nil {
		data, err := MarshalJSON(auth.CustomUserData)
		if err != nil {
			return err
		}
		if err = WriteFile(
			filepath.Join(dir, FileCustomUserData.String()),
			0666,
			bytes.NewReader(data),
		); err != nil {
			return err
		}
	}
	return nil
}

func writeSync(rootDir string, sync *SyncStructure) error {
	if sync == nil || sync.Config == nil {
		return nil
	}
	data, err := MarshalJSON(sync.Config)
	if err != nil {
		return err
	}
	if err = WriteFile(
		filepath.Join(rootDir, NameSync, FileConfig.String()),
		0666,
		bytes.NewReader(data),
	); err != nil {
		return err
	}
	return nil
}

func writeDataSources(rootDir string, dataSources []DataSourceStructure) error {
	dir := filepath.Join(rootDir, NameDataSources)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	for _, ds := range dataSources {
		name, ok := ds.Config["name"].(string)
		if !ok {
			return errors.New("error writing datasources")
		}
		config, err := MarshalJSON(ds.Config)
		if err != nil {
			return err
		}
		if err = WriteFile(
			filepath.Join(dir, name, FileConfig.String()),
			0666,
			bytes.NewReader(config),
		); err != nil {
			return err
		}
		for _, rule := range ds.Rules {
			schema := rule[NameSchema]
			dataSchema, err := MarshalJSON(schema)
			if err != nil {
				return err
			}
			ruleTemp := map[string]interface{}{}
			for k, v := range rule {
				ruleTemp[k] = v
			}
			delete(ruleTemp, NameSchema)
			dataRule, err := MarshalJSON(ruleTemp)
			if err != nil {
				return err
			}
			if err = WriteFile(
				filepath.Join(dir, name, fmt.Sprintf("%s", rule["database"]), fmt.Sprintf("%s", rule["collection"]), FileRules.String()),
				0666,
				bytes.NewReader(dataRule),
			); err != nil {
				return err
			}
			if err = WriteFile(
				filepath.Join(dir, name, fmt.Sprintf("%s", rule["database"]), fmt.Sprintf("%s", rule["collection"]), FileSchema.String()),
				0666,
				bytes.NewReader(dataSchema),
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeHTTPEndpoints(rootDir string, httpEndpoints []HTTPEndpointStructure) error {
	dir := filepath.Join(rootDir, NameHTTPEndpoints)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	for _, httpEndpoint := range httpEndpoints {
		nameHTTPEndpoint, ok := httpEndpoint.Config["name"].(string)
		if !ok {
			return errors.New("error writing http endpoints")
		}
		data, err := MarshalJSON(httpEndpoint.Config)
		if err != nil {
			return err
		}
		if err = WriteFile(
			filepath.Join(dir, nameHTTPEndpoint, FileConfig.String()),
			0666,
			bytes.NewReader(data),
		); err != nil {
			return err
		}
		for _, webhook := range httpEndpoint.IncomingWebhooks {
			src, ok := webhook[NameSource].(string)
			if !ok {
				return errors.New("error writing http endpoints")
			}
			name, ok := webhook["name"].(string)
			if !ok {
				return errors.New("error writing http endpoints")
			}
			dirHTTPEndpoint := filepath.Join(dir, nameHTTPEndpoint, name)
			webhookTemp := map[string]interface{}{}
			for k, v := range webhook {
				webhookTemp[k] = v
			}
			delete(webhookTemp, NameSource)
			config, err := MarshalJSON(webhookTemp)
			if err != nil {
				return err
			}
			if err = WriteFile(
				filepath.Join(dirHTTPEndpoint, FileConfig.String()),
				0666,
				bytes.NewReader(config),
			); err != nil {
				return err
			}
			if err = WriteFile(
				filepath.Join(dirHTTPEndpoint, FileSource.String()),
				0666,
				bytes.NewReader([]byte(src)),
			); err != nil {
				return err
			}
		}
	}
	return nil
}
