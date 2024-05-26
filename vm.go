package project

import (
	"context"
	"errors"
	"fmt"
)

type State struct {
	Mem []any
	PC  int64
}

type VM struct {
	Funcs   map[string]func(context.Context, []any) any
	program []any
	state   State
}

func (vm *VM) State() State {
	return vm.state
}

func (vm *VM) Step(ctx context.Context) error {
	if vm.state.PC >= int64(len(vm.program)) {
		return errors.New("end of program")
	}
	cmd := vm.program[vm.state.PC]
	switch t := cmd.(type) {
	case IRJumpIf:
		if t.Cond <= 0 || (t.Cond > 0 && vm.state.Mem[t.Cond].(bool)) {
			vm.state.PC = t.Label
		} else {
			vm.state.PC += 1
		}
	case IRCall:
		args := make([]any, 0, len(t.Inputs))
		for _, i := range t.Inputs {
			args = append(args, vm.state.Mem[i])
		}
		fn := vm.Funcs[t.Name]
		out := fn(ctx, args)
		if t.Output > 0 {
			vm.state.Mem[t.Output] = out
		}
		vm.state.PC += 1
	case IRGrow:
		vm.state.Mem = append(vm.state.Mem, make([]any, t.N)...)
	case IRShrink:
		vm.state.Mem = vm.state.Mem[:int64(len(vm.state.Mem))-int64(t.N)]
	default:
		panic(fmt.Errorf("unknown command %T", cmd))
	}
	return nil
}
