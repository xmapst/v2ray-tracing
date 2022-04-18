package engine

import "fmt"

func (e *Engine) output() {
	for s := range e.outputCh {
		fmt.Println(s)
	}
}
