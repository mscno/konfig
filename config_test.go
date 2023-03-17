package konfig

import (
	"context"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

var testProjectSet = Set{"playground-mscno", "playground-mscno", "playground-mscno"}

func TestConfig(t *testing.T) {
	ctx := context.Background()
	k := NewKonfig(
		testProjectSet,
		"us-central1")

	type Config struct {
		Project string `koanf:"project" validate:"required"`
		Region  string `koanf:"region" validate:"required"`
		Env     string `koanf:"env" validate:"required"`
		Runtime string `koanf:"runtime" validate:"required"`
	}

	cfg := &Config{}
	err := k.InitializeConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err.Error())
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	assert.Equal(t, DEV, k.Env())
}

func TestEnvOverrideConfig(t *testing.T) {
	ctx := context.Background()
	k := NewKonfig(
		testProjectSet,
		"us-central1")

	type Config struct {
		Project string `koanf:"project" validate:"required"`
		Region  string `koanf:"region" validate:"required"`
		Env     string `koanf:"env" validate:"required"`
		Runtime string `koanf:"runtime" validate:"required"`
	}
	k.SetRuntime(LOCAL)
	k.SetEnv(PROD)

	cfg := &Config{}
	err := k.InitializeConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err.Error())
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	assert.Equal(t, PROD, k.Env())
	assert.Equal(t, LOCAL, k.Runtime())
}

func TestGcpResolvingConfig(t *testing.T) {
	ctx := context.Background()
	k := NewKonfig(
		testProjectSet,
		"us-central1")

	type Config struct {
		Project          string `koanf:"project" validate:"required"`
		Region           string `koanf:"region" validate:"required"`
		Env              string `koanf:"env" validate:"required"`
		Runtime          string `koanf:"runtime" validate:"required"`
		TestNestedStruct struct {
			TestSecret string `koanf:"test_secret" validate:"required" gcpsecret:"TEST_SECRET"`
		} `koanf:"test_nested_struct"`
	}

	cfg := &Config{}
	err := k.InitializeConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err.Error())
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	assert.Equal(t, DEV, k.Env())
}

func TestOverridesOkResolvingConfig(t *testing.T) {
	ctx := context.Background()
	k := NewKonfig(
		testProjectSet,
		"us-central1",
		WithRuntimeOverrides(RuntimeOverrides{
			TEST: func(k *koanf.Koanf) error {
				return k.Set("region", "europe-west1")
			},
			CI: func(k *koanf.Koanf) error {
				return k.Set("region", "europe-west1")
			},
		}))

	type Config struct {
		Project          string `koanf:"project" validate:"required"`
		Region           string `koanf:"region" validate:"required"`
		Env              string `koanf:"env" validate:"required"`
		Runtime          string `koanf:"runtime" validate:"required"`
		TestNestedStruct struct {
			TestSecret string `koanf:"test_secret" validate:"required" gcpsecret:"TEST_SECRET"`
		} `koanf:"test_nested_struct"`
	}

	cfg := &Config{}
	err := k.InitializeConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err.Error())
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	assert.Equal(t, DEV, k.Env())
	assert.Equal(t, "europe-west1", cfg.Region)
}

func TestGcpOkSkipFromFile(t *testing.T) {
	ctx := context.Background()
	k := NewKonfig(testProjectSet, "us-central1", WithDefaults(Defaults{"test_secret_non_existing": "value"}))

	type Config struct {
		TestSecret            string `koanf:"test_secret" validate:"required" gcpsecret:"TEST_SECRET"`
		TestSecretNonExisting string `koanf:"test_secret_non_existing" validate:"required" gcpsecret:"TEST_SECRET_NON_EXISTING"`
	}

	cfg := &Config{}
	err := k.InitializeConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err.Error())
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	assert.Equal(t, DEV, k.Env())
	assert.NotEmpty(t, cfg.TestSecret)
	assert.NotEmpty(t, cfg.TestSecretNonExisting)
}

func TestDefaultsOkResolvingConfig(t *testing.T) {
	ctx := context.Background()
	k := NewKonfig(
		testProjectSet,
		"us-central1",
		WithDefaults(Defaults{
			"host": Set{"localhost", "global", "global"},
			"port": "8080",
		}))

	type Config struct {
		Host string `koanf:"host" validate:"required"`
		Port string `koanf:"port" validate:"required"`
	}

	k.SetEnv(STAGING)

	cfg := &Config{}
	err := k.InitializeConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err.Error())
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	assert.Equal(t, STAGING, k.Env())
	assert.Equal(t, "global", cfg.Host)
	assert.Equal(t, "8080", cfg.Port)
}
