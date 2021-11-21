package utilmath

import "math"

func FixedPeriodicPayment(loanAmount, annualRateOfInterest float64, loanTenureInYears, paymentPerYear int) float64 {
	// Fixed Periodic Payment = P *[(r/n) * (1 + r/n)n*t] / [(1 + r/n)n*t – 1]
	// P = Outstanding Loan Amount
	// r = Rate of interest (Annual)
	// t = Tenure of Loan in Years
	// n = Number of Periodic Payments Per Year
	p := loanAmount
	r := annualRateOfInterest
	t := float64(loanTenureInYears)
	n := float64(paymentPerYear)
	return p * ((r / n) * math.Pow(1.0+r/n, n*t)) / (math.Pow(1.0+r/n, n*t) - 1.0)
}

func OutstandingLoanBalance(loanAmount, annualRateOfInterest float64, loanTenureInYears, paymentPerYear, afterYears int) float64 {
	// Outstanding Loan Balance = P * [(1 + r/n)n*t – (1 + r/n)n*m] / [(1 + r/n)n*t – 1]
	// P = Outstanding Loan Amount
	// r = Rate of interest (Annual)
	// t = Tenure of Loan in Years
	// n = Number of Periodic Payments Per Year
	p := loanAmount
	r := annualRateOfInterest
	t := float64(loanTenureInYears)
	n := float64(paymentPerYear)
	m := float64(afterYears)
	return p * (math.Pow(1+r/n, n*t) - math.Pow(1+r/n, n*m)) / (math.Pow(1+r/n, n*t) - 1)
}
