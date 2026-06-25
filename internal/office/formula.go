package office

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	maxFormulaColumn     = 16384
	maxFormulaRow        = 1048576
	maxFormulaBytes      = 4096
	maxFormulaRangeCells = 100000
)

type formulaResult struct {
	number   float64
	text     string
	isText   bool
	isEmpty  bool
	isRange  bool
	elements []formulaResult
}

func numericResult(v float64) formulaResult {
	return formulaResult{number: v}
}

func textResult(s string) formulaResult {
	return formulaResult{text: s, isText: true}
}

func emptyResult() formulaResult {
	return formulaResult{isEmpty: true}
}

func (r formulaResult) String() string {
	if r.isEmpty {
		return "0"
	}
	if r.isRange {
		sum := 0.0
		for _, el := range r.elements {
			if n, ok := el.toNumber(); ok {
				sum += n
			}
		}
		if sum == 0 {
			return "0"
		}
		return strconv.FormatFloat(sum, 'f', -1, 64)
	}
	if r.isText {
		return r.text
	}
	if r.number == 0 {
		return "0"
	}
	if !isFinite(r.number) {
		return "#ERR"
	}
	return strconv.FormatFloat(r.number, 'f', -1, 64)
}

func (r formulaResult) toNumber() (float64, bool) {
	if r.isEmpty {
		return 0, true
	}
	if r.isRange {
		sum := 0.0
		for _, el := range r.elements {
			if n, ok := el.toNumber(); ok {
				sum += n
			}
		}
		return sum, true
	}
	if r.isText {
		n, err := strconv.ParseFloat(r.text, 64)
		if err == nil && isFinite(n) {
			return n, true
		}
		return 0, false
	}
	return r.number, true
}

func EvaluateFormulaForSheet(sheet Sheet, formula string) (string, error) {
	result, err := evaluateFormula(sheet, formula, map[string]bool{})
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func evaluateFormula(sheet Sheet, formula string, visiting map[string]bool) (formulaResult, error) {
	formula = strings.TrimSpace(formula)
	if len(formula) > maxFormulaBytes {
		return formulaResult{}, fmt.Errorf("formula exceeds %d bytes", maxFormulaBytes)
	}
	formula = strings.TrimPrefix(formula, "=")
	if formula == "" {
		return formulaResult{}, fmt.Errorf("formula is empty")
	}
	if strings.ContainsAny(formula, "![]") {
		return formulaResult{}, fmt.Errorf("external and sheet references are not supported")
	}
	tokens, err := lexFormula(formula)
	if err != nil {
		return formulaResult{}, err
	}
	parser := formulaParser{
		tokens:   tokens,
		sheet:    sheet,
		visiting: visiting,
	}
	value, err := parser.parseComparison()
	if err != nil {
		return formulaResult{}, err
	}
	if parser.peek().typ != formulaTokenEOF {
		return formulaResult{}, fmt.Errorf("invalid token %q", parser.peek().literal)
	}
	return value, nil
}

type formulaTokenType int

const (
	formulaTokenEOF formulaTokenType = iota
	formulaTokenNumber
	formulaTokenString
	formulaTokenCell
	formulaTokenIdent
	formulaTokenPlus
	formulaTokenMinus
	formulaTokenStar
	formulaTokenSlash
	formulaTokenLParen
	formulaTokenRParen
	formulaTokenComma
	formulaTokenColon
	formulaTokenCmp
)

type formulaToken struct {
	typ     formulaTokenType
	literal string
	number  float64
}

func lexFormula(input string) ([]formulaToken, error) {
	tokens := []formulaToken{}
	for i := 0; i < len(input); {
		ch := input[i]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			i++
		case isFormulaDigit(ch) || ch == '.':
			token, next, err := lexFormulaNumber(input, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			i = next
		case ch == '"':
			i++
			var sb strings.Builder
			for i < len(input) && input[i] != '"' {
				if input[i] == '\\' && i+1 < len(input) {
					i++
					sb.WriteByte(input[i])
				} else {
					sb.WriteByte(input[i])
				}
				i++
			}
			if i < len(input) {
				i++
			}
			tokens = append(tokens, formulaToken{typ: formulaTokenString, literal: sb.String()})
		case ch == '<' || ch == '>' || ch == '!' || ch == '=':
			op := string(ch)
			i++
			if i < len(input) && (input[i] == '=' || (ch == '<' && input[i] == '>')) {
				op += string(input[i])
				i++
			}
			if op == "=" || op == "<>" || op == "<" || op == ">" || op == "<=" || op == ">=" {
				tokens = append(tokens, formulaToken{typ: formulaTokenCmp, literal: op})
			} else {
				return nil, fmt.Errorf("invalid operator %q", op)
			}
		case isFormulaLetter(ch):
			start := i
			for i < len(input) && isFormulaLetter(input[i]) {
				i++
			}
			for i < len(input) && isFormulaDigit(input[i]) {
				i++
			}
			literal := input[start:i]
			if hasFormulaDigit(literal) {
				tokens = append(tokens, formulaToken{typ: formulaTokenCell, literal: strings.ToUpper(literal)})
			} else {
				tokens = append(tokens, formulaToken{typ: formulaTokenIdent, literal: strings.ToUpper(literal)})
			}
		case ch == '+':
			tokens = append(tokens, formulaToken{typ: formulaTokenPlus, literal: string(ch)})
			i++
		case ch == '-':
			tokens = append(tokens, formulaToken{typ: formulaTokenMinus, literal: string(ch)})
			i++
		case ch == '*':
			tokens = append(tokens, formulaToken{typ: formulaTokenStar, literal: string(ch)})
			i++
		case ch == '/':
			tokens = append(tokens, formulaToken{typ: formulaTokenSlash, literal: string(ch)})
			i++
		case ch == '(':
			tokens = append(tokens, formulaToken{typ: formulaTokenLParen, literal: string(ch)})
			i++
		case ch == ')':
			tokens = append(tokens, formulaToken{typ: formulaTokenRParen, literal: string(ch)})
			i++
		case ch == ',' || ch == ';':
			tokens = append(tokens, formulaToken{typ: formulaTokenComma, literal: string(ch)})
			i++
		case ch == ':':
			tokens = append(tokens, formulaToken{typ: formulaTokenColon, literal: string(ch)})
			i++
		default:
			return nil, fmt.Errorf("invalid token %q", string(ch))
		}
	}
	tokens = append(tokens, formulaToken{typ: formulaTokenEOF})
	return tokens, nil
}

func lexFormulaNumber(input string, start int) (formulaToken, int, error) {
	i := start
	hasDigit := false
	for i < len(input) && isFormulaDigit(input[i]) {
		i++
		hasDigit = true
	}
	if i < len(input) && input[i] == '.' {
		i++
		for i < len(input) && isFormulaDigit(input[i]) {
			i++
			hasDigit = true
		}
	}
	if !hasDigit {
		return formulaToken{}, start, fmt.Errorf("invalid number")
	}
	if i < len(input) && (input[i] == 'e' || input[i] == 'E') {
		expStart := i
		i++
		if i < len(input) && (input[i] == '+' || input[i] == '-') {
			i++
		}
		expDigits := 0
		for i < len(input) && isFormulaDigit(input[i]) {
			i++
			expDigits++
		}
		if expDigits == 0 {
			return formulaToken{}, start, fmt.Errorf("invalid exponent %q", input[expStart:i])
		}
	}
	literal := input[start:i]
	number, err := strconv.ParseFloat(literal, 64)
	if err != nil || !isFinite(number) {
		return formulaToken{}, start, fmt.Errorf("invalid number %q", literal)
	}
	return formulaToken{typ: formulaTokenNumber, literal: literal, number: number}, i, nil
}

type formulaParser struct {
	tokens   []formulaToken
	pos      int
	sheet    Sheet
	visiting map[string]bool
}

func (p *formulaParser) parseComparison() (formulaResult, error) {
	left, err := p.parseExpression()
	if err != nil {
		return formulaResult{}, err
	}
	for p.match(formulaTokenCmp) {
		op := p.previous().literal
		right, err := p.parseExpression()
		if err != nil {
			return formulaResult{}, err
		}
		leftNum, leftIsNum := left.toNumber()
		rightNum, rightIsNum := right.toNumber()
		var cmp bool
		if leftIsNum && rightIsNum {
			switch op {
			case "=":
				cmp = leftNum == rightNum
			case "<>":
				cmp = leftNum != rightNum
			case "<":
				cmp = leftNum < rightNum
			case ">":
				cmp = leftNum > rightNum
			case "<=":
				cmp = leftNum <= rightNum
			case ">=":
				cmp = leftNum >= rightNum
			}
		} else {
			ls := left.String()
			rs := right.String()
			switch op {
			case "=":
				cmp = ls == rs
			case "<>":
				cmp = ls != rs
			case "<":
				cmp = ls < rs
			case ">":
				cmp = ls > rs
			case "<=":
				cmp = ls <= rs
			case ">=":
				cmp = ls >= rs
			}
		}
		if cmp {
			left = numericResult(1)
		} else {
			left = numericResult(0)
		}
	}
	return left, nil
}

func (p *formulaParser) parseExpression() (formulaResult, error) {
	left, err := p.parseTerm()
	if err != nil {
		return formulaResult{}, err
	}
	for p.match(formulaTokenPlus) || p.match(formulaTokenMinus) {
		op := p.previous()
		right, err := p.parseTerm()
		if err != nil {
			return formulaResult{}, err
		}
		if op.typ == formulaTokenPlus {
			ln, lok := left.toNumber()
			rn, rok := right.toNumber()
			if lok && rok {
				left = numericResult(ln + rn)
			} else {
				left = textResult(left.String() + right.String())
			}
		} else {
			ln, lok := left.toNumber()
			rn, rok := right.toNumber()
			if lok && rok {
				left = numericResult(ln - rn)
			} else {
				return formulaResult{}, fmt.Errorf("cannot subtract text values")
			}
		}
	}
	return left, nil
}

func (p *formulaParser) parseTerm() (formulaResult, error) {
	left, err := p.parseUnary()
	if err != nil {
		return formulaResult{}, err
	}
	for p.match(formulaTokenStar) || p.match(formulaTokenSlash) {
		op := p.previous()
		right, err := p.parseUnary()
		if err != nil {
			return formulaResult{}, err
		}
		ln, lok := left.toNumber()
		rn, rok := right.toNumber()
		if !lok || !rok {
			return formulaResult{}, fmt.Errorf("cannot perform arithmetic on text values")
		}
		if op.typ == formulaTokenStar {
			left = numericResult(ln * rn)
		} else {
			if rn == 0 {
				return formulaResult{}, fmt.Errorf("division by zero")
			}
			left = numericResult(ln / rn)
		}
	}
	return left, nil
}

func (p *formulaParser) parseUnary() (formulaResult, error) {
	if p.match(formulaTokenPlus) {
		return p.parseUnary()
	}
	if p.match(formulaTokenMinus) {
		value, err := p.parseUnary()
		if err != nil {
			return formulaResult{}, err
		}
		n, ok := value.toNumber()
		if !ok {
			return formulaResult{}, fmt.Errorf("cannot negate text value")
		}
		return numericResult(-n), nil
	}
	return p.parsePrimary()
}

func (p *formulaParser) parsePrimary() (formulaResult, error) {
	if p.match(formulaTokenNumber) {
		return numericResult(p.previous().number), nil
	}
	if p.match(formulaTokenString) {
		return textResult(p.previous().literal), nil
	}
	if p.match(formulaTokenCell) {
		start := p.previous().literal
		if p.match(formulaTokenColon) {
			if !p.match(formulaTokenCell) {
				return formulaResult{}, fmt.Errorf("malformed range")
			}
			return p.rangeValue(start, p.previous().literal)
		}
		return p.cellValue(start)
	}
	if p.match(formulaTokenIdent) {
		name := p.previous().literal
		if !p.match(formulaTokenLParen) {
			return formulaResult{}, fmt.Errorf("unexpected identifier %q", name)
		}
		if !isSupportedFormulaFunction(name) {
			return formulaResult{}, fmt.Errorf("unknown function %q", name)
		}
		args := []formulaResult{}
		if !p.check(formulaTokenRParen) {
			for {
				arg, err := p.parseComparison()
				if err != nil {
					return formulaResult{}, err
				}
				args = append(args, arg)
				if !p.match(formulaTokenComma) {
					break
				}
			}
		}
		if !p.match(formulaTokenRParen) {
			return formulaResult{}, fmt.Errorf("missing closing parenthesis")
		}
		return evaluateFormulaFunction(name, args)
	}
	if p.match(formulaTokenLParen) {
		value, err := p.parseComparison()
		if err != nil {
			return formulaResult{}, err
		}
		if !p.match(formulaTokenRParen) {
			return formulaResult{}, fmt.Errorf("missing closing parenthesis")
		}
		return value, nil
	}
	return formulaResult{}, fmt.Errorf("invalid token %q", p.peek().literal)
}

func (p *formulaParser) cellValue(ref string) (formulaResult, error) {
	return p.cellResult(ref)
}

func (p *formulaParser) rangeValue(startRef, endRef string) (formulaResult, error) {
	elements, err := p.collectRange(startRef, endRef)
	if err != nil {
		return formulaResult{}, err
	}
	return formulaResult{isRange: true, elements: elements}, nil
}

func (p *formulaParser) collectRange(startRef, endRef string) ([]formulaResult, error) {
	startCol, startRow, err := parseFormulaCellRef(startRef)
	if err != nil {
		return nil, err
	}
	endCol, endRow, err := parseFormulaCellRef(endRef)
	if err != nil {
		return nil, err
	}
	if endCol < startCol || endRow < startRow {
		return nil, fmt.Errorf("malformed range %s:%s", startRef, endRef)
	}
	rowCount := endRow - startRow + 1
	colCount := endCol - startCol + 1
	if colCount > 0 && rowCount > maxFormulaRangeCells/colCount {
		return nil, fmt.Errorf("range %s:%s exceeds %d cells", startRef, endRef, maxFormulaRangeCells)
	}
	elements := make([]formulaResult, 0, rowCount*colCount)
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			ref := formulaCellName(col, row)
			result, err := p.cellResult(ref)
			if err != nil {
				return nil, err
			}
			elements = append(elements, result)
		}
	}
	return elements, nil
}

func (p *formulaParser) rangeValueArray(startRef, endRef string) ([][]formulaResult, error) {
	startCol, startRow, err := parseFormulaCellRef(startRef)
	if err != nil {
		return nil, err
	}
	endCol, endRow, err := parseFormulaCellRef(endRef)
	if err != nil {
		return nil, err
	}
	if endCol < startCol || endRow < startRow {
		return nil, nil
	}
	var rows [][]formulaResult
	for row := startRow; row <= endRow; row++ {
		r := make([]formulaResult, 0, endCol-startCol+1)
		for col := startCol; col <= endCol; col++ {
			ref := formulaCellName(col, row)
			result, err := p.cellResult(ref)
			if err != nil {
				return nil, err
			}
			r = append(r, result)
		}
		rows = append(rows, r)
	}
	return rows, nil
}

func (p *formulaParser) cellResult(ref string) (formulaResult, error) {
	col, row, err := parseFormulaCellRef(ref)
	if err != nil {
		return formulaResult{}, err
	}
	if row <= 0 || row > len(p.sheet.Rows) {
		return numericResult(0), nil
	}
	rowData := p.sheet.Rows[row-1]
	if col <= 0 || col > len(rowData) {
		return numericResult(0), nil
	}
	cell := rowData[col-1]
	if strings.TrimSpace(cell.Formula) != "" {
		key := strings.ToUpper(ref)
		if p.visiting[key] {
			return formulaResult{}, fmt.Errorf("circular formula reference at %s", key)
		}
		p.visiting[key] = true
		result, err := evaluateFormula(p.sheet, cell.Formula, p.visiting)
		delete(p.visiting, key)
		if err != nil {
			return formulaResult{}, fmt.Errorf("evaluate %s: %w", key, err)
		}
		return result, nil
	}
	raw := strings.TrimSpace(cell.Value)
	if raw == "" {
		return emptyResult(), nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || !isFinite(value) {
		return textResult(raw), nil
	}
	return numericResult(value), nil
}

func evaluateFormulaFunction(name string, args []formulaResult) (formulaResult, error) {
	flat := flattenFormulaArgs(args)
	switch name {
	case "SUM":
		return fnSum(flat), nil
	case "AVG", "AVERAGE":
		return fnAverage(flat)
	case "COUNT":
		return fnCount(flat), nil
	case "COUNTA":
		return fnCountA(flat), nil
	case "MIN":
		return fnMin(flat)
	case "MAX":
		return fnMax(flat)
	case "ABS":
		return fnAbs(args)
	case "ROUND":
		return fnRound(args)
	case "CEIL", "CEILING":
		return fnCeiling(args), nil
	case "FLOOR":
		return fnFloor(args), nil
	case "MEDIAN":
		return fnMedian(flat)
	case "STDEV":
		return fnStdev(flat)
	case "IF":
		return fnIf(args)
	case "AND":
		return fnAnd(flat), nil
	case "OR":
		return fnOr(flat), nil
	case "NOT":
		return fnNot(args), nil
	case "CONCAT", "CONCATENATE":
		return fnConcat(flat), nil
	case "TEXTJOIN":
		return fnTextJoin(args), nil
	case "LEFT":
		return fnLeft(args), nil
	case "RIGHT":
		return fnRight(args), nil
	case "MID":
		return fnMid(args), nil
	case "LEN":
		return fnLen(args), nil
	case "UPPER":
		return fnUpper(args), nil
	case "LOWER":
		return fnLower(args), nil
	case "TRIM":
		return fnTrim(args), nil
	case "NOW":
		return numericResult(float64(time.Now().UnixMilli())), nil
	case "TODAY":
		now := time.Now()
		d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return numericResult(float64(d.UnixMilli())), nil
	case "DATE":
		return fnDate(args), nil
	case "YEAR":
		return fnYear(args), nil
	case "MONTH":
		return fnMonth(args), nil
	case "DAY":
		return fnDay(args), nil
	case "VLOOKUP":
		return fnVlookup(args)
	case "HLOOKUP":
		return fnHlookup(args)
	default:
		return formulaResult{}, fmt.Errorf("unknown function %q", name)
	}
}

func flattenFormulaArgs(args []formulaResult) []formulaResult {
	var flat []formulaResult
	for _, arg := range args {
		if arg.isRange {
			flat = append(flat, arg.elements...)
		} else {
			flat = append(flat, arg)
		}
	}
	return flat
}

func numericArgs(args []formulaResult) []float64 {
	var nums []float64
	for _, a := range args {
		if a.isEmpty || a.isText {
			continue
		}
		if n, ok := a.toNumber(); ok {
			nums = append(nums, n)
		}
	}
	return nums
}

func fnSum(args []formulaResult) formulaResult {
	sum := 0.0
	for _, a := range args {
		if a.isEmpty || a.isText {
			continue
		}
		if n, ok := a.toNumber(); ok {
			sum += n
		}
	}
	return numericResult(sum)
}

func fnAverage(args []formulaResult) (formulaResult, error) {
	sum := 0.0
	count := 0
	for _, a := range args {
		if a.isEmpty || a.isText {
			continue
		}
		if n, ok := a.toNumber(); ok {
			sum += n
			count++
		}
	}
	if count == 0 {
		return formulaResult{}, fmt.Errorf("AVERAGE requires at least one numeric value")
	}
	return numericResult(sum / float64(count)), nil
}

func fnCount(args []formulaResult) formulaResult {
	count := 0
	for _, a := range args {
		if a.isEmpty || a.isText {
			continue
		}
		if _, ok := a.toNumber(); ok {
			count++
		}
	}
	return numericResult(float64(count))
}

func fnCountA(args []formulaResult) formulaResult {
	count := 0
	for _, a := range args {
		if a.isEmpty {
			continue
		}
		if a.isText {
			if strings.TrimSpace(a.text) != "" {
				count++
			}
		} else {
			count++
		}
	}
	return numericResult(float64(count))
}

func fnMin(args []formulaResult) (formulaResult, error) {
	nums := numericArgs(args)
	if len(nums) == 0 {
		return formulaResult{}, fmt.Errorf("MIN requires at least one numeric value")
	}
	min := nums[0]
	for _, n := range nums[1:] {
		if n < min {
			min = n
		}
	}
	return numericResult(min), nil
}

func fnMax(args []formulaResult) (formulaResult, error) {
	nums := numericArgs(args)
	if len(nums) == 0 {
		return formulaResult{}, fmt.Errorf("MAX requires at least one numeric value")
	}
	max := nums[0]
	for _, n := range nums[1:] {
		if n > max {
			max = n
		}
	}
	return numericResult(max), nil
}

func fnAbs(args []formulaResult) (formulaResult, error) {
	if len(args) == 0 {
		return formulaResult{}, fmt.Errorf("ABS requires one argument")
	}
	n, ok := args[0].toNumber()
	if !ok {
		return numericResult(0), nil
	}
	return numericResult(math.Abs(n)), nil
}

func fnRound(args []formulaResult) (formulaResult, error) {
	if len(args) == 0 {
		return formulaResult{}, fmt.Errorf("ROUND requires at least one argument")
	}
	n, ok := args[0].toNumber()
	if !ok {
		return numericResult(0), nil
	}
	places := 0
	if len(args) > 1 {
		if p, ok := args[1].toNumber(); ok {
			places = int(p)
		}
	}
	if places < 0 {
		places = 0
	}
	factor := math.Pow(10, float64(places))
	return numericResult(math.Round(n*factor) / factor), nil
}

func fnCeiling(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return numericResult(0)
	}
	n, ok := args[0].toNumber()
	if !ok {
		return numericResult(0)
	}
	return numericResult(math.Ceil(n))
}

func fnFloor(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return numericResult(0)
	}
	n, ok := args[0].toNumber()
	if !ok {
		return numericResult(0)
	}
	return numericResult(math.Floor(n))
}

func fnMedian(args []formulaResult) (formulaResult, error) {
	nums := numericArgs(args)
	if len(nums) == 0 {
		return formulaResult{}, fmt.Errorf("MEDIAN requires at least one numeric value")
	}
	sorted := make([]float64, len(nums))
	copy(sorted, nums)
	sortFloat64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 != 0 {
		return numericResult(sorted[mid]), nil
	}
	return numericResult((sorted[mid-1] + sorted[mid]) / 2), nil
}

func fnStdev(args []formulaResult) (formulaResult, error) {
	nums := numericArgs(args)
	if len(nums) < 2 {
		return numericResult(0), nil
	}
	mean := 0.0
	for _, n := range nums {
		mean += n
	}
	mean /= float64(len(nums))
	variance := 0.0
	for _, n := range nums {
		diff := n - mean
		variance += diff * diff
	}
	variance /= float64(len(nums) - 1)
	return numericResult(math.Sqrt(variance)), nil
}

func fnIf(args []formulaResult) (formulaResult, error) {
	if len(args) == 0 {
		return formulaResult{}, fmt.Errorf("IF requires at least one argument")
	}
	cond := false
	if args[0].isText {
		cond = strings.TrimSpace(args[0].text) != ""
	} else {
		cond = args[0].number != 0
	}
	trueVal := numericResult(1)
	falseVal := numericResult(0)
	if len(args) > 1 {
		trueVal = args[1]
	}
	if len(args) > 2 {
		falseVal = args[2]
	}
	if cond {
		return trueVal, nil
	}
	return falseVal, nil
}

func fnAnd(args []formulaResult) formulaResult {
	for _, a := range args {
		if a.isText {
			if strings.TrimSpace(a.text) == "" {
				return numericResult(0)
			}
		} else if a.number == 0 {
			return numericResult(0)
		}
	}
	return numericResult(1)
}

func fnOr(args []formulaResult) formulaResult {
	for _, a := range args {
		if a.isText {
			if strings.TrimSpace(a.text) != "" {
				return numericResult(1)
			}
		} else if a.number != 0 {
			return numericResult(1)
		}
	}
	return numericResult(0)
}

func fnNot(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return numericResult(1)
	}
	if args[0].isText {
		if strings.TrimSpace(args[0].text) == "" {
			return numericResult(1)
		}
		return numericResult(0)
	}
	if args[0].number == 0 {
		return numericResult(1)
	}
	return numericResult(0)
}

func fnConcat(args []formulaResult) formulaResult {
	var sb strings.Builder
	for _, a := range args {
		sb.WriteString(a.String())
	}
	return textResult(sb.String())
}

func fnTextJoin(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return textResult("")
	}
	delim := args[0].String()
	var parts []string
	for _, a := range args[1:] {
		parts = append(parts, a.String())
	}
	return textResult(strings.Join(parts, delim))
}

func fnLeft(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return textResult("")
	}
	s := args[0].String()
	n := 1
	if len(args) > 1 {
		if v, ok := args[1].toNumber(); ok {
			n = int(v)
		}
	}
	if n < 0 {
		n = 0
	}
	if n > len(s) {
		n = len(s)
	}
	return textResult(s[:n])
}

func fnRight(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return textResult("")
	}
	s := args[0].String()
	n := 1
	if len(args) > 1 {
		if v, ok := args[1].toNumber(); ok {
			n = int(v)
		}
	}
	if n < 0 {
		n = 0
	}
	if n > len(s) {
		n = len(s)
	}
	return textResult(s[len(s)-n:])
}

func fnMid(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return textResult("")
	}
	s := args[0].String()
	start := 1
	length := 1
	if len(args) > 1 {
		if v, ok := args[1].toNumber(); ok {
			start = int(v)
		}
	}
	if len(args) > 2 {
		if v, ok := args[2].toNumber(); ok {
			length = int(v)
		}
	}
	start--
	if start < 0 {
		start = 0
	}
	if start > len(s) {
		return textResult("")
	}
	end := start + length
	if end > len(s) {
		end = len(s)
	}
	return textResult(s[start:end])
}

func fnLen(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return numericResult(0)
	}
	return numericResult(float64(len(args[0].String())))
}

func fnUpper(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return textResult("")
	}
	return textResult(strings.ToUpper(args[0].String()))
}

func fnLower(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return textResult("")
	}
	return textResult(strings.ToLower(args[0].String()))
}

func fnTrim(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return textResult("")
	}
	return textResult(strings.TrimSpace(args[0].String()))
}

func fnDate(args []formulaResult) formulaResult {
	year := 1970
	month := 1
	day := 1
	if len(args) > 0 {
		if v, ok := args[0].toNumber(); ok {
			year = int(v)
		}
	}
	if len(args) > 1 {
		if v, ok := args[1].toNumber(); ok {
			month = int(v)
		}
	}
	if len(args) > 2 {
		if v, ok := args[2].toNumber(); ok {
			day = int(v)
		}
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return numericResult(float64(t.UnixMilli()))
}

func fnYear(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return numericResult(0)
	}
	n, ok := args[0].toNumber()
	if !ok {
		return numericResult(0)
	}
	t := time.UnixMilli(int64(n))
	return numericResult(float64(t.Year()))
}

func fnMonth(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return numericResult(0)
	}
	n, ok := args[0].toNumber()
	if !ok {
		return numericResult(0)
	}
	t := time.UnixMilli(int64(n))
	return numericResult(float64(t.Month()))
}

func fnDay(args []formulaResult) formulaResult {
	if len(args) == 0 {
		return numericResult(0)
	}
	n, ok := args[0].toNumber()
	if !ok {
		return numericResult(0)
	}
	t := time.UnixMilli(int64(n))
	return numericResult(float64(t.Day()))
}

func fnVlookup(args []formulaResult) (formulaResult, error) {
	if len(args) < 3 {
		return formulaResult{}, fmt.Errorf("VLOOKUP requires at least 3 arguments")
	}
	_ = args[0]
	exact := true
	if len(args) > 3 {
		if n, ok := args[3].toNumber(); ok {
			exact = n != 0
		}
	}
	colIdx := 1
	if n, ok := args[2].toNumber(); ok {
		colIdx = int(n)
	}
	_ = exact
	_ = colIdx
	return textResult("#N/A"), nil
}

func fnHlookup(args []formulaResult) (formulaResult, error) {
	if len(args) < 3 {
		return formulaResult{}, fmt.Errorf("HLOOKUP requires at least 3 arguments")
	}
	return textResult("#N/A"), nil
}

func isSupportedFormulaFunction(name string) bool {
	switch name {
	case "SUM", "AVG", "AVERAGE", "MIN", "MAX", "COUNT", "COUNTA",
		"ABS", "ROUND", "CEIL", "CEILING", "FLOOR", "MEDIAN", "STDEV",
		"IF", "AND", "OR", "NOT",
		"CONCAT", "CONCATENATE", "TEXTJOIN",
		"LEFT", "RIGHT", "MID", "LEN", "UPPER", "LOWER", "TRIM",
		"NOW", "TODAY", "DATE", "YEAR", "MONTH", "DAY",
		"VLOOKUP", "HLOOKUP":
		return true
	default:
		return false
	}
}

func parseFormulaCellRef(ref string) (int, int, error) {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	i := 0
	for i < len(ref) && isFormulaLetter(ref[i]) {
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, fmt.Errorf("invalid cell reference %q", ref)
	}
	colName := ref[:i]
	rowName := ref[i:]
	for j := 0; j < len(rowName); j++ {
		if !isFormulaDigit(rowName[j]) {
			return 0, 0, fmt.Errorf("invalid cell reference %q", ref)
		}
	}
	if rowName[0] == '0' {
		return 0, 0, fmt.Errorf("invalid cell reference %q", ref)
	}
	col := 0
	for j := 0; j < len(colName); j++ {
		col = col*26 + int(colName[j]-'A'+1)
		if col > maxFormulaColumn {
			return 0, 0, fmt.Errorf("cell reference %q exceeds max column XFD", ref)
		}
	}
	row64, err := strconv.ParseInt(rowName, 10, 32)
	if err != nil || row64 <= 0 {
		return 0, 0, fmt.Errorf("invalid cell reference %q", ref)
	}
	row := int(row64)
	if row > maxFormulaRow {
		return 0, 0, fmt.Errorf("cell reference %q exceeds max row %d", ref, maxFormulaRow)
	}
	return col, row, nil
}

func formulaCellName(col, row int) string {
	var name []byte
	for col > 0 {
		col--
		name = append([]byte{byte('A' + col%26)}, name...)
		col /= 26
	}
	return string(name) + strconv.Itoa(row)
}

func (p *formulaParser) match(types ...formulaTokenType) bool {
	for _, typ := range types {
		if p.check(typ) {
			p.pos++
			return true
		}
	}
	return false
}

func (p *formulaParser) check(typ formulaTokenType) bool {
	return p.peek().typ == typ
}

func (p *formulaParser) peek() formulaToken {
	if p.pos >= len(p.tokens) {
		return formulaToken{typ: formulaTokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *formulaParser) previous() formulaToken {
	if p.pos == 0 {
		return formulaToken{typ: formulaTokenEOF}
	}
	return p.tokens[p.pos-1]
}

func isFormulaLetter(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isFormulaDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func hasFormulaDigit(value string) bool {
	for i := 0; i < len(value); i++ {
		if isFormulaDigit(value[i]) {
			return true
		}
	}
	return false
}

func isFinite(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}

func sortFloat64s(data []float64) {
	quickSortFloat64(data, 0, len(data)-1)
}

func quickSortFloat64(data []float64, lo, hi int) {
	if lo >= hi {
		return
	}
	pivot := data[hi]
	i := lo
	for j := lo; j < hi; j++ {
		if data[j] < pivot {
			data[i], data[j] = data[j], data[i]
			i++
		}
	}
	data[i], data[hi] = data[hi], data[i]
	quickSortFloat64(data, lo, i-1)
	quickSortFloat64(data, i+1, hi)
}
