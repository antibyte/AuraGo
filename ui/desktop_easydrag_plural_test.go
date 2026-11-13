package ui

// easyDragPluralForms lists, per locale, the plural categories besides "other" that
// Intl.PluralRules gives for the whole numbers 0 to 10000. scripts/test-easydrag-format.mjs
// compares this table with Intl. A count text needs a key_<category> text for each entry;
// the bare key is the "other" form.
var easyDragPluralForms = map[string][]string{
	"cs": {"one", "few"},
	"da": {"one"},
	"de": {"one"},
	"el": {"one"},
	"en": {"one"},
	"es": {"one"},
	"fr": {"one"},
	"hi": {"one"},
	"it": {"one"},
	"ja": {},
	"nl": {"one"},
	"no": {"one"},
	"pl": {"one", "few", "many"},
	"pt": {"one"},
	"sv": {"one"},
	"zh": {},
}
