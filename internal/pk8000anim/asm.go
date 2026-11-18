package pk8000anim

import (
	"fmt"
	"strconv"
	"strings"
)

// Мини-ассемблер КР580ВМ80А (8080): полный набор команд,
// директивы ORG, EQU, DB, DW, DS, END. Два прохода.

type asmLine struct {
	num   int
	label string
	mnem  string
	args  []string
}

type Assembler struct {
	lines  []asmLine
	labels map[string]int
	base   int // адрес ORG
	out    []byte
}

var regs8 = map[string]byte{"b": 0, "c": 1, "d": 2, "e": 3, "h": 4, "l": 5, "m": 6, "a": 7}
var regsRP = map[string]byte{"b": 0, "d": 1, "h": 2, "sp": 3}
var regsRP2 = map[string]byte{"b": 0, "d": 1, "h": 2, "psw": 3}

func Assemble(src string) (base int, code []byte, err error) {
	a := &Assembler{labels: map[string]int{}}
	if err := a.parse(src); err != nil {
		return 0, nil, err
	}
	if err := a.pass1(); err != nil {
		return 0, nil, err
	}
	if err := a.pass2(); err != nil {
		return 0, nil, err
	}
	return a.base, a.out, nil
}

func (a *Assembler) parse(src string) error {
	for i, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		var l asmLine
		l.num = i + 1
		// метка: ИМЯ:  или  ИМЯ EQU значение
		if tok, rest := nextToken(line); tok != "" && isIdent(tok) {
			tok2, rest2 := nextToken(rest)
			if tok2 == ":" {
				l.label = tok
				line = strings.TrimSpace(rest2)
			} else if strings.EqualFold(tok2, "EQU") {
				l.label = tok
				l.mnem = "EQU"
				line = strings.TrimSpace(rest2)
			} else {
				l.mnem = strings.ToUpper(tok)
				line = strings.TrimSpace(rest)
			}
		} else if tok == ":" {
			line = strings.TrimSpace(rest)
		}
		if l.mnem != "" {
			l.args = splitArgs(line)
		} else if line != "" {
			tok, rest := nextToken(line)
			if tok == "" {
				return fmt.Errorf("строка %d: не удалось разобрать: %s", l.num, raw)
			}
			l.mnem = strings.ToUpper(tok)
			l.args = splitArgs(strings.TrimSpace(rest))
		}
		a.lines = append(a.lines, l)
	}
	return nil
}

func stripComment(s string) string {
	inStr := false
	for i, r := range s {
		switch r {
		case '"':
			inStr = !inStr
		case ';':
			if !inStr {
				return s[:i]
			}
		}
	}
	return s
}

func nextToken(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if strings.ContainsRune(":+-,", rune(s[0])) {
		return s[:1], s[1:]
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == ':' || c == '+' || c == '-' || c == ',' {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func splitArgs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	inQ := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQ = !inQ
			cur.WriteByte(c)
		case c == ',' && !inQ:
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	out = append(out, strings.TrimSpace(cur.String()))
	return out
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	return c == '_' || c == '.' || c == '$' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isDirective(s string) bool {
	switch strings.ToUpper(s) {
	case "ORG", "EQU", "DB", "DEFB", "DW", "DEFW", "DS", "DEFS", "END":
		return true
	}
	return false
}

var mnemonics = map[string]bool{}

func init() {
	for _, m := range strings.Fields(`
		MOV MVI LXI LDA STA LHLD SHLD LDAX STAX XCHG XTHL SPHL PCHL
		ADD ADC SUB SBB ANA XRA ORA CMP ADI ACI SUI SBI ANI XRI ORI CPI
		INR DCR INX DCX DAD DAA CMA CMC STC RLC RRC RAL RAR
		JMP JNZ JZ JNC JC JPO JPE JP JM CALL CNZ CZ CNC CC CPO CPE CP CM
		RET RNZ RZ RNC RC RPO RPE RP RM RST PUSH POP IN OUT
		NOP HLT EI DI`) {
		mnemonics[strings.ToUpper(m)] = true
	}
}

func isMnemonic(s string) bool { return mnemonics[strings.ToUpper(s)] }

// sizeOf возвращает размер команды/директивы в байтах (для прохода 1).
func (a *Assembler) sizeOf(l asmLine) (int, error) {
	switch l.mnem {
	case "", "EQU", "END":
		return 0, nil
	case "ORG":
		return 0, nil
	case "DS", "DEFS":
		n, err := a.eval(l.args[0], true)
		if err != nil {
			return 0, fmt.Errorf("строка %d: %w", l.num, err)
		}
		return n, nil
	case "DB", "DEFB":
		n := 0
		for _, arg := range l.args {
			if strings.HasPrefix(arg, "\"") {
				n += len(unescape(arg))
			} else {
				n++
			}
		}
		return n, nil
	case "DW", "DEFW":
		return 2 * len(l.args), nil
	}
	switch l.mnem {
	case "MOV", "INR", "DCR", "ADD", "ADC", "SUB", "SBB", "ANA", "XRA", "ORA", "CMP", "RST":
		return 1, nil
	case "MVI", "ADI", "ACI", "SUI", "SBI", "ANI", "XRI", "ORI", "CPI", "IN", "OUT":
		return 2, nil
	case "LXI", "LDA", "STA", "LHLD", "SHLD", "JMP", "JNZ", "JZ", "JNC", "JC", "JPO", "JPE", "JP", "JM",
		"CALL", "CNZ", "CZ", "CNC", "CC", "CPO", "CPE", "CP", "CM":
		return 3, nil
	default:
		return 1, nil
	}
}

func (a *Assembler) pass1() error {
	pc := 0
	started := false
	for _, l := range a.lines {
		if l.mnem == "ORG" {
			v, err := a.eval(l.args[0], true)
			if err != nil {
				return fmt.Errorf("строка %d: %w", l.num, err)
			}
			pc = v
			if !started {
				a.base = v
				started = true
			}
			continue
		}
		if l.label != "" && l.mnem != "EQU" {
			key := strings.ToUpper(l.label)
			if _, dup := a.labels[key]; dup {
				return fmt.Errorf("строка %d: метка %s уже определена (имена регистронезависимы)", l.num, l.label)
			}
			a.labels[key] = pc
		}
		sz, err := a.sizeOf(l)
		if err != nil {
			return err
		}
		if l.mnem == "EQU" {
			v, err := a.eval(l.args[0], true)
			if err != nil {
				return fmt.Errorf("строка %d: %w", l.num, err)
			}
			key := strings.ToUpper(l.label)
			if _, dup := a.labels[key]; dup {
				return fmt.Errorf("строка %d: метка %s уже определена (имена регистронезависимы)", l.num, l.label)
			}
			a.labels[key] = v
			continue
		}
		pc += sz
	}
	if !started {
		a.base = 0
	}
	return nil
}

func (a *Assembler) pass2() error {
	pc := a.base
	start := a.base
	end := a.base
	started := false
	emit := func(bs ...byte) {
		if !started {
			start = pc
			started = true
		}
		a.out = append(a.out, bs...)
		pc += len(bs)
		end = pc
	}
	for _, l := range a.lines {
		switch l.mnem {
		case "", "EQU", "END":
			continue
		case "ORG":
			v, err := a.eval(l.args[0], true)
			if err != nil {
				return fmt.Errorf("строка %d: %w", l.num, err)
			}
			if started && v != pc {
				if v < end {
					return fmt.Errorf("строка %d: ORG %04Xh пересекает уже выведенные данные", l.num, v)
				}
				for pc < v {
					emit(0)
				}
			}
			pc = v
			continue
		case "DS", "DEFS":
			n, err := a.eval(l.args[0], true)
			if err != nil {
				return fmt.Errorf("строка %d: %w", l.num, err)
			}
			for i := 0; i < n; i++ {
				emit(0)
			}
			continue
		case "DB", "DEFB":
			for _, arg := range l.args {
				if strings.HasPrefix(arg, "\"") {
					emit([]byte(unescape(arg))...)
					continue
				}
				v, err := a.eval(arg, false)
				if err != nil {
					return fmt.Errorf("строка %d: %w", l.num, err)
				}
				emit(byte(v))
			}
			continue
		case "DW", "DEFW":
			for _, arg := range l.args {
				v, err := a.eval(arg, false)
				if err != nil {
					return fmt.Errorf("строка %d: %w", l.num, err)
				}
				emit(byte(v), byte(v>>8))
			}
			continue
		}
		code, err := a.encode(l, pc)
		if err != nil {
			return fmt.Errorf("строка %d: %w", l.num, err)
		}
		if sz, _ := a.sizeOf(l); sz != len(code) {
			return fmt.Errorf("строка %d: внутренняя ошибка: размер %s = %d, ожидался %d",
				l.num, l.mnem, len(code), sz)
		}
		emit(code...)
	}
	a.out = a.out[:end-start]
	_ = start
	return nil
}

func reg8(name string) (byte, bool) { v, ok := regs8[strings.ToLower(name)]; return v, ok }
func regRP(name string) (byte, bool) { v, ok := regsRP[strings.ToLower(name)]; return v, ok }
func regRP2(name string) (byte, bool) { v, ok := regsRP2[strings.ToLower(name)]; return v, ok }

func (a *Assembler) encode(l asmLine, pc int) ([]byte, error) {
	m := l.mnem
	args := l.args
	need := func(n int) error {
		if len(args) != n {
			return fmt.Errorf("%s: ожидалось операндов: %d", m, n)
		}
		return nil
	}
	d8 := func(i int) (byte, error) {
		v, err := a.eval(args[i], false)
		if err != nil {
			return 0, err
		}
		if v < -128 || v > 255 {
			return 0, fmt.Errorf("значение %d не помещается в байт", v)
		}
		return byte(v), nil
	}
	d16 := func(i int) (int, error) {
		v, err := a.eval(args[i], false)
		if err != nil {
			return 0, err
		}
		if v < 0 || v > 0xFFFF {
			return 0, fmt.Errorf("значение %d не помещается в слово", v)
		}
		return v, nil
	}

	switch m {
	case "MOV":
		if err := need(2); err != nil {
			return nil, err
		}
		r1, ok1 := reg8(args[0])
		r2, ok2 := reg8(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("MOV: неверный регистр")
		}
		if r1 == 6 && r2 == 6 {
			return nil, fmt.Errorf("MOV M,M недопустим")
		}
		return []byte{0x40 | r1<<3 | r2}, nil
	case "MVI":
		if err := need(2); err != nil {
			return nil, err
		}
		r, ok := reg8(args[0])
		if !ok {
			return nil, fmt.Errorf("MVI: неверный регистр")
		}
		v, err := d8(1)
		if err != nil {
			return nil, err
		}
		return []byte{0x06 | r<<3, v}, nil
	case "LXI":
		if err := need(2); err != nil {
			return nil, err
		}
		rp, ok := regRP(args[0])
		if !ok {
			return nil, fmt.Errorf("LXI: неверная пара регистров")
		}
		v, err := d16(1)
		if err != nil {
			return nil, err
		}
		return []byte{0x01 | rp<<4, byte(v), byte(v >> 8)}, nil
	case "LDA":
		return a.op16(0x3A, args)
	case "STA":
		return a.op16(0x32, args)
	case "LHLD":
		return a.op16(0x2A, args)
	case "SHLD":
		return a.op16(0x22, args)
	case "LDAX", "STAX":
		if err := need(1); err != nil {
			return nil, err
		}
		rp, ok := regRP(args[0])
		if !ok || rp > 1 {
			return nil, fmt.Errorf("%s: допустимы только B и D", m)
		}
		op := byte(0x0A)
		if m == "STAX" {
			op = 0x02
		}
		return []byte{op | rp<<4}, nil
	case "XCHG":
		return []byte{0xEB}, nil
	case "XTHL":
		return []byte{0xE3}, nil
	case "SPHL":
		return []byte{0xF9}, nil
	case "PCHL":
		return []byte{0xE9}, nil

	case "ADD", "ADC", "SUB", "SBB", "ANA", "XRA", "ORA", "CMP":
		if err := need(1); err != nil {
			return nil, err
		}
		r, ok := reg8(args[0])
		if !ok {
			return nil, fmt.Errorf("%s: неверный регистр", m)
		}
		base := map[string]byte{"ADD": 0x80, "ADC": 0x88, "SUB": 0x90, "SBB": 0x98,
			"ANA": 0xA0, "XRA": 0xA8, "ORA": 0xB0, "CMP": 0xB8}[m]
		return []byte{base | r}, nil
	case "ADI", "ACI", "SUI", "SBI", "ANI", "XRI", "ORI", "CPI":
		if err := need(1); err != nil {
			return nil, err
		}
		v, err := d8(0)
		if err != nil {
			return nil, err
		}
		op := map[string]byte{"ADI": 0xC6, "ACI": 0xCE, "SUI": 0xD6, "SBI": 0xDE,
			"ANI": 0xE6, "XRI": 0xEE, "ORI": 0xF6, "CPI": 0xFE}[m]
		return []byte{op, v}, nil
	case "INR", "DCR":
		if err := need(1); err != nil {
			return nil, err
		}
		r, ok := reg8(args[0])
		if !ok {
			return nil, fmt.Errorf("%s: неверный регистр", m)
		}
		op := byte(0x04)
		if m == "DCR" {
			op = 0x05
		}
		return []byte{op | r<<3}, nil
	case "INX", "DCX", "DAD":
		if err := need(1); err != nil {
			return nil, err
		}
		rp, ok := regRP(args[0])
		if !ok {
			return nil, fmt.Errorf("%s: неверная пара регистров", m)
		}
		op := map[string]byte{"INX": 0x03, "DCX": 0x0B, "DAD": 0x09}[m]
		return []byte{op | rp<<4}, nil
	case "DAA":
		return []byte{0x27}, nil
	case "CMA":
		return []byte{0x2F}, nil
	case "CMC":
		return []byte{0x3F}, nil
	case "STC":
		return []byte{0x37}, nil
	case "RLC":
		return []byte{0x07}, nil
	case "RRC":
		return []byte{0x0F}, nil
	case "RAL":
		return []byte{0x17}, nil
	case "RAR":
		return []byte{0x1F}, nil

	case "JMP", "JNZ", "JZ", "JNC", "JC", "JPO", "JPE", "JP", "JM":
		op := map[string]byte{"JMP": 0xC3, "JNZ": 0xC2, "JZ": 0xCA, "JNC": 0xD2, "JC": 0xDA,
			"JPO": 0xE2, "JPE": 0xEA, "JP": 0xF2, "JM": 0xFA}[m]
		return a.op16(op, args)
	case "CALL", "CNZ", "CZ", "CNC", "CC", "CPO", "CPE", "CP", "CM":
		op := map[string]byte{"CALL": 0xCD, "CNZ": 0xC4, "CZ": 0xCC, "CNC": 0xD4, "CC": 0xDC,
			"CPO": 0xE4, "CPE": 0xEC, "CP": 0xF4, "CM": 0xFC}[m]
		return a.op16(op, args)
	case "RET", "RNZ", "RZ", "RNC", "RC", "RPO", "RPE", "RP", "RM":
		op := map[string]byte{"RET": 0xC9, "RNZ": 0xC0, "RZ": 0xC8, "RNC": 0xD0, "RC": 0xD8,
			"RPO": 0xE0, "RPE": 0xE8, "RP": 0xF0, "RM": 0xF8}[m]
		return []byte{op}, nil
	case "RST":
		if err := need(1); err != nil {
			return nil, err
		}
		v, err := a.eval(args[0], false)
		if err != nil {
			return nil, err
		}
		if v < 0 || v > 7 {
			return nil, fmt.Errorf("RST: допустимы 0..7")
		}
		return []byte{0xC7 | byte(v)<<3}, nil
	case "PUSH", "POP":
		if err := need(1); err != nil {
			return nil, err
		}
		rp, ok := regRP2(args[0])
		if !ok {
			return nil, fmt.Errorf("%s: допустимы B, D, H, PSW", m)
		}
		op := byte(0xC5)
		if m == "POP" {
			op = 0xC1
		}
		return []byte{op | rp<<4}, nil
	case "IN":
		if err := need(1); err != nil {
			return nil, err
		}
		v, err := d8(0)
		if err != nil {
			return nil, err
		}
		return []byte{0xDB, v}, nil
	case "OUT":
		if err := need(1); err != nil {
			return nil, err
		}
		v, err := d8(0)
		if err != nil {
			return nil, err
		}
		return []byte{0xD3, v}, nil
	case "NOP":
		return []byte{0x00}, nil
	case "HLT":
		return []byte{0x76}, nil
	case "EI":
		return []byte{0xFB}, nil
	case "DI":
		return []byte{0xF3}, nil
	}
	return nil, fmt.Errorf("неизвестная команда %s", m)
}

func (a *Assembler) op16(op byte, args []string) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("ожидался один операнд")
	}
	v, err := a.eval(args[0], false)
	if err != nil {
		return nil, err
	}
	if v < 0 || v > 0xFFFF {
		return nil, fmt.Errorf("адрес %d вне диапазона 0..65535", v)
	}
	return []byte{op, byte(v), byte(v >> 8)}, nil
}

// eval вычисляет выражение: [+|-] term [(+|-) term],
// term = число | 'символ' | метка | $ (текущий адрес).
// firstPass=true — допускаются только ранее определённые значения.
func (a *Assembler) eval(expr string, firstPass bool) (int, error) {
	expr = strings.ReplaceAll(expr, " ", "")
	if expr == "" {
		return 0, fmt.Errorf("пустое выражение")
	}
	type term struct {
		sign int
		text string
	}
	var terms []term
	sign := 1
	i := 0
	if expr[0] == '+' || expr[0] == '-' {
		if expr[0] == '-' {
			sign = -1
		}
		i = 1
	}
	cur := strings.Builder{}
	flush := func() {
		if cur.Len() > 0 {
			terms = append(terms, term{sign, cur.String()})
			cur.Reset()
			sign = 1
		}
	}
	for ; i < len(expr); i++ {
		c := expr[i]
		if c == '+' || c == '-' {
			flush()
			if c == '-' {
				sign = -1
			} else {
				sign = 1
			}
			continue
		}
		cur.WriteByte(c)
	}
	flush()
	total := 0
	for _, t := range terms {
		v, err := a.term(t.text, firstPass)
		if err != nil {
			return 0, err
		}
		total += t.sign * v
	}
	return total, nil
}

func (a *Assembler) term(text string, firstPass bool) (int, error) {
	if text == "" {
		return 0, fmt.Errorf("пустой терм")
	}
	if text == "$" {
		return 0, fmt.Errorf("$ не поддерживается в этом контексте")
	}
	if len(text) >= 3 && text[0] == '\'' && text[len(text)-1] == '\'' {
		s := unescape(text)
		if len(s) != 1 {
			return 0, fmt.Errorf("неверный символьный литерал %s", text)
		}
		return int(s[0]), nil
	}
	if text[0] >= '0' && text[0] <= '9' {
		if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
			v, err := strconv.ParseUint(text[2:], 16, 32)
			if err != nil {
				return 0, fmt.Errorf("неверное шестнадцатеричное число %s", text)
			}
			return int(v), nil
		}
		if strings.HasSuffix(text, "h") || strings.HasSuffix(text, "H") {
			v, err := strconv.ParseUint(text[:len(text)-1], 16, 32)
			if err != nil {
				return 0, fmt.Errorf("неверное шестнадцатеричное число %s", text)
			}
			return int(v), nil
		}
		v, err := strconv.Atoi(text)
		if err != nil {
			return 0, fmt.Errorf("неверное число %s", text)
		}
		return v, nil
	}
	if v, ok := a.labels[strings.ToUpper(text)]; ok {
		return v, nil
	}
	if firstPass {
		return 0, fmt.Errorf("метка %s ещё не определена", text)
	}
	return 0, fmt.Errorf("неизвестная метка %s", text)
}

func unescape(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	} else if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
	}
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\'`, `'`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\0`, "\x00")
	return s
}
