package office

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	maxFormulaBytes      = 4096
	maxFormulaRangeCells = 100000
)

// EvaluateFormulaForSheet validates and evaluates AuraGo's safe spreadsheet formula subset.
func EvaluateFormulaForSheet(sheet Sheet, formula string) (string, error) {
	value, err := evaluateFormulaNumber(sheet, formula, map[string]bool{})
	if err != nil {
		return "", err
	}
	if !isFinite(value) {
		return "", fmt.Errorf("formula result is not finite")
	}
	if value == 0 {
		return "0", nil
	}
	return strconv.FormatFloat(value, 'f', -1, 64), nil
}

func evaluateFormulaNumber(sheet Sheet, formula string, visiting map[string]bool) (float64, error) {
	formula = strings.TrimSpace(formula)
	if len(formula) > maxFormulaBytes {
		return 0, fmt.Errorf("formula exceeds %d bytes", maxFormulaBytes)
	}
	formula = strings.TrimPrefix(formula, "=")
	if formula == "" {
		return 0, fmt.Errorf("formula is empty")
	}
	if strings.ContainsAny(formula, "\"'") {
		return 0, fmt.Errorf("strings are not supported in formulas")
	}
	if strings.ContainsAny(formula, "![]") {
		return 0, fmt.Errorf("external and sheet references are not supported")
	}
	tokens, err := lexFormula(formula)
	if err != nil {
		return 0, err
	}
	parser := formulaParser{
		tokens:   tokens,
		sheet:    sheet,
		visiting: visiting,
	}
	value, err := parser.parseExpression()
	if err != nil {
		return 0, err
	}
	if parser.peek().typ != formulaTokenEOF {
		return 0, fmt.Errorf("invalid token %q", parser.peek().literal)
	}
	scalar, err := value.scalarForArithmetic()
	if err != nil {
		return 0, err
	}
	if !isFinite(scalar) {
		return 0, fmt.Errorf("formula result is not finite")
	}
	return scalar, nil
}

type formulaTokenType int

const (
	formulaTokenEOF formulaTokenType = iota
	formulaTokenNumber
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

type formulaValue struct {
	isRange  bool
	scalar   formulaElement
	elements []formulaElement
}

type formulaElement struct {
	value   float64
	numeric bool
}

func (p *formulaParser) parseExpression() (formulaValue, error) {
	left, err := p.parseTerm()
	if err != nil {
		return formulaValue{}, err
	}
	for p.match(formulaTokenPlus) || p.match(formulaTokenMinus) {
		op := p.previous()
		right, err := p.parseTerm()
		if err != nil {
			return formulaValue{}, err
		}
		leftScalar, err := left.scalarForArithmetic()
		if err != nil {
			return formulaValue{}, err
		}
		rightScalar, err := right.scalarForArithmetic()
		if err != nil {
			return formulaValue{}, err
		}
		if op.typ == formulaTokenPlus {
			left = numericFormulaValue(leftScalar + rightScalar)
		} else {
			left = numericFormulaValue(leftScalar - rightScalar)
		}
	}
	return left, nil
}

func (p *formulaParser) parseTerm() (formulaValue, error) {
	left, err := p.parseUnary()
	if err != nil {
		return formulaValue{}, err
	}
	for p.match(formulaTokenStar) || p.match(formulaTokenSlash) {
		op := p.previous()
		right, err := p.parseUnary()
		if err != nil {
			return formulaValue{}, err
		}
		leftScalar, err := left.scalarForArithmetic()
		if err != nil {
			return formulaValue{}, err
		}
		rightScalar, err := right.scalarForArithmetic()
		if err != nil {
			return formulaValue{}, err
		}
		if op.typ == formulaTokenStar {
			left = numericFormulaValue(leftScalar * rightScalar)
		} else {
			left = numericFormulaValue(leftScalar / rightScalar)
		}
	}
	return left, nil
}

func (p *formulaParser) parseUnary() (formulaValue, error) {
	if p.match(formulaTokenPlus) {
		return p.parseUnary()
	}
	if p.match(formulaTokenMinus) {
		value, err := p.parseUnary()
		if err != nil {
			return formulaValue{}, err
		}
		scalar, err := value.scalarForArithmetic()
		if err != nil {
			return formulaValue{}, err
		}
		return numericFormulaValue(-scalar), nil
	}
	return p.parsePrimary()
}

func (p *formulaParser) parsePrimary() (formulaValue, error) {
	if p.match(formulaTokenNumber) {
		return numericFormulaValue(p.previous().number), nil
	}
	if p.match(formulaTokenCell) {
		start := p.previous().literal
		if p.match(formulaTokenColon) {
			if !p.match(formulaTokenCell) {
				return formulaValue{}, fmt.Errorf("malformed range")
			}
			return p.rangeValue(start, p.previous().literal)
		}
		return p.cellValue(start)
	}
	if p.match(formulaTokenIdent) {
		name := p.previous().literal
		if !p.match(formulaTokenLParen) {
			return formulaValue{}, fmt.Errorf("unexpected identifier %q", name)
		}
		if !isSupportedFormulaFunction(name) {
			return formulaValue{}, fmt.Errorf("unknown function %q", name)
		}
		args := []formulaValue{}
		if !p.check(formulaTokenRParen) {
			for {
				arg, err := p.parseExpression()
				if err != nil {
					return formulaValue{}, err
				}
				args = append(args, arg)
				if !p.match(formulaTokenComma) {
					break
				}
			}
		}
		if !p.match(formulaTokenRParen) {
			return formulaValue{}, fmt.Errorf("missing closing parenthesis")
		}
		return evaluateFormulaFunction(name, args)
	}
	if p.match(formulaTokenLParen) {
		value, err := p.parseExpression()
		if err != nil {
			return formulaValue{}, err
		}
		if !p.match(formulaTokenRParen) {
			return formulaValue{}, fmt.Errorf("missing closing parenthesis")
		}
		return value, nil
	}
	return formulaValue{}, fmt.Errorf("invalid token %q", p.peek().literal)
}

func (p *formulaParser) cellValue(ref string) (formulaValue, error) {
	element, err := p.cellElement(ref)
	if err != nil {
		return formulaValue{}, err
	}
	return formulaValue{scalar: element}, nil
}

func (p *formulaParser) rangeValue(startRef, endRef string) (formulaValue, error) {
	startCol, startRow, err := parseFormulaCellRef(startRef)
	if err != nil {
		return formulaValue{}, err
	}
	endCol, endRow, err := parseFormulaCellRef(endRef)
	if err != nil {
		return formulaValue{}, err
	}
	if endCol < startCol || endRow < startRow {
		return formulaValue{}, fmt.Errorf("malformed range %s:%s", startRef, endRef)
	}
	rowCount := endRow - startRow + 1
	colCount := endCol - startCol + 1
	if colCount > 0 && rowCount > maxFormulaRangeCells/colCount {
		return formulaValue{}, fmt.Errorf("range %s:%s exceeds %d cells", startRef, endRef, maxFormulaRangeCells)
	}
	elements := []formulaElement{}
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			ref := formulaCellName(col, row)
			element, err := p.cellElement(ref)
			if err != nil {
				return formulaValue{}, err
			}
			elements = append(elements, element)
		}
	}
	return formulaValue{isRange: true, elements: elements}, nil
}

func (p *formulaParser) cellElement(ref string) (formulaElement, error) {
	col, row, err := parseFormulaCellRef(ref)
	if err != nil {
		return formulaElement{}, err
	}
	if row > len(p.sheet.Rows) || col > len(p.sheet.Rows[row-1]) {
		return formulaElement{}, nil
	}
	cell := p.sheet.Rows[row-1][col-1]
	if strings.TrimSpace(cell.Formula) != "" {
		key := strings.ToUpper(ref)
		if p.visiting[key] {
			return formulaElement{}, fmt.Errorf("circular formula reference at %s", key)
		}
		p.visiting[key] = true
		value, err := evaluateFormulaNumber(p.sheet, cell.Formula, p.visiting)
		delete(p.visiting, key)
		if err != nil {
			return formulaElement{}, fmt.Errorf("evaluate %s: %w", key, err)
		}
		return formulaElement{value: value, numeric: true}, nil
	}
	raw := strings.TrimSpace(cell.Value)
	if raw == "" {
		return formulaElement{}, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || !isFinite(value) {
		return formulaElement{}, fmt.Errorf("cell %s contains non-numeric value", strings.ToUpper(ref))
	}
	return formulaElement{value: value, numeric: true}, nil
}

func evaluateFormulaFunction(name string, args []formulaValue) (formulaValue, error) {
	elements := flattenFormulaArgs(args)
	switch name {
	case "SUM":
		var sum float64
		for _, element := range elements {
			if element.numeric {
				sum += element.value
			}
		}
		return numericFormulaValue(sum), nil
	case "AVG":
		var sum float64
		var count int
		for _, element := range elements {
			if element.numeric {
				sum += element.value
				count++
			}
		}
		if count == 0 {
			return formulaValue{}, fmt.Errorf("AVG requires at least one numeric value")
		}
		return numericFormulaValue(sum / float64(count)), nil
	case "MIN":
		value, ok := minFormulaElement(elements)
		if !ok {
			return formulaValue{}, fmt.Errorf("MIN requires at least one numeric value")
		}
		return numericFormulaValue(value), nil
	case "MAX":
		value, ok := maxFormulaElement(elements)
		if !ok {
			return formulaValue{}, fmt.Errorf("MAX requires at least one numeric value")
		}
		return numericFormulaValue(value), nil
	case "COUNT":
		var count int
		for _, element := range elements {
			if element.numeric {
				count++
			}
		}
		return numericFormulaValue(float64(count)), nil
	default:
		return formulaValue{}, fmt.Errorf("unknown function %q", name)
	}
}

func flattenFormulaArgs(args []formulaValue) []formulaElement {
	elements := []formulaElement{}
	for _, arg := range args {
		if arg.isRange {
			elements = append(elements, arg.elements...)
		} else {
			elements = append(elements, arg.scalar)
		}
	}
	return elements
}

func minFormulaElement(elements []formulaElement) (float64, bool) {
	var min float64
	var ok bool
	for _, element := range elements {
		if !element.numeric {
			continue
		}
		if !ok || element.value < min {
			min = element.value
			ok = true
		}
	}
	return min, ok
}

func maxFormulaElement(elements []formulaElement) (float64, bool) {
	var max float64
	var ok bool
	for _, element := range elements {
		if !element.numeric {
			continue
		}
		if !ok || element.value > max {
			max = element.value
			ok = true
		}
	}
	return max, ok
}

func (v formulaValue) scalarForArithmetic() (float64, error) {
	if v.isRange {
		return 0, fmt.Errorf("ranges are only supported as function arguments")
	}
	return v.scalar.value, nil
}

func numericFormulaValue(value float64) formulaValue {
	return formulaValue{scalar: formulaElement{value: value, numeric: true}}
}

func isSupportedFormulaFunction(name string) bool {
	switch name {
	case "SUM", "AVG", "MIN", "MAX", "COUNT":
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
	if rowName[0] == '0' {
		return 0, 0, fmt.Errorf("invalid cell reference %q", ref)
	}
	col := 0
	for j := 0; j < len(colName); j++ {
		col = col*26 + int(colName[j]-'A'+1)
	}
	row, err := strconv.Atoi(rowName)
	if err != nil || row <= 0 {
		return 0, 0, fmt.Errorf("invalid cell reference %q", ref)
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
