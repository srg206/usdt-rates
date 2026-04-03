package calc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"usdt-rates/internal/calc"
)

func TestTopN(t *testing.T) {
	tests := []struct {
		name      string
		side      []float64
		n         int
		expected  float64
		expectErr bool
	}{
		{
			name:      "first element (n=1)",
			side:      []float64{10.5, 11.2, 12.0},
			n:         1,
			expected:  10.5,
			expectErr: false,
		},
		{
			name:      "middle element",
			side:      []float64{10.5, 11.2, 12.0},
			n:         2,
			expected:  11.2,
			expectErr: false,
		},
		{
			name:      "last element",
			side:      []float64{10.5, 11.2, 12.0},
			n:         3,
			expected:  12.0,
			expectErr: false,
		},
		{
			name:      "out of bounds (n<1)",
			side:      []float64{10.5, 11.2, 12.0},
			n:         0,
			expected:  0,
			expectErr: true,
		},
		{
			name:      "out of bounds (n>len)",
			side:      []float64{10.5, 11.2, 12.0},
			n:         4,
			expected:  0,
			expectErr: true,
		},
		{
			name:      "empty slice",
			side:      []float64{},
			n:         1,
			expected:  0,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := calc.TopN(tt.side, tt.n)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.expected, val, 0.0001)
			}
		})
	}
}

func TestAvgNM(t *testing.T) {
	tests := []struct {
		name      string
		side      []float64
		n         int
		m         int
		expected  float64
		expectErr bool
	}{
		{
			name:      "average of all elements (1 to len)",
			side:      []float64{10.0, 12.0, 14.0},
			n:         1,
			m:         3,
			expected:  12.0, // (10+12+14)/3
			expectErr: false,
		},
		{
			name:      "average of one element (n=m)",
			side:      []float64{10.0, 12.0, 14.0},
			n:         2,
			m:         2,
			expected:  12.0,
			expectErr: false,
		},
		{
			name:      "average of subset",
			side:      []float64{10.0, 20.0, 30.0, 40.0},
			n:         2,
			m:         3,
			expected:  25.0, // (20+30)/2
			expectErr: false,
		},
		{
			name:      "out of bounds (n < 1)",
			side:      []float64{10.0, 20.0, 30.0},
			n:         0,
			m:         2,
			expected:  0,
			expectErr: true,
		},
		{
			name:      "invalid range (m < n)",
			side:      []float64{10.0, 20.0, 30.0},
			n:         3,
			m:         2,
			expected:  0,
			expectErr: true,
		},
		{
			name:      "out of bounds (m > len)",
			side:      []float64{10.0, 20.0, 30.0},
			n:         2,
			m:         4,
			expected:  0,
			expectErr: true,
		},
		{
			name:      "empty slice",
			side:      []float64{},
			n:         1,
			m:         1,
			expected:  0,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := calc.AvgNM(tt.side, tt.n, tt.m)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.expected, val, 0.0001)
			}
		})
	}
}
