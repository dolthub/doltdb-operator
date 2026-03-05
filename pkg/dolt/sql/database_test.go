// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid simple", "mydb", "mydb", false},
		{"valid with underscore", "my_db_123", "my_db_123", false},
		{"strips backticks", "my`db", "mydb", false},
		{"empty string", "", "", true},
		{"only backticks", "```", "", true},
		{"has spaces", "my db", "", true},
		{"has semicolon", "mydb;", "", true},
		{"has dash", "my-db", "", true},
		{"has quotes", "my'db", "", true},
		{"sql injection attempt", "a`;DROP TABLE--", "aDROPTABLE", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeIdentifier(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
