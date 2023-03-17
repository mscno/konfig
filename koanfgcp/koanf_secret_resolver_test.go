package koanfgcp

import (
	"context"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestKoanfSecret(t *testing.T) {

	type nested struct {
		Nested string `koanf:"simple" gcpsecret:"TEST_SECRET"`
	}

	type cfg struct {
		Simple string `koanf:"simple" gcpsecret:"TEST_SECRET"`
		Ignore string `koanf:"ignore"`
		Nested nested `koanf:"nested"`
	}

	ctx := context.Background()
	k := koanf.New(".")
	p, err := Provider(ctx,
		Config{Project: "playground-mscno"},
		&cfg{},
		func(s string) string { return s })
	require.NoError(t, err)
	err = k.Load(p, nil)
	require.NoError(t, err)
	require.NotEmpty(t, k.String("simple"))
	require.NotEmpty(t, k.String("nested.simple"))
}
