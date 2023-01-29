package zeroinit

import (
	"context"
	"fmt"
	"sync"

	"github.com/kendru/darwin/go/depgraph"
)

// Graph represents a directed graph.
type Graph struct {
	*depgraph.Graph
	ops map[string]*opState
}

type opState struct {
	fn    func(context.Context) error
	err   error
	fatal bool
}

// NewGraph creates a new instance of a Graph.
func NewGraph() *Graph {
	return &Graph{Graph: depgraph.New(), ops: make(map[string]*opState)}
}

func (g *Graph) AddOp(name string, opts ...GraphOption) error {
	state := &opState{}

	for _, o := range opts {
		if err := o(name, state, g); err != nil {
			return err
		}
	}
	g.ops[name] = state
	return nil
}

//func (g *Graph) Analyze()

func (g *Graph) Run(ctx context.Context) error {
	for _, layer := range g.TopoSortedLayers() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled")
		default:
			states := map[string]*opState{}

			for _, r := range layer {
				states[r] = g.ops[r]
			}

			var wg sync.WaitGroup
			for r, s := range states {
				if s.fn == nil {
					continue
				}
				fn := s.fn
				wg.Add(1)
				go func(ctx context.Context, g *Graph, key string) {
					defer wg.Done()
					g.ops[key].err = fn(ctx)
				}(ctx, g, r)
			}
			wg.Wait()

			for _, s := range states {
				if s.fatal && s.err != nil {
					return s.err
				}
			}
		}
	}
	return nil
}
