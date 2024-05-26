package project

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alecthomas/participle/v2"
)

func TestCompile(t *testing.T) {
	parser, err := participle.Build[Block]()
	if err != nil {
		panic(err)
	}
	block, err := parser.ParseString("", `
	{
		$a = true;
		loop {
			$b = false;
			cond $a {
				 print $b;
				 break;
			}
			print $a;
		}
	}
	`)
	if err != nil {
		panic(err)
	}
	str := &strings.Builder{}
	giveIdToVariables(&tab{}, block)
	format(Space{0, "\t"}, str, block)
	fmt.Println(str.String())
	lid := int64(0)
	program := compileIR(compileAST(map[string]int64{}, &lid, block))
	vm := &VM{
		Funcs:   make(map[string]func(context.Context, []any) any),
		program: program,
		state: State{
			Mem: nil,
			PC:  0,
		},
	}
	vm.Funcs["true"] = func(ctx context.Context, a []any) any {
		return true
	}
	vm.Funcs["false"] = func(ctx context.Context, a []any) any {
		return false
	}
	vm.Funcs["print"] = func(ctx context.Context, a []any) any {
		fmt.Println(a...)
		return nil
	}
	fmt.Println(program...)
	// TODO shrink stack on break to higher parent with label
	for range 100 {
		if err := vm.Step(context.Background()); err != nil {
			break
		}
	}
}
