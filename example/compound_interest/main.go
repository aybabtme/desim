package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/aybabtme/desim/pkg/desim"
	"github.com/aybabtme/desim/pkg/gen"
	"github.com/urfave/cli"
)

func main() {
	rateFlag := cli.Float64Flag{Name: "rate", Value: 3.5, Usage: "yearly interest rate", Required: true}
	termsFlag := cli.Float64Flag{Name: "terms", Value: 360, Usage: "number of terms, in month", Required: true}
	amountFlag := cli.Float64Flag{Name: "amount", Value: 1e6, Usage: "amount that is loaned", Required: true}

	app := cli.App{
		Name: "compound-interest",
		Flags: []cli.Flag{
			rateFlag,
			termsFlag,
			amountFlag,
		},
		Action: func(cctx *cli.Context) error {
			return run(
				cctx.Float64(rateFlag.Name),
				cctx.Int(termsFlag.Name),
				cctx.Float64(amountFlag.Name),
			)
		},
	}
	app.RunAndExitOnError()
}

func run(rate float64, term int, amount float64) error {
	loanTerms := LoanTerms{
		Rate: rate / 100.0, Term: term, Amount: amount,
	}
	loan := desim.MakeFIFOResource("loan", 1)
	var (
		r     = rand.New(rand.NewSource(42))
		start = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
		end   = start.AddDate(0, term+2, 0)
	)
	evs := desim.New(
		desim.NewLocalScheduler,
		r,
		gen.StaticTime(start),
		gen.StaticTime(end),
	).Run([]*desim.Actor{
		desim.MakeActor("investor", makeInvestor(loanTerms, loan)),
	},
		[]desim.Resource{loan},
		desim.LogJSON(os.Stderr),
	)
	for _, ev := range evs {
		fmt.Printf("%v: %s - %s\n", ev.Time, ev.Actor, ev.Kind)
	}
	return nil
}

type LoanTerms struct {
	Rate   float64
	Term   int
	Amount float64
}

func makeInvestor(loanTerms LoanTerms, loan desim.Resource) desim.Action {
	terms := 0
	amount := loanTerms.Amount
	ratePerTerm := loanTerms.Rate / 12.0

	return func(env desim.Env) bool {
		release, acquired := env.Acquire(loan, gen.StaticDuration(24*time.Hour))
		if !acquired {
			env.Log().Event("couldn't acquire loan resource")
			return false
		}
		defer release()

		if terms == loanTerms.Term {
			env.Log().Event("all terms of the loan have been completed")
			return false
		}

		interest := (amount * ratePerTerm)
		nextAmount := amount + interest
		terms++

		env.Log().
			KV("interest", fmt.Sprintf("%f", interest)).
			KV("old_amount", fmt.Sprintf("%f", amount)).
			KV("new_amount", fmt.Sprintf("%f", nextAmount)).Event("investment grew in value")

		amount = nextAmount

		nextEventIn := env.Now().AddDate(0, 1, 0).Sub(env.Now())

		if env.Sleep(gen.StaticDuration(nextEventIn)) {
			env.Log().Event("couldn't sleep")
			return false
		}

		return true
	}
}
