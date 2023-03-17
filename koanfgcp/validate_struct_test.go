package koanfgcp

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_validateAndResolve(t *testing.T) {
	type cfg struct {
		Simple string `koanf:"simple" gcpsecret:"gcp_simple"`
		Ignore string `koanf:"ignore"`
	}

	type cfgNested struct {
		Simple string `koanf:"outer" gcpsecret:"outer_gcp"`
		Nested cfg    `koanf:"nested"`
		Ignore string `koanf:"ignore"`
	}

	type cfgNestedWithPointer struct {
		Simple string `koanf:"outer" gcpsecret:"outer_gcp"`
		Nested *cfg   `koanf:"nested"`
		Ignore string `koanf:"ignore"`
	}

	type args struct {
		cfg interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr assert.ErrorAssertionFunc
	}{
		{"simple", args{cfg: &cfg{}}, map[string]string{"simple": "gcp_simple"}, assert.NoError},
		{"simple with nested", args{cfg: &cfgNested{}}, map[string]string{"nested.simple": "gcp_simple", "outer": "outer_gcp"}, assert.NoError},
		{"simple with nested with pointer", args{cfg: &cfgNestedWithPointer{Nested: &cfg{}}}, map[string]string{"nested.simple": "gcp_simple", "outer": "outer_gcp"}, assert.NoError},
		{"simple with nested with pointer that is nil", args{cfg: &cfgNestedWithPointer{Nested: nil}}, map[string]string{"outer": "outer_gcp"}, assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAndResolve(tt.args.cfg)
			if !tt.wantErr(t, err, fmt.Sprintf("validateAndResolve(%v)", tt.args.cfg)) {
				return
			}
			assert.Equalf(t, tt.want, got, "validateAndResolve(%v)", tt.args.cfg)
		})
	}
}
