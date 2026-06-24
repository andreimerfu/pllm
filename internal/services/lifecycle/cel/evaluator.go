package cel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	"github.com/amerfu/pllm/internal/core/models"
)

// EvalTimeout is the per-row hard cap on CEL evaluation. CEL is
// microsecond-fast for our predicates, so this only bites if someone
// writes an exponential blowup expression — at which point we want
// the daemon to stay responsive rather than block the tick.
const EvalTimeout = 200 * time.Millisecond

// CompiledRule is a parsed + type-checked CEL program bound to the
// scan target it was compiled against. A bad expression is rejected
// at Compile time — we don't carry compilation errors into Run.
type CompiledRule struct {
	Target  models.LifecycleScanTarget
	Program cel.Program
	Source  string // raw expression, kept for diagnostics
}

// Compile builds an environment for the given scan target, parses +
// type-checks the expression, and returns a runnable program. The
// returned error is wrapped so a config-load failure points operators
// at the offending rule name without further context plumbing.
func Compile(target models.LifecycleScanTarget, expr string) (*CompiledRule, error) {
	if expr == "" {
		return nil, errors.New("empty expression")
	}
	env, err := cel.NewEnv(envOptionsForTarget(target)...)
	if err != nil {
		return nil, fmt.Errorf("cel env: %w", err)
	}
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile: %w", issues.Err())
	}
	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("expression must return bool, got %s", ast.OutputType())
	}
	prg, err := env.Program(ast,
		cel.EvalOptions(cel.OptOptimize),
		cel.InterruptCheckFrequency(64),
	)
	if err != nil {
		return nil, fmt.Errorf("program: %w", err)
	}
	return &CompiledRule{Target: target, Program: prg, Source: expr}, nil
}

// Eval runs the compiled program against a single row's input map.
// The caller is expected to have produced the input via the matching
// *ToInput adapter — passing a mismatched shape returns an evaluation
// error rather than a panic, which is what we want when an upstream
// change quietly breaks the schema.
//
// `now` is provided once per evaluation so all rows in a single
// daemon tick see the same wall-clock — important for "older than X"
// rules to behave deterministically across a large scan.
func (r *CompiledRule) Eval(ctx context.Context, input map[string]any, now time.Time) (bool, error) {
	if r == nil || r.Program == nil {
		return false, errors.New("compiled rule is nil")
	}
	ctx, cancel := context.WithTimeout(ctx, EvalTimeout)
	defer cancel()

	vars := map[string]any{
		scanInputVar(r.Target): input,
		"now":                  now,
	}
	out, _, err := r.Program.ContextEval(ctx, vars)
	if err != nil {
		return false, fmt.Errorf("eval: %w", err)
	}
	return refToBool(out)
}

// refToBool extracts a Go bool from a CEL ref.Val, treating any
// non-bool as an error. We compile-time enforce that expressions
// return bool, but a defensive check here keeps panics out of the
// hot path if cel-go internals ever surface a wrapped value.
func refToBool(v ref.Val) (bool, error) {
	if v == nil {
		return false, errors.New("nil eval result")
	}
	if b, ok := v.Value().(bool); ok {
		return b, nil
	}
	if v == types.True {
		return true, nil
	}
	if v == types.False {
		return false, nil
	}
	return false, fmt.Errorf("non-bool result: %T", v.Value())
}
