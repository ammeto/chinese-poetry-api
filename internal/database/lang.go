package database

import (
	"fmt"
	"io"
)

// Lang represents the language variant for Chinese text
type Lang string

const (
	// LangHans represents Simplified Chinese (zh-Hans)
	LangHans Lang = "zh-Hans"
	// LangHant represents Traditional Chinese (zh-Hant)
	LangHant Lang = "zh-Hant"
)

// IsValid checks if the language variant is valid
func (l Lang) IsValid() bool {
	return l == LangHans || l == LangHant
}

// Default returns the default language (simplified Chinese)
func (l Lang) Default() Lang {
	if l.IsValid() {
		return l
	}
	return LangHans
}

// langAliases maps every accepted spelling of a language variant to its Lang.
var langAliases = map[string]Lang{
	"zh-Hans":    LangHans,
	"zh_Hans":    LangHans,
	"hans":       LangHans,
	"sc":         LangHans,
	"simplified": LangHans,

	"zh-Hant":     LangHant,
	"zh_Hant":     LangHant,
	"hant":        LangHant,
	"tc":          LangHant,
	"traditional": LangHant,
}

// ParseLang parses a string to Lang, defaulting to simplified Chinese
func ParseLang(s string) Lang {
	if lang, ok := langAliases[s]; ok {
		return lang
	}
	return LangHans
}

// LookupLang parses s like ParseLang but reports whether s was a recognised
// spelling, letting callers reject a typo instead of silently serving zh-Hans.
func LookupLang(s string) (Lang, bool) {
	lang, ok := langAliases[s]
	return lang, ok
}

// GraphQL enum names for Lang, as declared by the Lang enum in schema.graphqls.
const (
	gqlLangHans = "ZH_HANS"
	gqlLangHant = "ZH_HANT"
)

// UnmarshalGQL implements graphql.Unmarshaler so the schema's Lang enum maps
// onto this type.
//
// Without it, gqlgen's autobind converts the enum name straight to Lang, giving
// Lang("ZH_HANT") - a value that equals neither LangHans nor LangHant, so every
// table helper fell through to its simplified branch and the lang argument had
// no effect anywhere in the GraphQL API.
func (l *Lang) UnmarshalGQL(v any) error {
	name, ok := v.(string)
	if !ok {
		return fmt.Errorf("Lang must be one of %s or %s, got %T", gqlLangHans, gqlLangHant, v)
	}

	switch name {
	case gqlLangHans:
		*l = LangHans
	case gqlLangHant:
		*l = LangHant
	default:
		return fmt.Errorf("Lang must be one of %s or %s, got %q", gqlLangHans, gqlLangHant, name)
	}
	return nil
}

// MarshalGQL implements graphql.Marshaler, writing the enum name rather than
// the underlying "zh-Hans"/"zh-Hant" value, which is not a valid enum literal.
func (l Lang) MarshalGQL(w io.Writer) {
	name := gqlLangHans
	if l == LangHant {
		name = gqlLangHant
	}
	fmt.Fprintf(w, "%q", name)
}

// Table name helpers - these help construct table names with language suffix

// PoemsTable returns the poems table name for the given language
func PoemsTable(lang Lang) string {
	if lang == LangHant {
		return "poems_zh_hant"
	}
	return "poems_zh_hans"
}

// AuthorsTable returns the authors table name for the given language
func AuthorsTable(lang Lang) string {
	if lang == LangHant {
		return "authors_zh_hant"
	}
	return "authors_zh_hans"
}

// DynastiesTable returns the dynasties table name for the given language
func DynastiesTable(lang Lang) string {
	if lang == LangHant {
		return "dynasties_zh_hant"
	}
	return "dynasties_zh_hans"
}

// PoetryTypesTable returns the poetry_types table name for the given language
func PoetryTypesTable(lang Lang) string {
	if lang == LangHant {
		return "poetry_types_zh_hant"
	}
	return "poetry_types_zh_hans"
}

// PoemsFtsTable returns the FTS5 virtual table name backing full-text search
// for the given language's poems table
func PoemsFtsTable(lang Lang) string {
	if lang == LangHant {
		return "poems_fts_zh_hant"
	}
	return "poems_fts_zh_hans"
}

// Internal lowercase versions for use within this package
func poemsTable(lang Lang) string       { return PoemsTable(lang) }
func authorsTable(lang Lang) string     { return AuthorsTable(lang) }
func dynastiesTable(lang Lang) string   { return DynastiesTable(lang) }
func poetryTypesTable(lang Lang) string { return PoetryTypesTable(lang) }
func poemsFtsTable(lang Lang) string    { return PoemsFtsTable(lang) }
