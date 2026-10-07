package adt

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// Shapes read off 7.57: a hashed table with key components and a
// secondary key, value helps cut short (they are the same for every
// object), and a range table type on a data element.
const testTableTypeXML = `<?xml version="1.0" encoding="utf-8"?><ttyp:tableType adtcore:responsible="TESTUSER" adtcore:masterLanguage="EN" adtcore:masterSystem="DEV" adtcore:abapLanguageVersion="0" adtcore:name="ZDEMO_KV_T" adtcore:type="TTYP/DA" adtcore:changedAt="2025-12-24T14:34:48Z" adtcore:version="active" adtcore:description="Key/value pairs (hashed)" adtcore:descriptionTextLimit="60" adtcore:language="RU" xmlns:ttyp="http://www.sap.com/dictionary/tabletype" xmlns:adtcore="http://www.sap.com/adt/core">` +
	`<atom:link href="versions" rel="http://www.sap.com/adt/relations/versions" title="Historic versions" xmlns:atom="http://www.w3.org/2005/Atom"/>` +
	`<adtcore:packageRef adtcore:uri="/sap/bc/adt/packages/zdemo" adtcore:type="DEVC/K" adtcore:name="ZDEMO" adtcore:description="Demo"/>` +
	`<ttyp:rowType><ttyp:typeKind>dictionaryType</ttyp:typeKind><ttyp:typeName>ZDEMO_KV</ttyp:typeName><ttyp:builtInType><ttyp:dataType>STRU</ttyp:dataType><ttyp:length>000000</ttyp:length><ttyp:decimals>000000</ttyp:decimals></ttyp:builtInType><ttyp:rangeType/></ttyp:rowType>` +
	`<ttyp:initialRowCount>00005</ttyp:initialRowCount><ttyp:accessType>hashed</ttyp:accessType>` +
	`<ttyp:primaryKey ttyp:isVisible="true" ttyp:isEditable="true"><ttyp:definition>keyComponents</ttyp:definition><ttyp:kind>unique</ttyp:kind><ttyp:components ttyp:isVisible="true"><ttyp:component ttyp:name="KEY"/></ttyp:components><ttyp:alias>PK_KEY</ttyp:alias></ttyp:primaryKey>` +
	`<ttyp:secondaryKeys ttyp:isVisible="true" ttyp:isEditable="true"><ttyp:allowed>allowed</ttyp:allowed>` +
	`<ttyp:secondaryKey ttyp:identifier="BY_VALUE" ttyp:description="By value" ttyp:language="E" ttyp:access="nonUniqueSorted" ttyp:definition="keyComponents"><ttyp:components ttyp:isVisible="true"><ttyp:component ttyp:name="VALUE"/><ttyp:component ttyp:name="KEY"/></ttyp:components></ttyp:secondaryKey>` +
	`</ttyp:secondaryKeys>` +
	`<ttyp:valueHelps><ttyp:typeKindValues><ttyp:valueHelp ttyp:key="dictionaryType" ttyp:value="Dictionary Type" ttyp:description=""/></ttyp:typeKindValues></ttyp:valueHelps></ttyp:tableType>`

const testRangeTableTypeXML = `<?xml version="1.0" encoding="utf-8"?><ttyp:tableType adtcore:name="ZDEMO_FIELDNAME_RNG" adtcore:type="TTYP/DA" adtcore:version="active" adtcore:description="Field names" adtcore:language="RU" adtcore:masterLanguage="RU" xmlns:ttyp="http://www.sap.com/dictionary/tabletype" xmlns:adtcore="http://www.sap.com/adt/core">` +
	`<adtcore:packageRef adtcore:name="$TMP"/>` +
	`<ttyp:rowType><ttyp:typeKind>rangeTypeOnDataelement</ttyp:typeKind><ttyp:typeName>FIELDNAME</ttyp:typeName><ttyp:builtInType><ttyp:dataType>CHAR</ttyp:dataType><ttyp:length>000030</ttyp:length><ttyp:decimals>000000</ttyp:decimals></ttyp:builtInType><ttyp:rangeType>ZDEMO_FIELDNAME_RNG_S</ttyp:rangeType></ttyp:rowType>` +
	`<ttyp:initialRowCount>00000</ttyp:initialRowCount><ttyp:accessType>standard</ttyp:accessType>` +
	`<ttyp:primaryKey ttyp:isVisible="false" ttyp:isEditable="false"><ttyp:definition>standard</ttyp:definition><ttyp:kind>nonUnique</ttyp:kind><ttyp:components ttyp:isVisible="false"/><ttyp:alias/></ttyp:primaryKey>` +
	`<ttyp:secondaryKeys ttyp:isVisible="false" ttyp:isEditable="false"><ttyp:allowed>notSpecified</ttyp:allowed></ttyp:secondaryKeys></ttyp:tableType>`

func TestParseTableType(t *testing.T) {
	tt, err := parseTableType([]byte(testTableTypeXML))
	if err != nil {
		t.Fatal(err)
	}
	want := TableType{
		TypeKind: RowKindDictionary, TypeName: "ZDEMO_KV", DataType: "STRU",
		InitialRowCount: 5, AccessType: AccessHashed,
		KeyDefinition: KeyDefComponents, KeyKind: KeyKindUnique, KeyComponents: []string{"KEY"}, KeyAlias: "PK_KEY",
		SecondaryKeysAllowed: SecKeysAllowed,
		SecondaryKeys: []TableTypeSecondaryKey{{Name: "BY_VALUE", Description: "By value", Language: "E",
			Access: SecKeyNonUnique, Definition: KeyDefComponents, Components: []string{"VALUE", "KEY"}}},
	}
	want.ddicHeader = tt.ddicHeader
	if !reflect.DeepEqual(*tt, want) {
		t.Errorf("table type\n%+v\nwant\n%+v", *tt, want)
	}
	if tt.Name != "ZDEMO_KV_T" || tt.Package != "ZDEMO" || tt.MasterLanguage != "EN" || tt.Description != "Key/value pairs (hashed)" {
		t.Errorf("header = %+v", tt.ddicHeader)
	}

	r, err := parseTableType([]byte(testRangeTableTypeXML))
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsRange() || r.TypeName != "FIELDNAME" || r.RangeType != "ZDEMO_FIELDNAME_RNG_S" || r.Length != 30 || r.KeyComponents != nil {
		t.Errorf("range table type = %+v", r)
	}
}

// The body is read back by the same parser: what is written is what is read.
func TestTableTypeBodyRoundTrips(t *testing.T) {
	for _, src := range []string{testTableTypeXML, testRangeTableTypeXML} {
		tt, err := parseTableType([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		tt.Description = "Pairs <k> & v"
		body := string(tableTypeDocBody(tt, "ru"))
		if strings.Contains(body, "valueHelps") {
			t.Errorf("the body carries the value helps:\n%s", body)
		}
		again, err := parseTableType([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		tt.contentType, tt.Version = "", "" // the PUT names no version
		again.PackageRef = tt.PackageRef
		if !reflect.DeepEqual(again, tt) {
			t.Errorf("read back\n%+v\nwant\n%+v\nbody %s", again, tt, body)
		}
	}

	tt, _ := parseTableType([]byte(testTableTypeXML))
	body := string(tableTypeDocBody(tt, "RU"))
	for _, want := range []string{
		`<ttyp:tableType xmlns:ttyp="http://www.sap.com/dictionary/tabletype"`, `adtcore:type="TTYP/DA"`, `adtcore:masterLanguage="EN"`,
		`<adtcore:packageRef adtcore:name="ZDEMO"/>`,
		`<ttyp:rowType><ttyp:typeKind>dictionaryType</ttyp:typeKind><ttyp:typeName>ZDEMO_KV</ttyp:typeName><ttyp:builtInType><ttyp:dataType>STRU</ttyp:dataType><ttyp:length>000000</ttyp:length><ttyp:decimals>000000</ttyp:decimals></ttyp:builtInType><ttyp:rangeType/></ttyp:rowType>`,
		`<ttyp:initialRowCount>00005</ttyp:initialRowCount><ttyp:accessType>hashed</ttyp:accessType>`,
		`<ttyp:primaryKey><ttyp:definition>keyComponents</ttyp:definition><ttyp:kind>unique</ttyp:kind><ttyp:components><ttyp:component ttyp:name="KEY"/></ttyp:components><ttyp:alias>PK_KEY</ttyp:alias></ttyp:primaryKey>`,
		`<ttyp:secondaryKey ttyp:identifier="BY_VALUE" ttyp:description="By value" ttyp:language="E" ttyp:access="nonUniqueSorted" ttyp:definition="keyComponents">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %s:\n%s", want, body)
		}
	}
}

func TestTableTypeCheck(t *testing.T) {
	ok := func() TableType {
		return TableType{TypeKind: RowKindDictionary, TypeName: "ZDEMO_S", AccessType: AccessStandard,
			KeyDefinition: KeyDefStandard, KeyKind: KeyKindNonUnique, SecondaryKeysAllowed: NotSpecified}
	}
	for _, tc := range []struct {
		what string
		edit func(*TableType)
		fail bool
	}{
		{"as it is", func(*TableType) {}, false},
		{"no type name", func(t *TableType) { t.TypeName = "" }, true},
		{"unknown kind", func(t *TableType) { t.TypeKind = "structure" }, true},
		{"predefined", func(t *TableType) { t.TypeKind, t.TypeName, t.DataType, t.Length = TypeKindPredefined, "", "CHAR", 10 }, false},
		{"predefined without type", func(t *TableType) { t.TypeKind, t.TypeName = TypeKindPredefined, "" }, true},
		{"range without structure", func(t *TableType) { t.TypeKind, t.TypeName = RowKindRangeOnDataElement, "FIELDNAME" }, true},
		{"range", func(t *TableType) {
			t.TypeKind, t.TypeName, t.RangeType = RowKindRangeOnDataElement, "FIELDNAME", "ZDEMO_RNG_S"
		}, false},
		{"rangeType on a plain one", func(t *TableType) { t.RangeType = "ZDEMO_RNG_S" }, true},
		{"hashed non-unique", func(t *TableType) { t.AccessType = AccessHashed }, true},
		{"standard unique", func(t *TableType) { t.KeyKind = KeyKindUnique }, true},
		{"sorted unique by components", func(t *TableType) {
			t.AccessType, t.KeyKind, t.KeyDefinition, t.KeyComponents = AccessSorted, KeyKindUnique, KeyDefComponents, []string{"ID"}
		}, false},
		{"components without the definition", func(t *TableType) { t.KeyComponents = []string{"ID"} }, true},
		{"the definition without components", func(t *TableType) { t.KeyDefinition = KeyDefComponents }, true},
		{"unknown access", func(t *TableType) { t.AccessType = "HASHED" }, true},
		{"secondary key", func(t *TableType) {
			t.SecondaryKeys = []TableTypeSecondaryKey{{Name: "BY_ID", Access: SecKeyUniqueHashed, Definition: KeyDefComponents, Components: []string{"ID"}}}
		}, false},
		{"secondary key named primary_key", func(t *TableType) {
			t.SecondaryKeys = []TableTypeSecondaryKey{{Name: "primary_key", Access: SecKeyUniqueHashed, Definition: KeyDefRowType}}
		}, true},
		{"two secondary keys of one name", func(t *TableType) {
			k := TableTypeSecondaryKey{Name: "BY_ID", Access: SecKeyUniqueSorted, Definition: KeyDefRowType}
			t.SecondaryKeys = []TableTypeSecondaryKey{k, k}
		}, true},
		{"secondary key access", func(t *TableType) {
			t.SecondaryKeys = []TableTypeSecondaryKey{{Name: "BY_ID", Access: "hashed", Definition: KeyDefRowType}}
		}, true},
		{"initial rows", func(t *TableType) { t.InitialRowCount = 100000 }, true},
	} {
		tt := ok()
		tc.edit(&tt)
		if err := tt.Check(); (err != nil) != tc.fail {
			t.Errorf("%s: Check() = %v, want failure %v", tc.what, err, tc.fail)
		}
	}
}

func TestTableTypeDefaults(t *testing.T) {
	tt := &TableType{TypeKind: RowKindDictionary, TypeName: "ZDEMO_S"}
	tt.fillDefaults()
	if tt.AccessType != AccessStandard || tt.KeyDefinition != KeyDefStandard || tt.KeyKind != KeyKindNonUnique || tt.SecondaryKeysAllowed != NotSpecified {
		t.Errorf("defaults = %+v", tt)
	}
	h := &TableType{TypeKind: RowKindDictionary, TypeName: "ZDEMO_S", AccessType: AccessHashed, KeyDefinition: KeyDefComponents, KeyComponents: []string{"ID"}}
	h.fillDefaults()
	if h.KeyKind != KeyKindUnique || h.Check() != nil {
		t.Errorf("hashed defaults = %+v, check %v", h, h.Check())
	}
	// A range table type's key is fixed, whatever was asked for.
	r := &TableType{TypeKind: RowKindRangeOnDataElement, TypeName: "FIELDNAME", RangeType: "ZDEMO_RNG_S",
		AccessType: AccessSorted, KeyDefinition: KeyDefComponents, KeyComponents: []string{"LOW"}, KeyKind: KeyKindUnique}
	r.fillDefaults()
	if r.AccessType != AccessStandard || r.KeyDefinition != KeyDefStandard || r.KeyKind != KeyKindNonUnique || r.KeyComponents != nil || r.Check() != nil {
		t.Errorf("range defaults = %+v", r)
	}
}

func TestUpdateTableTypeWritesTheWholeDocument(t *testing.T) {
	rec := &adtRecorder{}
	var mu sync.Mutex
	var put, putType string
	client := newStubbedClient(t, rec, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = io.WriteString(w, testLockXML)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/ddic/tabletypes/"):
			w.Header().Set("Content-Type", "application/vnd.sap.adt.tabletype.v1+xml; charset=utf-8")
			_, _ = io.WriteString(w, testTableTypeXML)
		case r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			put, putType = string(b), r.Header.Get("Content-Type")
			mu.Unlock()
		default:
			w.WriteHeader(http.StatusOK)
		}
	}, WithLanguage("RU"))

	w, err := client.UpdateTableType(context.Background(), "zdemo_kv_t", "", func(t *TableType) error {
		t.SecondaryKeys = nil
		t.InitialRowCount = 0
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTableType: %v", err)
	}
	if w.Activation == nil || !w.Activation.Success || w.Type != "TTYP/DA" {
		t.Errorf("write = %+v", w)
	}
	if putType != "application/vnd.sap.adt.tabletype.v1+xml" {
		t.Errorf("PUT Content-Type %q", putType)
	}
	for _, want := range []string{
		`<ttyp:secondaryKeys><ttyp:allowed>allowed</ttyp:allowed></ttyp:secondaryKeys>`,
		`<ttyp:initialRowCount>00000</ttyp:initialRowCount>`, `<ttyp:component ttyp:name="KEY"/>`,
	} {
		if !strings.Contains(put, want) {
			t.Errorf("PUT lacks %s:\n%s", want, put)
		}
	}
	calls := rec.snapshot()
	lockAt := indexOfCall(calls, isLock)
	putAt := indexOfCall(calls, func(c wireCall) bool { return c.method == http.MethodPut })
	unlockAt := indexOfCall(calls, isUnlock)
	if lockAt < 0 || putAt < lockAt || unlockAt < putAt {
		t.Fatalf("want LOCK, PUT, UNLOCK: lock %d put %d unlock %d", lockAt, putAt, unlockAt)
	}
	assertWindowStateful(t, calls, lockAt, putAt+1)
}

func TestUpdateTableTypeRefusesAContradictionBeforeThePut(t *testing.T) {
	rec := &adtRecorder{}
	client := newStubbedClient(t, rec, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = io.WriteString(w, testLockXML)
		case r.Method == http.MethodGet:
			_, _ = io.WriteString(w, testTableTypeXML)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}, WithLanguage("RU"))
	_, err := client.UpdateTableType(context.Background(), "ZDEMO_KV_T", "", func(t *TableType) error {
		t.KeyKind = KeyKindNonUnique
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "hashed") {
		t.Fatalf("err = %v", err)
	}
	for _, c := range rec.snapshot() {
		if c.method == http.MethodPut {
			t.Errorf("a PUT went out: %+v", c)
		}
	}
}

func TestRangeStructureSource(t *testing.T) {
	for _, tc := range []struct {
		t       TableType
		release string
		want    []string
	}{
		{TableType{TypeKind: RowKindRangeOnDataElement, TypeName: "BUKRS"}, "750",
			[]string{"define type zdemo_rng_s {", "  low    : bukrs;", "  high   : bukrs;", "  sign   : ddsign;", "  option : ddoption;"}},
		{TableType{TypeKind: RowKindRangeOnPredefined, DataType: "STRING"}, "757",
			[]string{"define structure zdemo_rng_s {", "  low    : abap.string(0);"}},
		{TableType{TypeKind: RowKindRangeOnPredefined, DataType: "DEC", Length: 15, Decimals: 2}, "740",
			[]string{"define type zdemo_rng_s {", "  high   : abap.dec(15,2);"}},
		{TableType{TypeKind: RowKindRangeOnPredefined, DataType: "INT4"}, "757", []string{"  low    : abap.int4;"}},
	} {
		line, err := rangeLineType(&tc.t)
		if err != nil {
			t.Fatalf("%+v: %v", tc.t, err)
		}
		src := rangeStructureSource("ZDEMO_RNG_S", line, "Range for ZDEMO's", tc.release)
		for _, w := range append(tc.want, "@EndUserText.label : 'Range for ZDEMO''s'") {
			if !strings.Contains(src, w) {
				t.Errorf("source lacks %q:\n%s", w, src)
			}
		}
		if name, _, err := structureSourceName(src); err != nil || name != "ZDEMO_RNG_S" {
			t.Errorf("source declares %q (%v):\n%s", name, err, src)
		}
	}
	if _, err := rangeLineType(&TableType{TypeKind: RowKindRangeOnPredefined, DataType: "CHAR"}); err == nil {
		t.Error("CHAR without a length accepted")
	}
}
