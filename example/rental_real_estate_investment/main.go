package main

import (
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/aybabtme/desim/pkg/desim"
	"github.com/aybabtme/desim/pkg/gen"
	"github.com/aybabtme/desim/pkg/utilmath"
	"github.com/urfave/cli"
)

func main() {
	durationFlag := cli.IntFlag{Name: "duration_month", Value: 120, Usage: "how long to simulate for", Required: true}
	rateFlag := cli.Float64Flag{Name: "loan_rate", Value: 3.5, Usage: "yearly interest rate", Required: true}
	termsFlag := cli.Float64Flag{Name: "loan_terms", Value: 360, Usage: "number of terms, in month", Required: true}
	ltvFlag := cli.IntFlag{Name: "loan_ltv", Value: 65, Usage: "loan_to_value", Required: true}
	stockMarketGrowthRateFlag := cli.Float64Flag{Name: "stock_growth_rate", Value: 3.5, Usage: "annualized growth rate of the stock market", Required: true}
	brokerageFlag := cli.Float64Flag{Name: "brokerage", Value: 10e3, Usage: "amount in the brokerage at the begining", Required: true}
	monthlyRentalPaymentFlag := cli.Float64Flag{Name: "monthly_rental", Value: 1000, Usage: "amount that is paid for by the rental unit", Required: true}
	yearlyRentIncreaseFlag := cli.Float64Flag{Name: "rent_increase", Value: 1, Usage: "yearly rent increase in percent", Required: true}
	propertyValueFlag := cli.Float64Flag{Name: "property_value", Value: 100e3, Usage: "value of the property at the beginning", Required: true}
	yearlyPropertyValueIncreaseFlag := cli.Float64Flag{Name: "property_value_increase", Value: 4, Usage: "year-over-year property value increase ", Required: true}
	app := cli.App{
		Name: "real-estate-investor",
		Flags: []cli.Flag{
			durationFlag,
			rateFlag,
			termsFlag,
			ltvFlag,
			stockMarketGrowthRateFlag,
			brokerageFlag,
			monthlyRentalPaymentFlag,
			yearlyRentIncreaseFlag,
			propertyValueFlag,
			yearlyPropertyValueIncreaseFlag,
		},
		Action: func(cctx *cli.Context) error {
			return run(
				cctx.Int(durationFlag.Name),
				cctx.Float64(rateFlag.Name),
				cctx.Int(termsFlag.Name),
				cctx.Float64(ltvFlag.Name),
				cctx.Float64(stockMarketGrowthRateFlag.Name),
				cctx.Float64(brokerageFlag.Name),
				cctx.Float64(monthlyRentalPaymentFlag.Name),
				cctx.Float64(yearlyRentIncreaseFlag.Name),
				cctx.Float64(propertyValueFlag.Name),
				cctx.Float64(yearlyPropertyValueIncreaseFlag.Name),
			)
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(simulMonths int, mortgageRate float64, mortgageTerm int, mortgageLTV, stockMarketGrowthRate, brokerageInitialAmount, monthlyRentalPayment, yearlyRentIncrease, propertyValue, yoyPropertyValueIncrease float64) error {
	ltv := mortgageLTV / 100.0
	mortgageAmount := ltv * propertyValue
	downpaymentAmount := propertyValue - mortgageAmount
	assets := &Assets{
		Lock:                desim.MakeFIFOResource("brokerage", 2),
		InvestedInBrokerage: brokerageInitialAmount - downpaymentAmount,
		PropertyValue:       propertyValue,
	}
	mortgageTerms := &MortgageTerm{
		Lock:        desim.MakeFIFOResource("loan", 2),
		Rate:        mortgageRate / 100.0,
		Terms:       mortgageTerm,
		Amount:      mortgageAmount,
		LoanBalance: mortgageAmount,
		TermsLeft:   mortgageTerm,
	}
	leaseTerms := &LeaseTerm{
		Lock:                  desim.MakeFIFOResource("rent", 2),
		LengthMonths:          12,
		InitialMonthlyPayment: monthlyRentalPayment,
		YearlyIncreasePercent: yearlyRentIncrease / 100.0,
	}
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
		end   = start.AddDate(0, simulMonths, 0)
	)
	evs := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	).Run([]*desim.Actor{
		desim.MakeActor("bank", makeBank(mortgageTerms)),
		desim.MakeActor("investor", makeInvestor(assets, mortgageTerms)),
		desim.MakeActor("stock_market", makeStockMarket(assets, stockMarketGrowthRate/100.0)),
		desim.MakeActor("real_estate_market", makeRealEstateMarket(assets, yoyPropertyValueIncrease/100.0)),
		desim.MakeActor("rental", makeRentalUnit(leaseTerms, assets)),
	},
		[]desim.Resource{mortgageTerms.Lock, assets.Lock, leaseTerms.Lock},
		desim.LogJSON(os.Stdout),
	)
	_ = evs
	// for _, ev := range evs {
	// 	fmt.Printf("%v: %s - %s\n", ev.Time, ev.Actor, ev.Kind)
	// }

	return nil
}

type Assets struct {
	Lock                desim.Resource
	InvestedInBrokerage float64
	PropertyValue       float64
}

type MortgageTerm struct {
	Lock desim.Resource

	Rate   float64
	Terms  int
	Amount float64

	LastPayment float64

	LoanBalance   float64
	InterestPaid  float64
	PrincipalPaid float64
	TermsLeft     int
}

func (mt MortgageTerm) TermPayment() float64 {
	return utilmath.FixedPeriodicPayment(mt.Amount, mt.Rate, mt.Terms/12, 12)
}

func (mt MortgageTerm) TotalLoanBalanceToPay() float64 {
	return utilmath.OutstandingLoanBalance(mt.Amount, mt.Rate, mt.Terms/12, 12, 0)
}

func makeBank(mortgage *MortgageTerm) desim.Action {
	monthlyRate := mortgage.Rate / 12.0

	return func(env desim.Env) bool {

		nextEventIn := env.Now().AddDate(0, 1, 0).Sub(env.Now())

		if env.Sleep(gen.StaticDuration(nextEventIn)) {
			env.Log().Event("couldn't sleep")
			return false
		}

		release, acquired := env.Acquire(mortgage.Lock, gen.StaticDuration(24*time.Hour))
		if !acquired {
			env.Log().Event("couldn't acquire mortgage resource")
			return false
		}
		defer release()

		if mortgage.LastPayment == 0 {
			env.Log().Event("no payment received! sending to collection!")
			return false
		}

		payment := mortgage.LastPayment

		payingToInterest := mortgage.LoanBalance * monthlyRate
		payingToPrincipal := payment - payingToInterest

		mortgage.LoanBalance -= payingToPrincipal
		mortgage.InterestPaid += payingToInterest
		mortgage.PrincipalPaid += payingToPrincipal

		mortgage.TermsLeft--
		if mortgage.TermsLeft == 0 {
			env.Log().KVf("loan_balance", mortgage.LoanBalance).Event("all terms of the mortgage have been completed")
			env.Abort()
			return false
		}
		env.Log().KVf("collected", mortgage.InterestPaid+mortgage.PrincipalPaid).
			KVf("interest_paid", payingToInterest).
			KVf("principal_paid", payingToPrincipal).
			KVf("loan_balance", mortgage.LoanBalance).
			KVi("term", mortgage.Terms-mortgage.TermsLeft).
			Event("received payment")

		return true
	}
}

func makeInvestor(assets *Assets, mortgageTerms *MortgageTerm) desim.Action {
	monthlyPayment := mortgageTerms.TermPayment()
	return func(env desim.Env) bool {
		releaseBrkrg, acquired := env.Acquire(assets.Lock, gen.StaticDuration(24*time.Hour))
		if !acquired {
			env.Log().Event("couldn't acquire brokerage resource")
			return false
		}
		defer releaseBrkrg()

		releaseMrtg, acquired := env.Acquire(mortgageTerms.Lock, gen.StaticDuration(24*time.Hour))
		if !acquired {
			env.Log().Event("couldn't acquire mortgage resource")
			return false
		}
		defer releaseMrtg()

		netWorth := assets.InvestedInBrokerage + assets.PropertyValue - mortgageTerms.LoanBalance

		if mortgageTerms.TermsLeft == 0 {
			env.Log().
				KVf("brokerage_value", assets.InvestedInBrokerage).
				KVf("property_value", assets.PropertyValue).
				KVf("net-worth", netWorth).
				Event("collecting money")
			return true
		}
		moneyLeft := assets.InvestedInBrokerage - monthlyPayment

		env.Log().
			KVf("brokerage_value", assets.InvestedInBrokerage).
			KVf("property_value", assets.PropertyValue).
			KVf("loan_balance", mortgageTerms.LoanBalance).
			KVf("net-worth", netWorth).
			KVf("left_after", moneyLeft).
			Event("making payment from brokerage")

		assets.InvestedInBrokerage = moneyLeft
		mortgageTerms.LastPayment = monthlyPayment

		if moneyLeft <= 0.0 {
			env.Log().Event("no money left to pay mortgage!")
			env.Abort()
			return false
		}

		nextEventIn := env.Now().AddDate(0, 1, 0).Sub(env.Now())

		if env.Sleep(gen.StaticDuration(nextEventIn)) {
			env.Log().Event("couldn't sleep")
			return false
		}

		return true
	}
}

func makeStockMarket(brokerage *Assets, annualizedGrowthRate float64) desim.Action {
	growthRatePerFreq := annualizedGrowthRate / 12.0
	return func(env desim.Env) bool {
		releaseBrkrg, acquired := env.Acquire(brokerage.Lock, gen.StaticDuration(24*time.Hour))
		if !acquired {
			env.Log().Event("couldn't acquire brokerage resource")
			return false
		}

		increase := brokerage.InvestedInBrokerage * growthRatePerFreq
		brokerage.InvestedInBrokerage += increase

		if brokerage.InvestedInBrokerage <= 0 {
			releaseBrkrg()
			env.Log().Event("rent out of money!")
			env.Abort()
			return false
		}
		releaseBrkrg()

		env.Log().
			KVf("invested", brokerage.InvestedInBrokerage).
			KVf("growth", increase).
			Event("growing investment")

		nextEventIn := env.Now().AddDate(0, 1, 0).Sub(env.Now())

		if env.Sleep(gen.StaticDuration(nextEventIn)) {
			env.Log().Event("couldn't sleep")
			return false
		}

		return true
	}
}

func makeRealEstateMarket(assets *Assets, annualizedGrowthRate float64) desim.Action {
	growthRatePerFreq := annualizedGrowthRate / 12.0
	return func(env desim.Env) bool {
		releaseBrkrg, acquired := env.Acquire(assets.Lock, gen.StaticDuration(24*time.Hour))
		if !acquired {
			env.Log().Event("couldn't acquire brokerage resource")
			return false
		}

		increase := assets.PropertyValue * growthRatePerFreq
		assets.PropertyValue += increase

		if assets.PropertyValue <= 0 {
			releaseBrkrg()
			env.Log().Event("property is worth nothing!")
			env.Abort()
			return false
		}
		releaseBrkrg()

		env.Log().
			KVf("property_value", assets.PropertyValue).
			KVf("growth", increase).
			Event("growing property value")

		nextEventIn := env.Now().AddDate(0, 1, 0).Sub(env.Now())

		if env.Sleep(gen.StaticDuration(nextEventIn)) {
			env.Log().Event("couldn't sleep")
			return false
		}

		return true
	}
}

type LeaseTerm struct {
	Lock                  desim.Resource
	LengthMonths          int
	InitialMonthlyPayment float64
	YearlyIncreasePercent float64

	TotalPaidRent float64
}

func makeRentalUnit(lease *LeaseTerm, brokerage *Assets) desim.Action {
	monthyPayment := lease.InitialMonthlyPayment
	leaseLength := lease.LengthMonths
	var renew time.Time

	return func(env desim.Env) bool {
		if renew.IsZero() {
			renew = env.Now().AddDate(0, leaseLength, 0)
		}
		releaseRent, acquired := env.Acquire(lease.Lock, gen.StaticDuration(24*time.Hour))
		if !acquired {
			env.Log().Event("couldn't acquire lease resource")
			return false
		}

		releaseBrkg, acquired := env.Acquire(lease.Lock, gen.StaticDuration(24*time.Hour))
		if !acquired {
			env.Log().Event("couldn't acquire lease resource")
			return false
		}

		if env.Now().After(renew) {
			renew = env.Now().AddDate(0, leaseLength, 0)
			// increase the rent
			increase := monthyPayment * lease.YearlyIncreasePercent
			newMonthyPayment := monthyPayment + increase
			env.Log().
				KVf("new_rent", newMonthyPayment).
				KVf("old_rent", monthyPayment).
				KVf("increase", increase).
				Event("rent increase")
			monthyPayment = newMonthyPayment
		}

		brokerage.InvestedInBrokerage += monthyPayment
		lease.TotalPaidRent += monthyPayment
		env.Log().KVf("total_paid", lease.TotalPaidRent).Event("paying rent")

		releaseRent()
		releaseBrkg()

		nextEventIn := env.Now().AddDate(0, 1, 0).Sub(env.Now())

		if env.Sleep(gen.StaticDuration(nextEventIn)) {
			env.Log().Event("couldn't sleep")
			return false
		}

		return true
	}
}
