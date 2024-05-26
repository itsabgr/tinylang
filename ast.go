package project

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Statements struct {
	List []*Statement `@@*`
}
type Block struct {
	Statements *Statements `"{" @@ "}"`
	allocs     int64
}

type Cond struct {
	Condition *Expr  `"cond" @@`
	True      *Block `@@`
	False     *Block `("else" @@)?`
}
type Loop struct {
	Name  string `"loop" @Ident?`
	Block *Block `@@`
}
type Statement struct {
	Cond  *Cond     `@@`
	Loop  *Loop     `| @@`
	Break *Break    `| @@`
	Call  *CallStmt `| @@`
	Block *Block    `| @@`
}

func (s *Statement) value() any {
	switch {
	case s.Cond != nil:
		return s.Cond
	case s.Loop != nil:
		return s.Loop
	case s.Break != nil:
		return s.Break
	case s.Call != nil:
		return s.Call
	case s.Block != nil:
		return s.Block
	default:
		panic("nil statement")
	}
}

type Break struct {
	Label string `("break" ";") | ("break" @Ident ";")`
}
type Var struct {
	Name string `"$"@Ident`
	id   int64
}
type Expr struct {
	Var  *Var      `@@`
	Expr *Expr     `| "(" @@ ")"`
	Call *CallExpr `| @@`
}

func (ex *Expr) value() any {
	switch {
	case ex.Var != nil:
		return ex.Var
	case ex.Expr != nil:
		return ex.Expr
	case ex.Call != nil:
		return ex.Call
	default:
		panic("nil expression")
	}
}

type Exprs struct {
	List []*Expr `@@*`
}
type CallExpr struct {
	Name   string `@Ident`
	Params *Exprs `@@`
}
type CallStmt struct {
	Result *Var      `(@@ "=")?`
	Expr   *CallExpr `@@ ";"`
}
type Space struct {
	Count int
	Char  string
}

type tab struct {
	id     int64
	vars   map[string]int64
	parent *tab
}

func (t *tab) born() *tab {
	return &tab{
		id:     t.id,
		parent: t,
	}
}

func (t *tab) new(name string) int64 {
	t.id += 1
	if t.vars == nil {
		t.vars = map[string]int64{}
	}
	t.vars[name] = t.id
	return t.id
}
func (t *tab) sym(name string) int64 {
	id := t.lookup(name)
	if id >= 0 {
		return id
	}
	return t.new(name)
}
func (t *tab) lookup(name string) int64 {
	tmp := t
	for {
		if tmp == nil {
			return -1
		}
		if len(tmp.vars) == 0 {
			tmp = tmp.parent
			continue
		}
		id, has := tmp.vars[name]
		if !has {
			tmp = tmp.parent
			continue
		}
		return id
	}
}

func createVarName() string {
	return "$" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
func extractVarsIdFromExprs(exprs ...*Expr) []int64 {
	vars := []int64{}
	for _, expr := range exprs {
		switch t := expr.value().(type) {
		case *Expr:
			vars = append(vars, extractVarsIdFromExprs(t)...)
		case *Var:
			vars = append(vars, t.id)
		case *CallExpr:
			panic(errors.New("call expr is not supported yets"))
		default:
			panic(fmt.Errorf("unknown expr %T", expr))
		}
	}
	return vars
}

type IRCall struct {
	Name   string
	Output int64
	Inputs []int64
}

func (ir IRCall) String() string {
	return fmt.Sprintf("{%d=%s%v}", ir.Output, ir.Name, ir.Inputs)
}
func compileIR(ir []any) []any {
	labels := getLabelPcs(ir)
	result := []any{}
	for _, inst := range ir {
		switch t := inst.(type) {
		case IRJump:
			pc := labels[t.Label]
			result = append(result, IRJumpIf{0, IRJump{pc}})
		case IRJumpIf:
			pc := labels[t.Label]
			result = append(result, IRJumpIf{t.Cond, IRJump{pc}})
		case IRLabel:
		case IRCall:
			result = append(result, t)
		case IRGrow:
			result = append(result, t)
		case IRShrink:
			result = append(result, t)
		default:
			panic(fmt.Errorf("unexpected ir %T", t))
		}
	}
	return result
}
func getLabelPcs(ir []any) map[int64]int64 {
	pc := int64(0)
	labels := map[int64]int64{}
	for _, inst := range ir {
		switch t := inst.(type) {
		case IRGrow:
			pc++
		case IRShrink:
			pc++
		case IRJump:
			pc++
		case IRJumpIf:
			pc++
		case IRLabel:
			labels[t.Id] = pc
		case IRCall:
			pc++
		default:
			panic(fmt.Errorf("unexpected ir %T", t))
		}
	}
	return labels
}

type IRJump struct {
	Label int64
}

func (ir IRJump) String() string {
	return fmt.Sprintf("{%d}", ir.Label)
}

type IRLabel struct {
	Id int64
}

func (ir IRLabel) String() string {
	return fmt.Sprintf("{:%d}", ir.Id)
}

type IRJumpIf struct {
	Cond int64
	IRJump
}
type IRGrow struct {
	N int64
}

func (ir IRGrow) String() string {
	return fmt.Sprintf("{+%d}", ir.N)
}

type IRShrink struct {
	N int64
}

func (ir IRShrink) String() string {
	return fmt.Sprintf("{-%d}", ir.N)
}
func (ir IRJumpIf) String() string {
	if ir.Cond <= 0 {
		return ir.IRJump.String()
	}
	return fmt.Sprintf("{%d?%d}", ir.Cond, ir.IRJump.Label)
}

func compileAST(loops map[string]int64, label *int64, node any) []any {
	if v := reflect.ValueOf(node); v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	commands := []any{}
	switch t := node.(type) {
	case *CallExpr:
		panic(errors.New("call expr is not supported yets"))
	case *CallStmt:
		vars := extractVarsIdFromExprs(t.Expr.Params.List...)
		inst := IRCall{
			Name:   t.Expr.Name,
			Inputs: vars,
		}
		if t.Result != nil {
			inst.Output = t.Result.id
		}
		commands = append(commands, inst)
	case *Statement:
		commands = append(commands, compileAST(loops, label, t.value())...)

	case *Statements:
		for _, stmt := range t.List {
			commands = append(commands, compileAST(loops, label, stmt)...)
		}
	case *Cond:
		cond := extractVarsIdFromExprs(t.Condition)[0]
		trueBlock := compileAST(loops, label, t.True)
		falseBlock := compileAST(loops, label, t.False)
		*label += 2
		startOfTrue := *label - 1
		endOfTrue := *label
		//cond? goto startOfTrue
		//false (optional)
		//goto endOfTrue
		//startOfTrue
		//true
		//endOfTrue
		commands = append(commands, IRJumpIf{cond, IRJump{startOfTrue}})
		commands = append(commands, falseBlock...)
		commands = append(commands, IRJump{endOfTrue}, IRLabel{startOfTrue})
		commands = append(commands, trueBlock...)
		commands = append(commands, IRLabel{endOfTrue})
	case *Break:
		endOfLoop, _ := loops[t.Label]
		if endOfLoop <= 0 {
			panic(errors.New("bad break"))
		}
		commands = append(commands, IRJump{endOfLoop})
	case *Loop:
		*label += 2
		startOfLoop := *label - 1
		endOfLoop := *label
		endOfParentLoop, hasParentLoop := loops[""]
		loops[""] = endOfLoop
		if t.Name != "" {
			if _, duplicate := loops[t.Name]; duplicate {
				panic(fmt.Errorf("conflict loop name %q", t.Name))
			}
			loops[t.Name] = endOfLoop
		}
		loopBlock := compileAST(loops, label, t.Block.Statements)
		delete(loops, t.Name)
		if hasParentLoop {
			loops[""] = endOfParentLoop
		}
		//grow if allocs
		//startOfLoop
		//loop
		//goto startOfLoop
		//endOfLoop
		//shrink if grew
		if t.Block.allocs > 0 {
			commands = append(commands, IRGrow{t.Block.allocs})
		}
		commands = append(commands, IRLabel{startOfLoop})
		commands = append(commands, loopBlock...)
		commands = append(commands, IRJump{startOfLoop})
		commands = append(commands, IRLabel{endOfLoop})
		if t.Block.allocs > 0 {
			commands = append(commands, IRShrink{t.Block.allocs})
		}
	case *Block:
		block := compileAST(loops, label, t.Statements)
		if t.allocs > 0 {
			commands = append(commands, IRGrow{t.allocs})
			block = append(block, IRShrink{t.allocs})
		}
		commands = append(commands, block...)
	default:
		panic(fmt.Errorf("unexpected node %T", node))
	}
	return commands
}
func (t *tab) size() int64 {
	return int64(len(t.vars))
}
func giveIdToVariables(sym *tab, node any) {
	if v := reflect.ValueOf(node); v.Kind() == reflect.Pointer && v.IsNil() {
		return
	}
	switch t := node.(type) {
	case *Break:
		break
	case *Statement:
		giveIdToVariables(sym, t.value())
	case *CallExpr:
		giveIdToVariables(sym, t.Params)
	case *CallStmt:
		giveIdToVariables(sym, t.Result)
		giveIdToVariables(sym, t.Expr)
	case *Expr:
		giveIdToVariables(sym, t.value())
	case *Cond:
		giveIdToVariables(sym, t.Condition)
		giveIdToVariables(sym, t.True)
		giveIdToVariables(sym, t.False)
	case *Exprs:
		for _, expr := range t.List {
			giveIdToVariables(sym, expr)
		}
	case *Statements:
		for _, stmt := range t.List {
			giveIdToVariables(sym, stmt)
		}
	case *Loop:
		giveIdToVariables(sym, t.Block)
	case *Block:
		isym := sym.born()
		giveIdToVariables(isym, t.Statements)
		t.allocs = isym.size()
	case *Var:
		t.id = sym.sym(t.Name)
	default:
		panic(fmt.Errorf("unknown node %T", node))
	}
}
func format(space Space, res *strings.Builder, node any) {
	if v := reflect.ValueOf(node); v.Kind() == reflect.Pointer && v.IsNil() {
		return
	}
	switch t := node.(type) {
	case Space:
		for range space.Count {
			res.WriteString(space.Char)
		}
	case string:
		res.WriteString(t)
	case *Statement:
		space.Count++
		format(space, res, space)
		format(space, res, t.value())
	case *CallExpr:
		format(space, res, t.Name)
		if len(t.Params.List) > 0 {
			format(space, res, " ")
			format(space, res, t.Params)
		}
	case *CallStmt:
		if t.Result != nil {
			format(space, res, t.Result)
			format(space, res, " = ")
		}
		format(space, res, t.Expr)
		format(space, res, ";")
	case *Expr:
		switch t.value().(type) {
		case *Expr:
			format(space, res, "(")
			format(space, res, t.Expr)
			format(space, res, ")")
		default:
			format(space, res, t.value())
		}
	case *Cond:
		format(space, res, "cond ")
		format(space, res, t.Condition)
		format(space, res, " ")
		format(space, res, t.True)
		if t.False != nil {
			format(space, res, " else ")
			format(space, res, t.False)
		}
	case *Exprs:
		for i, expr := range t.List {
			format(space, res, expr)
			if i < len(t.List)-1 {
				format(space, res, " ")
			}
		}
	case *Statements:
		for _, stmt := range t.List {
			format(space, res, stmt)
			format(space, res, "\n")
		}
	case *Block:
		if len(t.Statements.List) == 0 {
			format(space, res, "{}")
		} else {
			format(space, res, "{\n")
			format(space, res, t.Statements)
			format(space, res, space)
			format(space, res, "}")
		}
	case *Var:
		res.WriteString(fmt.Sprint(t.id))
		format(space, res, "$")
		format(space, res, t.Name)
	case *Break:
		format(space, res, "break")
		if t.Label != "" {
			format(space, res, " ")
			format(space, res, t.Label)
		}
		format(space, res, ";")
	case *Loop:

		format(space, res, "loop ")
		if t.Name != "" {
			format(space, res, t.Name)
			format(space, res, " ")
		}
		format(space, res, t.Block)
	default:
		panic(fmt.Errorf("unknown node %T", node))
	}
}
