package konfig

import (
	"cloud.google.com/go/compute/metadata"
	"context"
	"flag"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/mscno/konfig/koanfgcp"
	"github.com/pkg/errors"
	"os"
	"strings"
)

// Config Strategy
// 1. Detect if running on GCP
// 2. Set env, project and region from GCP metadata if running on GCP
// 3. Set env, project and region from defaults if not running on GCP
// 4. Set base config from env
// 5. Resolve secrets
// 6. If local, set config from file

type Defaults map[string]interface{}
type RuntimeOverrides map[RUNTIME]func(k *koanf.Koanf) error

type Konfig struct {
	*koanf.Koanf
	projects         Set
	defaultRegion    string
	defaults         Defaults
	runtimeOverrides RuntimeOverrides
	configPath       string
}

func (k *Konfig) K() *koanf.Koanf {
	return k.Koanf
}

type Option func(k *Konfig)

func WithDefaults(defaults Defaults) Option {
	return func(k *Konfig) {
		k.defaults = defaults
	}
}

func WithRuntimeOverrides(runtimeOverrides RuntimeOverrides) Option {
	return func(k *Konfig) {
		k.runtimeOverrides = runtimeOverrides
	}
}

func WithConfigPath(configPath string) Option {
	return func(k *Konfig) {
		k.configPath = configPath
	}
}

func NewKonfig(projects Set, defaultRegion string, opts ...Option) *Konfig {
	k := koanf.New(".")
	ko := &Konfig{
		Koanf:         k,
		projects:      projects,
		defaultRegion: defaultRegion,
	}
	for _, opt := range opts {
		opt(ko)
	}
	return ko
}

//const (
//	// Default values
//	defaultRegion       = "us-central1"
//	defaultEnv          = PROD
//	defaultProject      = projectProd
//	projectProd         = "mac-address-api-prod"
//	projectStaging      = "mac-address-api-dev"
//	projectDev          = "mac-address-api-dev"
//	storageBucketPrefix = "mac-address-api-storage"
//)

type PROJECT string

func (v PROJECT) String() string {
	return string(v)
}

type REGION string

func (v REGION) String() string {
	return string(v)
}

type ENV string

func (v ENV) String() string {
	return string(v)
}

const (
	DEV     ENV = "dev"
	STAGING ENV = "staging"
	PROD    ENV = "prod"
)

type RUNTIME string

func (v RUNTIME) String() string {
	return string(v)
}

const (
	LOCAL RUNTIME = "local"
	CI    RUNTIME = "ci"
	TEST  RUNTIME = "test"
	CLOUD RUNTIME = "cloud"
)

const (
	// Config keys
	envKey     = "env"
	runtimeKey = "runtime"
	projectKey = "project"
	regionKey  = "region"
)

func (k *Konfig) SetEnv(env ENV) {
	k.Set(envKey, env)
}

func (k *Konfig) SetRuntime(env RUNTIME) {
	k.Set(runtimeKey, env)
}

func getRuntime() RUNTIME {
	if IsCi() {
		return CI
	}
	if IsTest() {
		return TEST
	}
	if metadata.OnGCE() {
		return CLOUD
	}
	return LOCAL
}

func (k *Konfig) Runtime() RUNTIME {
	if runtime, ok := k.Get(runtimeKey).(RUNTIME); ok {
		return runtime
	}
	runtime := getRuntime()
	k.Set(runtimeKey, runtime)
	return runtime
}

func projectFromEnv(k *Konfig, env ENV) string {
	if env == PROD {
		return k.projects[2].(string)
	}
	if env == STAGING {
		return k.projects[1].(string)
	}
	return k.projects[0].(string)
}

func (k *Konfig) SetDebug() {
	k.Set("debug", true)
}

func (k *Konfig) Debug() bool {
	return k.Bool("debug")
}

func initializeEnvAndRuntime(k *Konfig) error {
	// First we need to get the runtime and find out if we are running on GCP or test or ci or local
	runtime := k.Runtime()

	// Then we need to set the env and project and region based on the runtime
	switch runtime {
	case CI, TEST:
		if env, ok := k.Get(envKey).(ENV); ok {
			k.Set(projectKey, projectFromEnv(k, env))
		} else {
			k.Set(envKey, DEV)
		}
		k.Set(regionKey, k.defaultRegion)
		k.Set(projectKey, k.projects[1].(string))
	case CLOUD:
		gcp, project, region, err := k.getMetadataFromGcp()
		if err != nil {
			return errors.Wrap(err, "failed to get metadata from gcp")
		}
		env := parseEnvFromProjectName(project)
		if gcp {
			k.Set(envKey, env)
			k.Set(regionKey, region)
			k.Set(projectKey, project)
		}
	case LOCAL:
		k.Set(regionKey, k.defaultRegion)
		if env, ok := k.Get(envKey).(ENV); ok {
			k.Set(projectKey, projectFromEnv(k, env))
		} else {
			k.Set(envKey, DEV)
			k.Set(projectKey, k.projects[1].(string))
		}
	}
	return nil
}

func (k *Konfig) Env() ENV {
	if env, ok := k.Get(envKey).(ENV); ok {
		return env
	}
	panic("env not set")
}

func (k *Konfig) Project() PROJECT {
	if project, ok := k.Get(projectKey).(string); ok {
		return PROJECT(project)
	}
	panic("project not set")
}

func parseDefaults(env ENV, m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		if s, ok := v.(Set); ok {
			switch env {
			case DEV:
				m[k] = s[0]
			case STAGING:
				m[k] = s[1]
			case PROD:
				m[k] = s[2]
			}
		}
	}
	return m
}

type Set [3]interface{}

func loadBase(k *Konfig, env ENV) {
	d := parseDefaults(env, k.defaults)
	k.Load(confmap.Provider(d, "."), nil)
}

func (k *Konfig) InitializeConfig(ctx context.Context, pointer interface{}) error {
	// First we need to initialize the env and runtime
	err := initializeEnvAndRuntime(k)
	if err != nil {
		return err
	}

	// Load defaults
	loadBase(k, k.Env())

	for runtime, fn := range k.runtimeOverrides {
		if runtime != k.Runtime() {
			continue
		}
		err := fn(k.Koanf)
		if err != nil {
			return err
		}
	}

	// If we are running on GCP, we need to set the port from the environment variable
	if !RunningOnCloud() {
		f := file.Provider(".env")
		if k.configPath == "" {
			k.configPath = "config.yaml"
		}
		f = file.Provider(k.configPath)
		if err := k.Load(f, yaml.Parser()); err != nil {
			if k.Debug() {
				fmt.Println("could not load config file" + err.Error())
			}
		}
	}

	cfg := pointer
	gcpKoanfProvider, err := koanfgcp.Provider(ctx,
		koanfgcp.Config{Project: string(k.Project()), SkipKeys: k.Keys()},
		cfg,
		func(s string) string { return s })
	if err != nil {
		return errors.Wrap(err, "could not initialize gcp config provider")
	}
	err = k.Load(gcpKoanfProvider, nil)
	if err != nil {
		return errors.Wrap(err, "could not load gcp config")
	}

	// resolveSecrets
	err = k.Unmarshal("", &cfg)
	if err != nil {
		return errors.Wrap(err, "could not unmarshal config")
	}

	err = validator.New().Struct(cfg)
	if err != nil {
		return errors.Wrap(err, "could not validate config")
	}

	return nil
}

func (k *Konfig) OnGcp() bool {
	return k.Get("gcp").(bool)
}

func (k *Konfig) getMetadataFromGcp() (OnGCP bool, ProjectId string, Region string, err error) {

	projectId, err := metadata.ProjectID()
	if err != nil {
		return true, "", "", errors.Wrap(err, "failed to get project id")
	}
	if !k.isInternalProject(projectId) {
		return false, "", "", nil
	}

	zone, err := metadata.Zone()
	if err != nil {
		return true, "", "", errors.Wrap(err, "failed to get compute zone")
	}
	region := zone[:strings.LastIndex(zone, "-")]

	if region == "" {
		return true, "", "", errors.Wrap(err, "failed to get region")
	}

	return true, projectId, region, nil
}

func parseEnvFromProjectName(project string) ENV {
	if strings.Contains(project, string(PROD)) {
		return PROD
	}
	if strings.Contains(project, string(STAGING)) {
		return STAGING
	}
	if strings.Contains(project, string(DEV)) {
		return DEV
	}
	return DEV
}

func minimumStaging(env ENV) ENV {
	if env == DEV {
		return STAGING
	}
	return env
}

func (k *Konfig) isInternalProject(project string) bool {
	for _, p := range k.projects {
		if p.(string) == project {
			return true
		}
	}
	return false
}

func IsCi() bool {
	return os.Getenv("CI") == "true"
}

func IsTest() bool {
	if flag.Lookup("test.v") == nil {
		return false
	} else {
		return true
	}
}
