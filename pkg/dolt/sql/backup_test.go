// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterSystemDatabases(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "filters all system databases",
			input:    []string{"information_schema", "mysql", "dolt_cluster", "mydb"},
			expected: []string{"mydb"},
		},
		{
			name:     "no system databases",
			input:    []string{"app", "analytics"},
			expected: []string{"app", "analytics"},
		},
		{
			name:     "only system databases",
			input:    []string{"information_schema", "mysql", "dolt_cluster"},
			expected: nil,
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "mixed databases",
			input:    []string{"mysql", "production", "information_schema", "staging", "dolt_cluster"},
			expected: []string{"production", "staging"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterSystemDatabases(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
