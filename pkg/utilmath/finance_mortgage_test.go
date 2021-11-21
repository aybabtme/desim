package utilmath

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixedPeriodicPayment(t *testing.T) {
	type args struct {
		loanAmount           float64
		annualRateOfInterest float64
		loanTenureInYears    int
		paymentPerYear       int
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "simple case",
			args: args{
				loanAmount:           2_000_000.0,
				annualRateOfInterest: 0.08,
				loanTenureInYears:    5,
				paymentPerYear:       12,
			},
			want: 40_552.79,
		},
		{
			name: "real case",
			args: args{
				loanAmount:           1_000_000.0,
				annualRateOfInterest: 0.025,
				loanTenureInYears:    30,
				paymentPerYear:       12,
			},
			want: 3951.21,
		},
		{
			name: "annuity",
			args: args{
				loanAmount:           6000.0,
				annualRateOfInterest: 0.05,
				loanTenureInYears:    10,
				paymentPerYear:       1,
			},
			want: 777.03,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FixedPeriodicPayment(tt.args.loanAmount, tt.args.annualRateOfInterest, tt.args.loanTenureInYears, tt.args.paymentPerYear); got != tt.want {
				got = math.Round(got*100.0) / 100.0
				require.Equal(t, tt.want, got)
			}
		})
	}
}
