// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"context"
	"os"
	"strings"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// serverSchema loads the structural schema of spec.server from the generated DoltDB CRD.
func serverSchema(t *testing.T) *structuralschema.Structural {
	t.Helper()

	data, err := os.ReadFile("../../config/crd/bases/k8s.dolthub.com_doltdbs.yaml")
	if err != nil {
		t.Fatalf("error reading DoltDB CRD: %v", err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("error unmarshaling DoltDB CRD: %v", err)
	}

	var props apiextensions.JSONSchemaProps
	err = apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema, &props, nil,
	)
	if err != nil {
		t.Fatalf("error converting DoltDB CRD schema: %v", err)
	}
	root, err := structuralschema.NewStructural(&props)
	if err != nil {
		t.Fatalf("error building structural schema: %v", err)
	}

	server := root.Properties["spec"].Properties["server"]
	return &server
}

func TestServerRemotesAPIPortValidation(t *testing.T) {
	schema := serverSchema(t)
	validator := cel.NewValidator(schema, false, celconfig.PerCallLimit)

	tests := []struct {
		name    string
		server  string
		wantErr string
	}{
		{
			name:   "no remotes API",
			server: `{}`,
		},
		{
			name:   "remotes API with default port",
			server: `{"remotesAPI": {}}`,
		},
		{
			name:   "remotes API alongside every other port at its default",
			server: `{"remotesAPI": {}, "listener": {}, "cluster": {"remotesAPI": {}}, "mcpServer": {}, "metrics": {"enabled": true}}`,
		},
		{
			name:    "collides with default listener port",
			server:  `{"remotesAPI": {"port": 3306}}`,
			wantErr: "remotesAPI.port must differ from listener.port",
		},
		{
			name:    "collides with custom listener port",
			server:  `{"remotesAPI": {"port": 4000}, "listener": {"port": 4000}}`,
			wantErr: "remotesAPI.port must differ from listener.port",
		},
		{
			name:   "listener moved off the remotes API port",
			server: `{"remotesAPI": {"port": 3306}, "listener": {"port": 4000}}`,
		},
		{
			name:    "collides with default cluster remotes API port",
			server:  `{"remotesAPI": {"port": 50051}}`,
			wantErr: "remotesAPI.port must differ from cluster.remotesAPI.port",
		},
		{
			name:    "collides with custom cluster remotes API port",
			server:  `{"remotesAPI": {"port": 60000}, "cluster": {"remotesAPI": {"port": 60000}}}`,
			wantErr: "remotesAPI.port must differ from cluster.remotesAPI.port",
		},
		{
			name:   "cluster remotes API moved off the remotes API port",
			server: `{"remotesAPI": {"port": 50051}, "cluster": {"remotesAPI": {"port": 60000}}}`,
		},
		{
			name:    "collides with default MCP server port",
			server:  `{"remotesAPI": {"port": 7007}, "mcpServer": {}}`,
			wantErr: "remotesAPI.port must differ from mcpServer.port",
		},
		{
			name:   "MCP server port without MCP server",
			server: `{"remotesAPI": {"port": 7007}}`,
		},
		{
			name:    "collides with enabled metrics port",
			server:  `{"remotesAPI": {"port": 11228}, "metrics": {"enabled": true}}`,
			wantErr: "remotesAPI.port must differ from metrics.port",
		},
		{
			name:   "metrics port with metrics disabled",
			server: `{"remotesAPI": {"port": 11228}, "metrics": {"enabled": false}}`,
		},
		{
			name:    "collides with enabled profiler port",
			server:  `{"remotesAPI": {"port": 6060}, "profiler": {"enablePProf": true}}`,
			wantErr: "remotesAPI.port must differ from the profiler port 6060",
		},
		{
			name:   "profiler port with profiler disabled",
			server: `{"remotesAPI": {"port": 6060}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Decode like the API server does, with integers as int64 rather than float64.
			var obj map[string]any
			if err := json.Unmarshal([]byte(tt.server), &obj); err != nil {
				t.Fatalf("error unmarshaling server: %v", err)
			}
			// The API server applies defaults before running validation rules.
			defaulting.Default(obj, schema)

			errs, _ := validator.Validate(
				context.Background(), field.NewPath("spec", "server"), schema, obj, nil, celconfig.RuntimeCELCostBudget,
			)

			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Errorf("expected no errors, got %v", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
			}
			if !strings.Contains(errs[0].Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, errs[0].Error())
			}
		})
	}
}
