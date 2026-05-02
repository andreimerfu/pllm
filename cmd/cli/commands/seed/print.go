package seed

import (
	"encoding/json"
	"fmt"
	"os"
)

type printer struct {
	json bool
	log  []map[string]any
}

func newPrinter(asJSON bool) *printer {
	return &printer{json: asJSON}
}

func (p *printer) Step(s string) {
	if p.json {
		p.log = append(p.log, map[string]any{"step": s})
		return
	}
	fmt.Fprintf(os.Stderr, "\n\033[1;36m== %s ==\033[0m\n", s)
}

func (p *printer) Item(s string) {
	if p.json {
		p.log = append(p.log, map[string]any{"item": s})
		return
	}
	fmt.Fprintf(os.Stderr, "  \033[0;32m✓\033[0m %s\n", s)
}

// Result emits a key/value pair (used for the keys block at the end).
func (p *printer) Result(k string, v any) {
	if p.json {
		p.log = append(p.log, map[string]any{"result": k, "value": v})
		return
	}
	fmt.Fprintf(os.Stderr, "  %s: %v\n", k, v)
}

func (p *printer) Done(s string) {
	if p.json {
		p.log = append(p.log, map[string]any{"done": s})
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"events": p.log})
		return
	}
	fmt.Fprintf(os.Stderr, "\n\033[1;32m%s\033[0m\n", s)
}
