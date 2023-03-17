package koanfgcp

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"context"
	"fmt"
	"github.com/knadh/koanf/maps"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"strings"
	"sync"
	"time"
)

const gcpSecretTag = "gcpsecret"
const koanfTag = "koanf"
const defaultConcurrency = 50

// Config holds the AWS SecretsManager Configuration.
type Config struct {
	Project string
	// The AWS SecretsManager Delim that might be used
	// delim string
	Delim string

	// Time interval at which the watcher will refresh the configuration.
	// Defaults to 3600 seconds.
	WatchInterval time.Duration

	//Concurrency is the number of goroutines to use when fetching secrets
	Concurrency int

	SkipKeys []string
}

// SMConfig implements an AWS SecretsManager provider.
type SMConfig struct {
	client *secretmanager.Client
	config Config
	target interface{}
	input  *secretmanagerpb.AccessSecretVersionRequest
	cb     func(s string) string
}

// Provider returns an AWS SecretsManager provider.
func Provider(ctx context.Context, cfg Config, target interface{}, cb func(s string) string) (*SMConfig, error) {
	// load the default config
	client, err := secretmanager.NewClient(ctx, option.WithScopes("https://www.googleapis.com/auth/cloud-platform"))
	if err != nil {
		return nil, errors.Wrap(err, "could not create secretmanager client")
	}

	// check inputs and set
	if cfg.Delim == "" {
		cfg.Delim = "."
	}

	if cfg.Concurrency == 0 {
		cfg.Concurrency = defaultConcurrency
	}

	return &SMConfig{client: client, config: cfg, cb: cb, target: target}, nil
}

// ProviderWithClient returns an AWS SecretsManager provider
// using an existing AWS SecretsManager client.
func ProviderWithClient(cfg Config, cb func(s string) string, client *secretmanager.Client) *SMConfig {
	return &SMConfig{client: client, config: cfg, cb: cb}
}

// Read is not supported by the SecretsManager provider.
func (sm *SMConfig) Read() (map[string]interface{}, error) {

	// check if secretId is provided
	if sm.target == nil {
		return nil, errors.New("no secret id  provided")
	}

	secretsToFetch, err := validateAndResolve(sm.target)
	if err != nil {
		return nil, err
	}

	for _, k := range sm.config.SkipKeys {
		delete(secretsToFetch, k)
	}

	res, err := sm.getSecrets(context.TODO(), secretsToFetch)
	if err != nil {
		return nil, err
	}

	mp := make(map[string]interface{})
	for key, value := range res {
		if sm.cb != nil {
			key = sm.cb(key)
		}
		mp[key] = value
	}

	return maps.Unflatten(mp, sm.config.Delim), nil
}

type koanfParams struct {
	koanfName string
	gcpName   string
}

func (sm *SMConfig) getSecrets(ctx context.Context, secretsToFetch map[string]string) (map[string]string, error) {
	var wg sync.WaitGroup

	type errorStruct struct {
		err  error
		name string
	}

	c := make(chan errorStruct, len(secretsToFetch))
	var mux sync.Mutex
	res := map[string]string{}

	p, _ := ants.NewPoolWithFunc(sm.config.Concurrency, func(i interface{}) {
		defer wg.Done()
		p := i.(koanfParams)

		secret, err := getLatestSecretVersion(sm.client, sm.config.Project, p.gcpName)
		if err != nil {
			c <- errorStruct{err, p.gcpName}
			return
		}

		mux.Lock()
		defer mux.Unlock()
		res[p.koanfName] = string(secret.Payload.Data)
	})

	for k, v := range secretsToFetch {
		wg.Add(1)
		_ = p.Invoke(koanfParams{koanfName: k, gcpName: v})
	}

	defer p.Release()

	wg.Wait()

	close(c)

	var errs []string
	for e := range c {
		errs = append(errs, e.name+": "+e.err.Error())
	}

	if len(errs) != 0 {
		return nil, errors.New("Error when fetching secrets: " + strings.Join(errs, ", "))
	}

	return res, nil
}

func getLatestSecretVersion(client *secretmanager.Client, project, name string) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", project, name),
	}

	secret, err := client.AccessSecretVersion(context.TODO(), req)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// ReadBytes returns the raw bytes for parsing.
func (sm *SMConfig) ReadBytes() ([]byte, error) {
	// shoud implement for SecretBinary. maybe in future
	return nil, errors.New("secretsmanager provider does not support this method")
}

// Watch polls AWS AppConfig for configuration updates.
func (sm *SMConfig) Watch(cb func(event interface{}, err error)) error {
	//if sm.config.WatchInterval == 0 {
	//	// Set default watch interval to 3600 seconds. to reduce cost
	//	sm.config.WatchInterval = 3600 * time.Second
	//}
	//
	//go func() {
	//loop:
	//	for {
	//		conf, err := sm.client.GetSecretValue(context.TODO(), &sm.input)
	//		if err != nil {
	//			cb(nil, err)
	//			break loop
	//		}
	//
	//		// Check if the the configuration has been updated.
	//		if *conf.VersionId == *sm.config.VersionId {
	//			// Configuration is not updated and we have the latest version.
	//			// Sleep for WatchInterval and retry watcher.
	//			time.Sleep(sm.config.WatchInterval)
	//			continue
	//		}
	//
	//		// Trigger event.
	//		cb(nil, nil)
	//	}
	//}()

	return nil
}
