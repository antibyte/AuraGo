package ecalc

import (
	"strings"
)

// The query API of the component codes: DescribeCode describes a code item, QueryDecode and QueryEncode wrap Decode and
// Encode (codes.go) for the agent tool. They resolve the code kind, turn the errors of the codes into QueryErrors, and
// write a result without the empty parts (a NaN value, empty lists) and with the value formatted. The lengths of the
// texts are limited by Decode and Encode (64 characters each). Output texts are English.

// CodeDescription is what DescribeCode returns. CodeKind is the kind that decode and encode take as their kind; Decode
// lists the forms a code may be written in, Encode the fields of an encode request and Fields what a result carries
// besides the code and the value.
type CodeDescription struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Category string   `json:"category"`
	Name     string   `json:"name"`
	Doc      string   `json:"doc,omitempty"`
	Source   string   `json:"source"`
	CodeKind string   `json:"code_kind"`
	Decode   []string `json:"decode"`
	Encode   []string `json:"encode"`
	Fields   []string `json:"fields"`
}

// codeForms is what is known about the input and output of a code kind without loading its data.
type codeForms struct {
	decode, encode, fields []string
}

// codeFormsByKind describes the five code kinds of codes.go; the keys are the kind ids.
var codeFormsByKind = map[string]codeForms{
	"resistor_bands": {
		decode: []string{"1 to 6 band colours as English names, for example blue grey red (3 bands), brown black red gold (4), red yellow white orange brown (5)"},
		encode: []string{"value (required, for example 4k7)", "tolerance (for example 5%; it fixes the series)", "tempco (6 bands, for example 50ppm)", "bands (3, 4, 5 or 6)"},
		fields: []string{"tolerance", "tcr (temperature coefficient)"},
	},
	"inductor_bands": {
		decode: []string{"4 band colours (EIA) or 5 (MIL, silver first), for example silver blue gold grey silver"},
		encode: []string{"value (required, for example 6.8u)", "tolerance (5% or 10%)", "bands (4 for EIA, 5 for MIL)"},
		fields: []string{"layout", "tolerance"},
	},
	"smd_resistor": {
		decode: []string{"a marking as printed: 472, 4R7, 01C (EIA-96), 1L5, 4K7, 000"},
		encode: []string{"value (required, for example 10k)", "tolerance (for example 1%; it chooses the 3-digit, 4-digit or EIA-96 marking)"},
		fields: []string{"family", "code_3digit, code_4digit, code_eia96 (encode, those that exist)", "tolerance (encode)"},
	},
	"capacitor": {
		decode: []string{"up to three parts as printed: a value (104, 4p7), a tolerance letter (K) and a voltage code (1H), for example 104K 1H"},
		encode: []string{"value (for example 100n)", "tolerance (a letter such as K, or a percentage)", "voltage (for example 50V)"},
		fields: []string{"family", "tolerance (with tolerance_minus and tolerance_plus)", "voltage", "value_code and rkm (encode)"},
	},
	"ceramic_temp": {
		decode: []string{"a ceramic temperature code as printed: X7R, C0G, NP0, U2J"},
		encode: []string{"class (1 or 2, required)", "class 1: tempco (for example -750ppm) and tolerance (for example 120ppm)", "class 2: value as a temperature range (-55..125) and tolerance (for example 15%)"},
		fields: []string{"class 1: class, tc, tolerance, industry_code", "class 2: class, temp_low, temp_high, change"},
	},
}

// genericCodeForms describes a code kind that has no entry of its own.
var genericCodeForms = codeForms{
	decode: []string{"the code as printed or as a list of English colour names"},
	encode: []string{"value (required)"},
	fields: []string{"further decoded fields as listed in the result"},
}

// codeFormsOf returns the forms of a code kind.
func codeFormsOf(kind string) codeForms {
	if f, ok := codeFormsByKind[kind]; ok {
		return f
	}
	return genericCodeForms
}

// lookupCodeItem finds a code item by id (case-insensitive), like lookupTable.
func (c *Catalog) lookupCodeItem(id string) (*Code, error) {
	key := strings.ToLower(strings.TrimSpace(id))
	if key == "" {
		return nil, queryErr(CodeUnknownID, "id is required; use search to find a code id")
	}
	if k := c.Code(key); k != nil {
		return k, nil
	}
	return nil, c.notFound(id, key, "code")
}

// DescribeCode returns what a code item is for: its kind and the forms in which decode takes a code and encode takes a
// value.
func (c *Catalog) DescribeCode(id string) (CodeDescription, error) {
	k, err := c.lookupCodeItem(id)
	if err != nil {
		return CodeDescription{}, err
	}
	f := codeFormsOf(k.CodeKind)
	return CodeDescription{
		ID: k.ID, Kind: "code", Category: k.Category, Name: k.Name.in("en"), Doc: k.Doc.in("en"), Source: k.Source,
		CodeKind: k.CodeKind, Decode: f.decode, Encode: f.encode, Fields: f.fields,
	}, nil
}

// CodeAnswerField is one further field of a decoded or encoded code. Value is absent for a field without a number.
type CodeAnswerField struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Text  string   `json:"text"`
	Value *float64 `json:"value,omitempty"`
	Dim   string   `json:"dim,omitempty"`
}

// CodeAnswer is the answer of QueryDecode and QueryEncode: the CodeResult of the code kind without its empty parts and
// with the value formatted. Kind is the code kind, Code the canonical code text, Bands the colours of a band code. Value
// (SI, in the dimension Dim) and Formatted are absent for a code without a value.
type CodeAnswer struct {
	Kind      string            `json:"kind"`
	Code      string            `json:"code"`
	Bands     []string          `json:"bands,omitempty"`
	Value     *float64          `json:"value,omitempty"`
	Dim       string            `json:"dim,omitempty"`
	Formatted string            `json:"formatted,omitempty"`
	Fields    []CodeAnswerField `json:"fields"`
	Warnings  []string          `json:"warnings,omitempty"`
}

// resolveCodeKind returns the registered code kind a request means: the kind itself or the id of a code item (whose kind
// it is). The error of anything else lists the kinds.
func (c *Catalog) resolveCodeKind(kind string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(kind))
	if _, ok := codeKindOf(key); ok && key != "" {
		return key, nil
	}
	if k := c.Code(key); k != nil {
		return k.CodeKind, nil
	}
	return "", queryErr(CodeUnknownCodeKind, "unknown code kind %q; kinds: %s", kind, strings.Join(CodeKindIDs(), ", "))
}

// codeQueryError turns an error of Decode or Encode into a QueryError that keeps its code (invalid_code, invalid_value
// or unsupported); any other error becomes invalid_code with its text.
func codeQueryError(err error) *QueryError {
	if ce := AsCodeError(err); ce != nil {
		return &QueryError{Code: ce.Code, Message: ce.Message}
	}
	return &QueryError{Code: CodeInvalidCode, Message: err.Error()}
}

func (c *Catalog) codeAnswer(res CodeResult) CodeAnswer {
	out := CodeAnswer{Kind: res.Kind, Code: res.Code, Bands: res.Bands, Dim: res.Dim, Fields: make([]CodeAnswerField, 0, len(res.Fields)), Warnings: res.Warnings}
	if out.Value = finitePtr(res.Value); out.Value != nil && res.Dim != "" {
		unit := ""
		if res.Dim == "tempco" {
			unit = "ppmK" // -750 ppm/K, not -0.00075 1/K
		}
		out.Formatted = c.formatValue(res.Value, res.Dim, unit)
	}
	if out.Value == nil {
		out.Dim = ""
	}
	for _, f := range res.Fields {
		field := CodeAnswerField{ID: f.ID, Name: f.Name, Text: f.Text, Value: finitePtr(f.Value)}
		if field.Value != nil {
			field.Dim = f.Dim
		}
		out.Fields = append(out.Fields, field)
	}
	return out
}

// QueryDecode reads a component code: band colours as English colour names, a marking as printed (103, 4R7, 104K 1H,
// X7R). The kind is a code kind or the id of a code item.
func (c *Catalog) QueryDecode(kind, code string) (CodeAnswer, error) {
	k, err := c.resolveCodeKind(kind)
	if err != nil {
		return CodeAnswer{}, err
	}
	res, err := c.Decode(k, strings.TrimSpace(code))
	if err != nil {
		return CodeAnswer{}, codeQueryError(err)
	}
	return c.codeAnswer(res), nil
}

// QueryEncode writes the code of a value: the nearest standard value of the series that the tolerance implies, as band
// colours or as a marking. The kind is a code kind or the id of a code item. What each kind reads from the request is in
// codeFormsByKind; Encode checks the texts.
func (c *Catalog) QueryEncode(kind string, req EncodeRequest) (CodeAnswer, error) {
	k, err := c.resolveCodeKind(kind)
	if err != nil {
		return CodeAnswer{}, err
	}
	req.Value, req.Tolerance, req.TempCo = strings.TrimSpace(req.Value), strings.TrimSpace(req.Tolerance), strings.TrimSpace(req.TempCo)
	req.Voltage, req.Class = strings.TrimSpace(req.Voltage), strings.TrimSpace(req.Class)
	res, err := c.Encode(k, req)
	if err != nil {
		return CodeAnswer{}, codeQueryError(err)
	}
	return c.codeAnswer(res), nil
}
