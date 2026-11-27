package agent

import (
	"errors"
	"time"

	"github.com/daptin/daptin/server/llm"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
)

type executionLimit struct{ code, message string }

func (e *executionLimit) Error() string { return e.message }

func (state *runState) finishActive(now time.Time) {
	if !state.ActiveSince.IsZero() {
		if elapsed := now.Sub(state.ActiveSince).Milliseconds(); elapsed > 0 {
			state.ActiveMS += elapsed
		}
		state.ActiveSince = time.Time{}
	}
}

func (state *runState) remaining(now time.Time) (time.Duration, string, error) {
	remaining := state.Started.Add(time.Duration(state.Config.Limits.Seconds) * time.Second).Sub(now)
	code := "lifetime_exceeded"
	if remaining <= 0 {
		return 0, code, &executionLimit{code, "execution time limit reached"}
	}
	if seconds := state.Config.Limits.ActiveSeconds; seconds > 0 {
		used := time.Duration(state.ActiveMS) * time.Millisecond
		if !state.ActiveSince.IsZero() && now.After(state.ActiveSince) {
			used += now.Sub(state.ActiveSince)
		}
		active := time.Duration(seconds)*time.Second - used
		if active <= 0 {
			return 0, "active_time_exceeded", &executionLimit{"active_time_exceeded", "active execution time limit reached"}
		}
		if active < remaining {
			remaining, code = active, "active_time_exceeded"
		}
	}
	return remaining, code, nil
}

func (state *runState) budget(ref, token string) resource.MeteringBudget {
	return resource.MeteringBudget{Run: ref, Attempt: state.Active, Claim: token,
		Tokens: state.Config.Limits.Tokens, CostMicros: state.Config.Limits.CostMicros,
		Started: state.Started, Deadline: state.Started.Add(time.Duration(state.Config.Limits.Seconds) * time.Second)}
}

func errorCode(err error) string {
	if errors.Is(err, llm.ErrBudgetUnbounded) {
		return "budget_unbounded"
	}
	if errors.Is(err, llm.ErrBudgetUnpriced) {
		return "budget_unpriced"
	}
	var limit *executionLimit
	if errors.As(err, &limit) {
		return limit.code
	}
	var provider *contract.Error
	if errors.As(err, &provider) {
		if provider.Code == contract.ErrorInsufficientQuota {
			return "quota_exceeded"
		}
		return string(provider.Code)
	}
	return "execution_failed"
}
