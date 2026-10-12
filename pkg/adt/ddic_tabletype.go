package adt

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Table types over /sap/bc/adt/ddic/tabletypes: one XML document per
// object, read whole and written whole like domains and data elements
// (see ddic_objects.go). 7.57 has the resource; 7.50 and 7.40 do not, and
// adt-fallback serves the same document under /sap/bc/adt/zvsp.
//
// The document as 7.57 serves it: the row type (typeKind, typeName,
// builtInType, rangeType), initialRowCount, accessType, primaryKey
// (definition, kind, components, alias) and secondaryKeys (allowed, and
// each key with its access, definition and components), then valueHelps —
// the lists of the allowed values, the same for every object, which a
// write leaves out. A range table type names its range structure in
// rangeType; its key is fixed (standard, non-unique). Neither the
// standard nor the dictionary's activation makes that structure: SE11 has a
// button for it. A write here makes it when it is missing (see
// ensureRangeStructure).

// ObjectTypeTableType is a DDIC table type (SE11).
const ObjectTypeTableType CreatableObjectType = "TTYP/DA"

const tableTypeNS = "http://www.sap.com/dictionary/tabletype"

// tableTypeAccept is the only version there is (7.57).
const tableTypeAccept = "application/vnd.sap.adt.tabletype.v1+xml"

// The row type kinds of a table type, as the document names them. The
// predefined and reference kinds are the data element's TypeKind* values.
const (
	RowKindDictionary         = "dictionaryType"
	RowKindRefToClassOrIntf   = "refToClassOrInterfaceType"
	RowKindRangeOnPredefined  = "rangeTypeOnPredefinedType"
	RowKindRangeOnDataElement = "rangeTypeOnDataelement"
)

// The access types, key definitions and key kinds of a table type.
const (
	AccessStandard     = "standard"
	AccessSorted       = "sorted"
	AccessHashed       = "hashed"
	AccessIndex        = "index"
	NotSpecified       = "notSpecified"
	KeyDefStandard     = "standard"
	KeyDefRowType      = "rowType"
	KeyDefComponents   = "keyComponents"
	KeyKindUnique      = "unique"
	KeyKindNonUnique   = "nonUnique"
	SecKeyUniqueHashed = "uniqueHashed"
	SecKeyUniqueSorted = "uniqueSorted"
	SecKeyNonUnique    = "nonUniqueSorted"
	SecKeysAllowed     = "allowed"
	SecKeysNotAllowed  = "notAllowed"
)

// TableType is a DDIC table type.
type TableType struct {
	ddicHeader
	// TypeKind is one of RowKind* or TypeKindPredefined,
	// TypeKindRefToPredefined, TypeKindRefToDictionary. TypeName is the
	// dictionary type, the referenced type or class, or the data element of
	// a range; DataType, Length and Decimals the predefined type (for a
	// dictionary type, what it is, as activated: STRU, TTYP, CHAR...).
	TypeKind string `json:"typeKind"`
	TypeName string `json:"typeName,omitempty"`
	DataType string `json:"dataType,omitempty"`
	Length   int    `json:"length"`
	Decimals int    `json:"decimals"`
	// RangeType is the range structure of a range table type.
	RangeType string `json:"rangeType,omitempty"`

	InitialRowCount int    `json:"initialRowCount"`
	AccessType      string `json:"accessType"`
	// KeyDefinition: standard, rowType, keyComponents or notSpecified;
	// KeyKind: unique, nonUnique or notSpecified.
	KeyDefinition string   `json:"keyDefinition"`
	KeyKind       string   `json:"keyKind"`
	KeyComponents []string `json:"keyComponents,omitempty"`
	KeyAlias      string   `json:"keyAlias,omitempty"`

	// SecondaryKeysAllowed: allowed, notAllowed or notSpecified — whether
	// a declaration may add further secondary keys.
	SecondaryKeysAllowed string                  `json:"secondaryKeysAllowed"`
	SecondaryKeys        []TableTypeSecondaryKey `json:"secondaryKeys,omitempty"`

	contentType string
}

// TableTypeSecondaryKey is one secondary key.
type TableTypeSecondaryKey struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Language is the description's, as the document has it (one letter).
	Language string `json:"language,omitempty"`
	// Access: uniqueHashed, uniqueSorted or nonUniqueSorted.
	Access string `json:"access"`
	// Definition: rowType or keyComponents.
	Definition string   `json:"definition"`
	Components []string `json:"components,omitempty"`
}

func init() {
	objectTypes[ObjectTypeTableType] = objectTypeInfo{
		creationPath: "/sap/bc/adt/ddic/tabletypes",
		rootName:     "ttyp:tableType",
		namespace:    `xmlns:ttyp="` + tableTypeNS + `"`,
		bodyBuilder:  buildDDICCreateBody,
	}
}

func tableTypeURL(name string) string {
	return "/sap/bc/adt/ddic/tabletypes/" + url.PathEscape(strings.ToLower(name))
}

// IsRange tells a range table type.
func (t *TableType) IsRange() bool {
	return t.TypeKind == RowKindRangeOnPredefined || t.TypeKind == RowKindRangeOnDataElement
}

// --- reading ---

type ttypComponents struct {
	Names []struct {
		Name string `xml:"name,attr"`
	} `xml:"component"`
}

func (c ttypComponents) list() []string {
	var out []string
	for _, n := range c.Names {
		out = append(out, n.Name)
	}
	return out
}

type tableTypeDoc struct {
	ddicHeader
	Row struct {
		TypeKind string `xml:"typeKind"`
		TypeName string `xml:"typeName"`
		BuiltIn  struct {
			DataType string `xml:"dataType"`
			Length   string `xml:"length"`
			Decimals string `xml:"decimals"`
		} `xml:"builtInType"`
		RangeType string `xml:"rangeType"`
	} `xml:"rowType"`
	InitialRowCount string `xml:"initialRowCount"`
	AccessType      string `xml:"accessType"`
	PrimaryKey      struct {
		Definition string         `xml:"definition"`
		Kind       string         `xml:"kind"`
		Components ttypComponents `xml:"components"`
		Alias      string         `xml:"alias"`
	} `xml:"primaryKey"`
	SecondaryKeys struct {
		Allowed string `xml:"allowed"`
		Keys    []struct {
			Name        string         `xml:"identifier,attr"`
			Description string         `xml:"description,attr"`
			Language    string         `xml:"language,attr"`
			Access      string         `xml:"access,attr"`
			Definition  string         `xml:"definition,attr"`
			Components  ttypComponents `xml:"components"`
		} `xml:"secondaryKey"`
	} `xml:"secondaryKeys"`
}

// GetTableType reads a table type: the newest version, inactive if there
// is one.
func (c *Client) GetTableType(ctx context.Context, name string) (*TableType, error) {
	if err := c.checkSafety(OpRead, "GetTableType"); err != nil {
		return nil, err
	}
	name = strings.ToUpper(strings.TrimSpace(name))
	body, ct, err := c.readDDIC(ctx, tableTypeURL(name), tableTypeAccept, false)
	if err != nil {
		return nil, fmt.Errorf("reading table type %s: %w", name, err)
	}
	t, err := parseTableType(body)
	if err != nil {
		return nil, fmt.Errorf("parsing table type %s: %w", name, err)
	}
	t.contentType = ct
	return t, nil
}

func parseTableType(body []byte) (*TableType, error) {
	var doc tableTypeDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	t := &TableType{
		ddicHeader:           doc.ddicHeader,
		TypeKind:             doc.Row.TypeKind,
		TypeName:             doc.Row.TypeName,
		DataType:             doc.Row.BuiltIn.DataType,
		Length:               num(doc.Row.BuiltIn.Length),
		Decimals:             num(doc.Row.BuiltIn.Decimals),
		RangeType:            doc.Row.RangeType,
		InitialRowCount:      num(doc.InitialRowCount),
		AccessType:           doc.AccessType,
		KeyDefinition:        doc.PrimaryKey.Definition,
		KeyKind:              doc.PrimaryKey.Kind,
		KeyComponents:        doc.PrimaryKey.Components.list(),
		KeyAlias:             doc.PrimaryKey.Alias,
		SecondaryKeysAllowed: doc.SecondaryKeys.Allowed,
	}
	t.Package = t.PackageRef.Name
	for _, k := range doc.SecondaryKeys.Keys {
		t.SecondaryKeys = append(t.SecondaryKeys, TableTypeSecondaryKey{
			Name: k.Name, Description: k.Description, Language: k.Language,
			Access: k.Access, Definition: k.Definition, Components: k.Components.list(),
		})
	}
	return t, nil
}

// --- writing ---

func ttypComponentsXML(names []string) string {
	if len(names) == 0 {
		return `<ttyp:components/>`
	}
	var b strings.Builder
	b.WriteString(`<ttyp:components>`)
	for _, n := range names {
		fmt.Fprintf(&b, `<ttyp:component ttyp:name="%s"/>`, escapeXML(strings.ToUpper(n)))
	}
	b.WriteString(`</ttyp:components>`)
	return b.String()
}

// tableTypeDocBody is the PUT document: the whole table type, the
// elements in the order the GET serves them, without the value helps.
func tableTypeDocBody(t *TableType, lang string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(&b, `<ttyp:tableType xmlns:ttyp="%s" %s>`, tableTypeNS, ddicRootAttrs(t.ddicHeader, string(ObjectTypeTableType), lang))
	b.WriteString(ddicPackageRef(t.ddicHeader))
	b.WriteString(`<ttyp:rowType>`)
	b.WriteString(xmlEl("ttyp:typeKind", t.TypeKind))
	b.WriteString(xmlEl("ttyp:typeName", strings.ToUpper(t.TypeName)))
	b.WriteString(`<ttyp:builtInType>`)
	b.WriteString(xmlEl("ttyp:dataType", strings.ToUpper(t.DataType)))
	fmt.Fprintf(&b, `<ttyp:length>%06d</ttyp:length><ttyp:decimals>%06d</ttyp:decimals>`, t.Length, t.Decimals)
	b.WriteString(`</ttyp:builtInType>`)
	b.WriteString(xmlEl("ttyp:rangeType", strings.ToUpper(t.RangeType)))
	b.WriteString(`</ttyp:rowType>`)
	fmt.Fprintf(&b, `<ttyp:initialRowCount>%05d</ttyp:initialRowCount>`, t.InitialRowCount)
	b.WriteString(xmlEl("ttyp:accessType", t.AccessType))
	b.WriteString(`<ttyp:primaryKey>`)
	b.WriteString(xmlEl("ttyp:definition", t.KeyDefinition))
	b.WriteString(xmlEl("ttyp:kind", t.KeyKind))
	b.WriteString(ttypComponentsXML(t.KeyComponents))
	b.WriteString(xmlEl("ttyp:alias", strings.ToUpper(t.KeyAlias)))
	b.WriteString(`</ttyp:primaryKey><ttyp:secondaryKeys>`)
	b.WriteString(xmlEl("ttyp:allowed", t.SecondaryKeysAllowed))
	for _, k := range t.SecondaryKeys {
		fmt.Fprintf(&b, `<ttyp:secondaryKey ttyp:identifier="%s" ttyp:description="%s" ttyp:language="%s" ttyp:access="%s" ttyp:definition="%s">`,
			escapeXML(strings.ToUpper(k.Name)), escapeXML(k.Description), escapeXML(k.Language), escapeXML(k.Access), escapeXML(k.Definition))
		b.WriteString(ttypComponentsXML(k.Components))
		b.WriteString(`</ttyp:secondaryKey>`)
	}
	b.WriteString(`</ttyp:secondaryKeys></ttyp:tableType>`)
	return []byte(b.String())
}

// fillDefaults gives what a new table type leaves open SE11's defaults: a
// standard table with the standard key, non-unique; and a range table type
// its fixed key.
func (t *TableType) fillDefaults() {
	if t.AccessType == "" {
		t.AccessType = AccessStandard
	}
	if t.KeyDefinition == "" {
		t.KeyDefinition = KeyDefStandard
	}
	if t.KeyKind == "" {
		t.KeyKind = KeyKindNonUnique
		if t.AccessType == AccessHashed {
			t.KeyKind = KeyKindUnique
		}
	}
	if t.SecondaryKeysAllowed == "" {
		t.SecondaryKeysAllowed = NotSpecified
	}
	if t.IsRange() {
		t.AccessType, t.KeyDefinition, t.KeyKind = AccessStandard, KeyDefStandard, KeyKindNonUnique
		t.KeyComponents, t.KeyAlias = nil, ""
	}
	for i := range t.SecondaryKeys {
		if t.SecondaryKeys[i].Definition == "" {
			t.SecondaryKeys[i].Definition = KeyDefComponents
		}
	}
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// Check refuses a table type the dictionary would refuse, or one whose
// parts contradict each other.
func (t *TableType) Check() error {
	fail := func(format string, a ...any) error {
		return fmt.Errorf("table type %s: "+format, append([]any{t.Name}, a...)...)
	}
	switch t.TypeKind {
	case RowKindDictionary, TypeKindRefToDictionary, RowKindRefToClassOrIntf, RowKindRangeOnDataElement:
		if strings.TrimSpace(t.TypeName) == "" {
			return fail("row type kind %s needs a type name", t.TypeKind)
		}
	case TypeKindPredefined, TypeKindRefToPredefined, RowKindRangeOnPredefined:
		if strings.TrimSpace(t.DataType) == "" {
			return fail("row type kind %s needs a data type", t.TypeKind)
		}
	default:
		return fail("row type kind %q: want %s", t.TypeKind, strings.Join([]string{RowKindDictionary, TypeKindPredefined,
			TypeKindRefToPredefined, TypeKindRefToDictionary, RowKindRefToClassOrIntf, RowKindRangeOnPredefined, RowKindRangeOnDataElement}, ", "))
	}
	if t.IsRange() && strings.TrimSpace(t.RangeType) == "" {
		return fail("a range table type needs its range structure (rangeType)")
	}
	if !t.IsRange() && t.RangeType != "" {
		return fail("rangeType belongs to a range table type")
	}
	if t.Length < 0 || t.Decimals < 0 {
		return fail("negative length or decimals")
	}
	if t.InitialRowCount < 0 || t.InitialRowCount > 99999 {
		return fail("initial row count %d, want 0 to 99999", t.InitialRowCount)
	}
	if n := len([]rune(t.Description)); n > 60 {
		return fail("short text of %d characters, at most 60", n)
	}
	if !oneOf(t.AccessType, AccessStandard, AccessSorted, AccessHashed, AccessIndex, NotSpecified) {
		return fail("access type %q: want standard, sorted, hashed, index or notSpecified", t.AccessType)
	}
	if !oneOf(t.KeyDefinition, KeyDefStandard, KeyDefRowType, KeyDefComponents, NotSpecified) {
		return fail("key definition %q: want standard, rowType, keyComponents or notSpecified", t.KeyDefinition)
	}
	if !oneOf(t.KeyKind, KeyKindUnique, KeyKindNonUnique, NotSpecified) {
		return fail("key kind %q: want unique, nonUnique or notSpecified", t.KeyKind)
	}
	if t.AccessType == AccessHashed && t.KeyKind == KeyKindNonUnique {
		return fail("a hashed table has a unique key")
	}
	if (t.AccessType == AccessStandard || t.AccessType == AccessIndex) && t.KeyKind == KeyKindUnique {
		return fail("a %s table cannot have a unique key", t.AccessType)
	}
	if (t.KeyDefinition == KeyDefComponents) != (len(t.KeyComponents) > 0) {
		return fail("key components go with key definition keyComponents, and it needs them")
	}
	if len(t.KeyAlias) > 30 {
		return fail("key alias %s longer than 30 characters", t.KeyAlias)
	}
	if !oneOf(t.SecondaryKeysAllowed, SecKeysAllowed, SecKeysNotAllowed, NotSpecified) {
		return fail("secondaryKeysAllowed %q: want allowed, notAllowed or notSpecified", t.SecondaryKeysAllowed)
	}
	seen := map[string]bool{}
	for _, k := range t.SecondaryKeys {
		name := strings.ToUpper(strings.TrimSpace(k.Name))
		if name == "" || len(name) > 30 {
			return fail("secondary key name %q: 1 to 30 characters", k.Name)
		}
		if name == "PRIMARY_KEY" || name == strings.ToUpper(t.KeyAlias) || seen[name] {
			return fail("secondary key %s: the name is taken", name)
		}
		seen[name] = true
		if !oneOf(k.Access, SecKeyUniqueHashed, SecKeyUniqueSorted, SecKeyNonUnique) {
			return fail("secondary key %s: access %q, want uniqueHashed, uniqueSorted or nonUniqueSorted", name, k.Access)
		}
		if !oneOf(k.Definition, KeyDefRowType, KeyDefComponents) {
			return fail("secondary key %s: definition %q, want rowType or keyComponents", name, k.Definition)
		}
		if (k.Definition == KeyDefComponents) != (len(k.Components) > 0) {
			return fail("secondary key %s: components go with definition keyComponents, and it needs them", name)
		}
		if n := len([]rune(k.Description)); n > 60 {
			return fail("secondary key %s: description of %d characters, at most 60", name, n)
		}
	}
	return nil
}

// FillDefaults gives what a table type leaves open SE11's defaults.
func (t *TableType) FillDefaults() { t.fillDefaults() }

// rangeLineType is the DDL type of LOW and HIGH of a range structure: the
// data element, or the predefined type as abap.<type>(<length>[,<decimals>]).
func rangeLineType(t *TableType) (string, error) {
	if t.TypeKind == RowKindRangeOnDataElement {
		return strings.ToLower(t.TypeName), nil
	}
	typ := strings.ToLower(strings.TrimSpace(t.DataType))
	switch typ {
	case "strg":
		typ = "string"
	case "rstr":
		typ = "rawstring"
	case "sstr":
		typ = "sstring"
	}
	switch typ {
	case "int1", "int2", "int4", "int8", "dats", "tims", "fltp", "clnt", "lang", "cuky", "accp", "prec",
		"df16_raw", "df16_scl", "df34_raw", "df34_scl", "datn", "timn", "utclong":
		return "abap." + typ, nil
	case "string", "rawstring":
		return fmt.Sprintf("abap.%s(%d)", typ, t.Length), nil
	case "char", "numc", "raw", "sstring", "lchr", "lraw", "unit":
		if t.Length <= 0 {
			return "", fmt.Errorf("range table type %s: %s needs a length", t.Name, strings.ToUpper(typ))
		}
		return fmt.Sprintf("abap.%s(%d)", typ, t.Length), nil
	case "dec", "curr", "quan", "df16_dec", "df34_dec":
		if t.Length <= 0 {
			return "", fmt.Errorf("range table type %s: %s needs a length", t.Name, strings.ToUpper(typ))
		}
		return fmt.Sprintf("abap.%s(%d,%d)", typ, t.Length, t.Decimals), nil
	}
	return "", fmt.Errorf("range table type %s: no range structure can be made for type %s; create it first", t.Name, t.DataType)
}

// rangeStructureSource is the DDL of a range structure: SIGN, OPTION, LOW
// and HIGH, as SE11 makes it. Before 7.52 a structure is "define type".
func rangeStructureSource(name, lineType, description, release string) string {
	keyword := "structure"
	if n, err := strconv.Atoi(strings.TrimSpace(release)); err == nil && n < 752 {
		keyword = "type"
	}
	label := strings.ReplaceAll(description, "'", "''")
	return fmt.Sprintf("@EndUserText.label : '%s'\n@AbapCatalog.enhancementCategory : #NOT_EXTENSIBLE\n"+
		"define %s %s {\n  sign   : ddsign;\n  option : ddoption;\n  low    : %s;\n  high   : %s;\n}\n",
		label, keyword, strings.ToLower(name), lineType, lineType)
}

// ensureRangeStructure makes the range structure of a range table type
// when the dictionary has none, in the table type's package, and says so.
func (c *Client) ensureRangeStructure(ctx context.Context, t *TableType, pkg, transport string) (string, error) {
	if !t.IsRange() {
		return "", nil
	}
	name := strings.ToUpper(strings.TrimSpace(t.RangeType))
	_, err := c.GetStructure(ctx, name)
	if err == nil {
		return "", nil
	}
	if !IsNotFoundError(err) || IsNoHandlerError(err) {
		return "", fmt.Errorf("range structure %s: %w", name, err)
	}
	line, err := rangeLineType(t)
	if err != nil {
		return "", err
	}
	info, err := c.GetSystemInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("range structure %s is missing, and the release to write it for is unknown: %w", name, err)
	}
	description := "Range for " + t.Name
	if _, err := c.CreateStructure(ctx, StructureOptions{
		Name:        name,
		Description: description,
		Package:     pkg,
		Transport:   transport,
		Source:      rangeStructureSource(name, line, description, info.SAPRelease),
	}); err != nil {
		return "", fmt.Errorf("making range structure %s: %w", name, err)
	}
	return fmt.Sprintf("range structure %s created (SIGN, OPTION, LOW, HIGH of %s)", name, line), nil
}

// CreateTableTypeFrom creates a table type and activates it. t.Package and
// t.Description are required; what is left open gets SE11's defaults. A
// range table type's structure is made when it is missing.
func (c *Client) CreateTableTypeFrom(ctx context.Context, t *TableType, transport string) (*DDICWrite, error) {
	t.Name = strings.ToUpper(strings.TrimSpace(t.Name))
	t.fillDefaults()
	if err := t.Check(); err != nil {
		return nil, err
	}
	if t.IsRange() && strings.TrimSpace(t.Package) == "" {
		return nil, fmt.Errorf("%s %s: package and description are required", ObjectTypeTableType, t.Name)
	}
	note, err := c.ensureRangeStructure(ctx, t, t.Package, transport)
	if err != nil {
		return nil, err
	}
	w, err := c.createDDIC(ctx, ObjectTypeTableType, t.ddicHeader, transport)
	if w != nil && note != "" {
		w.Notes = append([]string{note}, w.Notes...)
	}
	if err != nil {
		return w, err
	}
	err = c.writeDDIC(ctx, w, tableTypeURL(t.Name), func(cur []byte) ([]byte, error) {
		return tableTypeDocBody(t, c.config.Language), nil
	}, tableTypeAccept)
	return w, err
}

// UpdateTableType reads a table type, lets change edit it and writes it
// back whole, then activates it. A range structure the change names and the
// dictionary lacks is made first, outside the table type's lock.
func (c *Client) UpdateTableType(ctx context.Context, name, transport string, change func(*TableType) error) (*DDICWrite, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	w := &DDICWrite{Type: string(ObjectTypeTableType), Name: name, Transport: transport}
	if err := c.checkMutation(ctx, MutationContext{Op: OpUpdate, OpName: "UpdateTableType", ObjectURL: tableTypeURL(name), Transport: transport}); err != nil {
		return nil, err
	}
	if before, err := c.GetTableType(ctx, name); err == nil {
		probe := *before
		if err := change(&probe); err != nil {
			return nil, err
		}
		// Before the range structure is made: a refused change writes nothing.
		if err := c.checkTextLanguage(probe.ddicHeader, ObjectTypeTableType, probe.Description != before.Description); err != nil {
			return nil, err
		}
		if probe.IsRange() {
			note, err := c.ensureRangeStructure(ctx, &probe, before.Package, transport)
			if err != nil {
				return w, err
			}
			if note != "" {
				w.Notes = append(w.Notes, note)
			}
		}
	}
	err := c.writeDDIC(ctx, w, tableTypeURL(name), func(cur []byte) ([]byte, error) {
		t, err := parseTableType(cur)
		if err != nil {
			return nil, fmt.Errorf("parsing table type %s: %w", name, err)
		}
		t.Name = name
		description := t.Description
		if err := change(t); err != nil {
			return nil, err
		}
		if err := c.checkTextLanguage(t.ddicHeader, ObjectTypeTableType, t.Description != description); err != nil {
			return nil, err
		}
		t.fillDefaults()
		if err := t.Check(); err != nil {
			return nil, err
		}
		return tableTypeDocBody(t, c.config.Language), nil
	}, tableTypeAccept)
	return w, err
}
