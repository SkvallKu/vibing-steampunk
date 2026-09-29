package adt

import (
	"context"
	"encoding/xml"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Domains and data elements over /sap/bc/adt/ddic/domains and
// /sap/bc/adt/ddic/dataelements. Each object is one XML document: the GET
// serves it, the PUT takes it whole — there is no partial update, so a
// write reads the object, changes what it was asked to and puts all of it
// back. The documents are the same on every release that has the resource
// (7.50 serves data elements as v1, 7.57 both as v2, element for element
// alike). A release without the resource — domains on 7.50, both on
// 7.40 — answers with the ADT router's 404.
//
// A new object is created empty (CreateObject), then written and activated
// like any change. The texts the GET serves are the master language's
// (7.50 serves a German original in German to a Russian logon); which
// language a PUT writes to an object whose master language is not the
// logon language is not checked. A new object takes the logon language as
// its master language.

const (
	// ObjectTypeDomain is a DDIC domain (SE11).
	ObjectTypeDomain CreatableObjectType = "DOMA/DD"
	// ObjectTypeDataElement is a DDIC data element (SE11).
	ObjectTypeDataElement CreatableObjectType = "DTEL/DE"
)

const (
	domainNS      = "http://www.sap.com/dictionary/domain"
	dataElementNS = "http://www.sap.com/adt/dictionary/dataelements"
	dtelWbobjNS   = "http://www.sap.com/wbobj/dictionary/dtel"

	// Each release accepts only the versions it knows: 7.50 answers a v2-only
	// Accept for data elements with 406, 7.57 a v1-only one for domains.
	domainAccept      = "application/vnd.sap.adt.domains.v2+xml, application/vnd.sap.adt.domains.v1+xml"
	dataElementAccept = "application/vnd.sap.adt.dataelements.v2+xml, application/vnd.sap.adt.dataelements.v1+xml"
)

// Maximum lengths of the four field labels (DD04V-SCRTEXT_S/M/L, REPTEXT).
const (
	maxShortLabel   = 10
	maxMediumLabel  = 20
	maxLongLabel    = 40
	maxHeadingLabel = 55
)

// The type kinds of a data element, as the document names them.
const (
	TypeKindDomain           = "domain"
	TypeKindPredefined       = "predefinedAbapType"
	TypeKindRefToPredefined  = "refToPredefinedAbapType"
	TypeKindRefToDictionary  = "refToDictionaryType"
	TypeKindRefToClassOrIntf = "refToClifType"
)

func init() {
	objectTypes[ObjectTypeDomain] = objectTypeInfo{
		creationPath: "/sap/bc/adt/ddic/domains",
		rootName:     "doma:domain",
		namespace:    `xmlns:doma="` + domainNS + `"`,
		bodyBuilder:  buildDDICCreateBody,
	}
	objectTypes[ObjectTypeDataElement] = objectTypeInfo{
		creationPath: "/sap/bc/adt/ddic/dataelements",
		rootName:     "blue:wbobj",
		namespace:    `xmlns:blue="` + dtelWbobjNS + `"`,
		bodyBuilder:  buildDDICCreateBody,
	}
}

// buildDDICCreateBody is the create document of a domain or data element:
// the name, short text and package; the content follows with the first
// write. The logon language becomes the master language.
func buildDDICCreateBody(opts CreateObjectOptions, typeInfo objectTypeInfo, responsible string) string {
	lang := escapeXML(strings.ToUpper(opts.Language))
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<%s %s xmlns:adtcore="http://www.sap.com/adt/core"
  adtcore:name="%s"
  adtcore:type="%s"
  adtcore:description="%s"
  adtcore:language="%s"
  adtcore:masterLanguage="%s"
  adtcore:responsible="%s">
  <adtcore:packageRef adtcore:name="%s"/>
</%s>`, typeInfo.rootName, typeInfo.namespace, escapeXML(opts.Name), opts.ObjectType,
		escapeXML(opts.Description), lang, lang, escapeXML(responsible), escapeXML(opts.PackageName), typeInfo.rootName)
}

func domainURL(name string) string {
	return "/sap/bc/adt/ddic/domains/" + url.PathEscape(strings.ToLower(name))
}

func dataElementURL(name string) string {
	return "/sap/bc/adt/ddic/dataelements/" + url.PathEscape(strings.ToLower(name))
}

// ddicHeader is what both documents carry on the root and beside the content.
type ddicHeader struct {
	Name           string `xml:"name,attr" json:"name"`
	Description    string `xml:"description,attr" json:"description"`
	MasterLanguage string `xml:"masterLanguage,attr" json:"masterLanguage,omitempty"`
	Language       string `xml:"language,attr" json:"language,omitempty"`
	Version        string `xml:"version,attr" json:"version,omitempty"`
	PackageRef     struct {
		Name string `xml:"name,attr"`
	} `xml:"packageRef" json:"-"`
	Package string `xml:"-" json:"package"`
}

// Domain is a DDIC domain.
type Domain struct {
	ddicHeader
	DataType       string           `json:"dataType"`
	Length         int              `json:"length"`
	Decimals       int              `json:"decimals"`
	OutputLength   int              `json:"outputLength"`
	OutputStyle    string           `json:"outputStyle,omitempty"`
	ConversionExit string           `json:"conversionExit,omitempty"`
	SignExists     bool             `json:"signExists"`
	Lowercase      bool             `json:"lowercase"`
	AMPMFormat     bool             `json:"ampmFormat"`
	ValueTable     string           `json:"valueTable,omitempty"`
	AppendExists   bool             `json:"appendExists"`
	FixValues      []DomainFixValue `json:"fixValues,omitempty"`

	// contentType is the version of the document the system served,
	// which is the one it takes back.
	contentType string
}

// DomainFixValue is one fixed value, or an interval when High is set.
type DomainFixValue struct {
	Position int    `json:"position,omitempty"`
	Low      string `json:"low"`
	High     string `json:"high,omitempty"`
	Text     string `json:"text,omitempty"`
}

// DataElement is a DDIC data element.
type DataElement struct {
	ddicHeader
	// TypeKind is one of the TypeKind* constants. TypeName is the domain,
	// or the referenced type; DataType, Length and Decimals are the
	// predefined type (for a domain, the domain's, as activated).
	TypeKind string `json:"typeKind"`
	TypeName string `json:"typeName,omitempty"`
	DataType string `json:"dataType,omitempty"`
	Length   int    `json:"length"`
	Decimals int    `json:"decimals"`

	ShortLabel    string `json:"shortLabel"`
	ShortLength   int    `json:"shortLength"`
	MediumLabel   string `json:"mediumLabel"`
	MediumLength  int    `json:"mediumLength"`
	LongLabel     string `json:"longLabel"`
	LongLength    int    `json:"longLength"`
	HeadingLabel  string `json:"headingLabel"`
	HeadingLength int    `json:"headingLength"`

	SearchHelp              string `json:"searchHelp,omitempty"`
	SearchHelpParameter     string `json:"searchHelpParameter,omitempty"`
	SetGetParameter         string `json:"setGetParameter,omitempty"`
	DefaultComponentName    string `json:"defaultComponentName,omitempty"`
	DeactivateInputHistory  bool   `json:"deactivateInputHistory"`
	ChangeDocument          bool   `json:"changeDocument"`
	LeftToRightDirection    bool   `json:"leftToRightDirection"`
	DeactivateBIDIFiltering bool   `json:"deactivateBIDIFiltering"`

	contentType string
}

// --- reading ---

type domainDoc struct {
	ddicHeader
	Content struct {
		Type struct {
			DataType string `xml:"datatype"`
			Length   string `xml:"length"`
			Decimals string `xml:"decimals"`
		} `xml:"typeInformation"`
		Output struct {
			Length         string `xml:"length"`
			Style          string `xml:"style"`
			ConversionExit string `xml:"conversionExit"`
			SignExists     bool   `xml:"signExists"`
			Lowercase      bool   `xml:"lowercase"`
			AMPMFormat     bool   `xml:"ampmFormat"`
		} `xml:"outputInformation"`
		Values struct {
			ValueTable struct {
				Name string `xml:"name,attr"`
			} `xml:"valueTableRef"`
			AppendExists bool `xml:"appendExists"`
			FixValues    []struct {
				Position string `xml:"position"`
				Low      string `xml:"low"`
				High     string `xml:"high"`
				Text     string `xml:"text"`
				// A value an append contributes is the append's, not the domain's.
				Append *struct{} `xml:"contributingAppendRef"`
			} `xml:"fixValues>fixValue"`
		} `xml:"valueInformation"`
	} `xml:"content"`
}

type dataElementDocFull struct {
	ddicHeader
	DTEL struct {
		TypeKind                string `xml:"typeKind"`
		TypeName                string `xml:"typeName"`
		DataType                string `xml:"dataType"`
		Length                  string `xml:"dataTypeLength"`
		Decimals                string `xml:"dataTypeDecimals"`
		ShortLabel              string `xml:"shortFieldLabel"`
		ShortLength             string `xml:"shortFieldLength"`
		MediumLabel             string `xml:"mediumFieldLabel"`
		MediumLength            string `xml:"mediumFieldLength"`
		LongLabel               string `xml:"longFieldLabel"`
		LongLength              string `xml:"longFieldLength"`
		HeadingLabel            string `xml:"headingFieldLabel"`
		HeadingLength           string `xml:"headingFieldLength"`
		SearchHelp              string `xml:"searchHelp"`
		SearchHelpParameter     string `xml:"searchHelpParameter"`
		SetGetParameter         string `xml:"setGetParameter"`
		DefaultComponentName    string `xml:"defaultComponentName"`
		DeactivateInputHistory  bool   `xml:"deactivateInputHistory"`
		ChangeDocument          bool   `xml:"changeDocument"`
		LeftToRightDirection    bool   `xml:"leftToRightDirection"`
		DeactivateBIDIFiltering bool   `xml:"deactivateBIDIFiltering"`
	} `xml:"dataElement"`
}

// num reads a zero-padded number of the documents ("000004"); empty is 0.
func num(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func mediaType(contentType string) string {
	if mt, _, err := mime.ParseMediaType(contentType); err == nil {
		return mt
	}
	return contentType
}

// readDDIC GETs a document. Stateful keeps the read on the session of a
// lock already taken (#91).
func (c *Client) readDDIC(ctx context.Context, path, accept string, stateful bool) ([]byte, string, error) {
	resp, err := c.transport.Request(ctx, path, &RequestOptions{
		Method:   http.MethodGet,
		Accept:   accept,
		Stateful: stateful,
	})
	if err != nil {
		return nil, "", err
	}
	return resp.Body, mediaType(resp.Headers.Get("Content-Type")), nil
}

// GetDomain reads a domain: the newest version, inactive if there is one.
func (c *Client) GetDomain(ctx context.Context, name string) (*Domain, error) {
	if err := c.checkSafety(OpRead, "GetDomain"); err != nil {
		return nil, err
	}
	return c.readDomain(ctx, name, false)
}

func (c *Client) readDomain(ctx context.Context, name string, stateful bool) (*Domain, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	body, ct, err := c.readDDIC(ctx, domainURL(name), domainAccept, stateful)
	if err != nil {
		return nil, fmt.Errorf("reading domain %s: %w", name, err)
	}
	d, err := parseDomain(body)
	if err != nil {
		return nil, fmt.Errorf("parsing domain %s: %w", name, err)
	}
	d.contentType = ct
	return d, nil
}

func parseDomain(body []byte) (*Domain, error) {
	var doc domainDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	d := &Domain{
		ddicHeader:     doc.ddicHeader,
		DataType:       doc.Content.Type.DataType,
		Length:         num(doc.Content.Type.Length),
		Decimals:       num(doc.Content.Type.Decimals),
		OutputLength:   num(doc.Content.Output.Length),
		OutputStyle:    doc.Content.Output.Style,
		ConversionExit: doc.Content.Output.ConversionExit,
		SignExists:     doc.Content.Output.SignExists,
		Lowercase:      doc.Content.Output.Lowercase,
		AMPMFormat:     doc.Content.Output.AMPMFormat,
		ValueTable:     doc.Content.Values.ValueTable.Name,
		AppendExists:   doc.Content.Values.AppendExists,
	}
	d.Package = d.PackageRef.Name
	for _, v := range doc.Content.Values.FixValues {
		if v.Append != nil {
			continue
		}
		d.FixValues = append(d.FixValues, DomainFixValue{Position: num(v.Position), Low: v.Low, High: v.High, Text: v.Text})
	}
	return d, nil
}

// GetDataElement reads a data element: the newest version, inactive if
// there is one.
func (c *Client) GetDataElement(ctx context.Context, name string) (*DataElement, error) {
	if err := c.checkSafety(OpRead, "GetDataElement"); err != nil {
		return nil, err
	}
	return c.readDataElement(ctx, name, false)
}

func (c *Client) readDataElement(ctx context.Context, name string, stateful bool) (*DataElement, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	body, ct, err := c.readDDIC(ctx, dataElementURL(name), dataElementAccept, stateful)
	if err != nil {
		return nil, fmt.Errorf("reading data element %s: %w", name, err)
	}
	e, err := parseDataElement(body)
	if err != nil {
		return nil, fmt.Errorf("parsing data element %s: %w", name, err)
	}
	e.contentType = ct
	return e, nil
}

func parseDataElement(body []byte) (*DataElement, error) {
	var doc dataElementDocFull
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	x := doc.DTEL
	e := &DataElement{
		ddicHeader:              doc.ddicHeader,
		TypeKind:                x.TypeKind,
		TypeName:                x.TypeName,
		DataType:                x.DataType,
		Length:                  num(x.Length),
		Decimals:                num(x.Decimals),
		ShortLabel:              x.ShortLabel,
		ShortLength:             num(x.ShortLength),
		MediumLabel:             x.MediumLabel,
		MediumLength:            num(x.MediumLength),
		LongLabel:               x.LongLabel,
		LongLength:              num(x.LongLength),
		HeadingLabel:            x.HeadingLabel,
		HeadingLength:           num(x.HeadingLength),
		SearchHelp:              x.SearchHelp,
		SearchHelpParameter:     x.SearchHelpParameter,
		SetGetParameter:         x.SetGetParameter,
		DefaultComponentName:    x.DefaultComponentName,
		DeactivateInputHistory:  x.DeactivateInputHistory,
		ChangeDocument:          x.ChangeDocument,
		LeftToRightDirection:    x.LeftToRightDirection,
		DeactivateBIDIFiltering: x.DeactivateBIDIFiltering,
	}
	e.Package = e.PackageRef.Name
	return e, nil
}

// --- writing ---

func xmlEl(tag, value string) string {
	if value == "" {
		return "<" + tag + "/>"
	}
	return "<" + tag + ">" + escapeXML(value) + "</" + tag + ">"
}

func ddicRootAttrs(h ddicHeader, adtType, lang string) string {
	master := h.MasterLanguage
	if master == "" {
		master = lang
	}
	return fmt.Sprintf(`xmlns:adtcore="http://www.sap.com/adt/core" adtcore:name="%s" adtcore:type="%s" adtcore:description="%s" adtcore:language="%s" adtcore:masterLanguage="%s"`,
		escapeXML(strings.ToUpper(h.Name)), adtType, escapeXML(h.Description), escapeXML(strings.ToUpper(lang)), escapeXML(strings.ToUpper(master)))
}

func ddicPackageRef(h ddicHeader) string {
	if h.Package == "" {
		return ""
	}
	return fmt.Sprintf(`<adtcore:packageRef adtcore:name="%s"/>`, escapeXML(strings.ToUpper(h.Package)))
}

// domainBody is the PUT document: the whole domain.
func domainBody(d *Domain, lang string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(&b, `<doma:domain xmlns:doma="%s" %s>`, domainNS, ddicRootAttrs(d.ddicHeader, string(ObjectTypeDomain), lang))
	b.WriteString(ddicPackageRef(d.ddicHeader))
	b.WriteString(`<doma:content><doma:typeInformation>`)
	b.WriteString(xmlEl("doma:datatype", strings.ToUpper(d.DataType)))
	fmt.Fprintf(&b, `<doma:length>%06d</doma:length><doma:decimals>%06d</doma:decimals>`, d.Length, d.Decimals)
	b.WriteString(`</doma:typeInformation><doma:outputInformation>`)
	fmt.Fprintf(&b, `<doma:length>%06d</doma:length>`, d.OutputLength)
	style := d.OutputStyle
	if style == "" {
		style = "00"
	}
	b.WriteString(xmlEl("doma:style", style))
	b.WriteString(xmlEl("doma:conversionExit", strings.ToUpper(d.ConversionExit)))
	fmt.Fprintf(&b, `<doma:signExists>%t</doma:signExists><doma:lowercase>%t</doma:lowercase><doma:ampmFormat>%t</doma:ampmFormat>`,
		d.SignExists, d.Lowercase, d.AMPMFormat)
	b.WriteString(`</doma:outputInformation><doma:valueInformation>`)
	if d.ValueTable == "" {
		b.WriteString(`<doma:valueTableRef/>`)
	} else {
		vt := strings.ToUpper(d.ValueTable)
		fmt.Fprintf(&b, `<doma:valueTableRef adtcore:uri="/sap/bc/adt/ddic/tables/%s" adtcore:type="TABL/DT" adtcore:name="%s"/>`,
			url.PathEscape(strings.ToLower(vt)), escapeXML(vt))
	}
	fmt.Fprintf(&b, `<doma:appendExists>%t</doma:appendExists>`, d.AppendExists)
	if len(d.FixValues) == 0 {
		b.WriteString(`<doma:fixValues/>`)
	} else {
		b.WriteString(`<doma:fixValues>`)
		for i, v := range d.FixValues {
			pos := v.Position
			if pos == 0 {
				pos = i + 1
			}
			fmt.Fprintf(&b, `<doma:fixValue><doma:position>%04d</doma:position>%s%s%s</doma:fixValue>`,
				pos, xmlEl("doma:low", v.Low), xmlEl("doma:high", v.High), xmlEl("doma:text", v.Text))
		}
		b.WriteString(`</doma:fixValues>`)
	}
	b.WriteString(`</doma:valueInformation></doma:content></doma:domain>`)
	return []byte(b.String())
}

// dataElementBody is the PUT document: the whole data element.
func dataElementBody(e *DataElement, lang string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(&b, `<blue:wbobj xmlns:blue="%s" %s>`, dtelWbobjNS, ddicRootAttrs(e.ddicHeader, string(ObjectTypeDataElement), lang))
	b.WriteString(ddicPackageRef(e.ddicHeader))
	fmt.Fprintf(&b, `<dtel:dataElement xmlns:dtel="%s">`, dataElementNS)
	b.WriteString(xmlEl("dtel:typeKind", e.TypeKind))
	b.WriteString(xmlEl("dtel:typeName", strings.ToUpper(e.TypeName)))
	b.WriteString(xmlEl("dtel:dataType", strings.ToUpper(e.DataType)))
	fmt.Fprintf(&b, `<dtel:dataTypeLength>%06d</dtel:dataTypeLength><dtel:dataTypeDecimals>%06d</dtel:dataTypeDecimals>`, e.Length, e.Decimals)
	for _, l := range []struct {
		prefix        string
		label         string
		length, limit int
	}{
		{"short", e.ShortLabel, e.ShortLength, maxShortLabel},
		{"medium", e.MediumLabel, e.MediumLength, maxMediumLabel},
		{"long", e.LongLabel, e.LongLength, maxLongLabel},
		{"heading", e.HeadingLabel, e.HeadingLength, maxHeadingLabel},
	} {
		b.WriteString(xmlEl("dtel:"+l.prefix+"FieldLabel", l.label))
		fmt.Fprintf(&b, `<dtel:%sFieldLength>%02d</dtel:%sFieldLength><dtel:%sFieldMaxLength>%d</dtel:%sFieldMaxLength>`,
			l.prefix, l.length, l.prefix, l.prefix, l.limit, l.prefix)
	}
	b.WriteString(xmlEl("dtel:searchHelp", strings.ToUpper(e.SearchHelp)))
	b.WriteString(xmlEl("dtel:searchHelpParameter", strings.ToUpper(e.SearchHelpParameter)))
	b.WriteString(xmlEl("dtel:setGetParameter", strings.ToUpper(e.SetGetParameter)))
	b.WriteString(xmlEl("dtel:defaultComponentName", strings.ToUpper(e.DefaultComponentName)))
	fmt.Fprintf(&b, `<dtel:deactivateInputHistory>%t</dtel:deactivateInputHistory><dtel:changeDocument>%t</dtel:changeDocument>`+
		`<dtel:leftToRightDirection>%t</dtel:leftToRightDirection><dtel:deactivateBIDIFiltering>%t</dtel:deactivateBIDIFiltering>`,
		e.DeactivateInputHistory, e.ChangeDocument, e.LeftToRightDirection, e.DeactivateBIDIFiltering)
	b.WriteString(`</dtel:dataElement></blue:wbobj>`)
	return []byte(b.String())
}

// Check refuses what the dictionary would refuse only at activation, or
// silently cut: texts longer than their fields, a length outside its type.
func (d *Domain) Check() error {
	if strings.TrimSpace(d.DataType) == "" {
		return fmt.Errorf("domain %s: no data type", d.Name)
	}
	if d.Length < 0 || d.Decimals < 0 || d.OutputLength < 0 {
		return fmt.Errorf("domain %s: negative length or decimals", d.Name)
	}
	if n := len([]rune(d.Description)); n > 60 {
		return fmt.Errorf("domain %s: short text of %d characters, at most 60", d.Name, n)
	}
	for _, v := range d.FixValues {
		if len([]rune(v.Low)) > 10 || len([]rune(v.High)) > 10 {
			return fmt.Errorf("domain %s: fixed value %q..%q longer than 10 characters", d.Name, v.Low, v.High)
		}
		if len([]rune(v.Text)) > 60 {
			return fmt.Errorf("domain %s: text of fixed value %q longer than 60 characters", d.Name, v.Low)
		}
	}
	return nil
}

// Check refuses a data element the dictionary would refuse or cut.
func (e *DataElement) Check() error {
	switch e.TypeKind {
	case TypeKindDomain, TypeKindRefToDictionary, TypeKindRefToClassOrIntf:
		if strings.TrimSpace(e.TypeName) == "" {
			return fmt.Errorf("data element %s: type kind %s needs a type name", e.Name, e.TypeKind)
		}
	case TypeKindPredefined, TypeKindRefToPredefined:
		if strings.TrimSpace(e.DataType) == "" {
			return fmt.Errorf("data element %s: type kind %s needs a data type", e.Name, e.TypeKind)
		}
	default:
		return fmt.Errorf("data element %s: type kind %q: want %s, %s, %s, %s or %s", e.Name, e.TypeKind,
			TypeKindDomain, TypeKindPredefined, TypeKindRefToPredefined, TypeKindRefToDictionary, TypeKindRefToClassOrIntf)
	}
	if n := len([]rune(e.Description)); n > 60 {
		return fmt.Errorf("data element %s: short text of %d characters, at most 60", e.Name, n)
	}
	for _, l := range []struct {
		what          string
		label         string
		length, limit int
	}{
		{"short", e.ShortLabel, e.ShortLength, maxShortLabel},
		{"medium", e.MediumLabel, e.MediumLength, maxMediumLabel},
		{"long", e.LongLabel, e.LongLength, maxLongLabel},
		{"heading", e.HeadingLabel, e.HeadingLength, maxHeadingLabel},
	} {
		if n := len([]rune(l.label)); n > l.limit {
			return fmt.Errorf("data element %s: %s label of %d characters, at most %d", e.Name, l.what, n, l.limit)
		}
		if l.length > l.limit {
			return fmt.Errorf("data element %s: %s label length %d, at most %d", e.Name, l.what, l.length, l.limit)
		}
	}
	return nil
}

// fillLabelLengths gives a label without a length the length of its field,
// as SE11 does.
func (e *DataElement) fillLabelLengths() {
	for _, l := range []struct {
		label  string
		length *int
		limit  int
	}{
		{e.ShortLabel, &e.ShortLength, maxShortLabel},
		{e.MediumLabel, &e.MediumLength, maxMediumLabel},
		{e.LongLabel, &e.LongLength, maxLongLabel},
		{e.HeadingLabel, &e.HeadingLength, maxHeadingLabel},
	} {
		if l.label != "" && *l.length == 0 {
			*l.length = l.limit
		}
	}
}

// relengthChangedLabels gives a label that changed while its length did not
// its field's full length: the old length was the old label's, and a longer
// text would be cut to it.
func (e *DataElement) relengthChangedLabels(before *DataElement) {
	for _, l := range []struct {
		label, old string
		length     *int
		oldLength  int
	}{
		{e.ShortLabel, before.ShortLabel, &e.ShortLength, before.ShortLength},
		{e.MediumLabel, before.MediumLabel, &e.MediumLength, before.MediumLength},
		{e.LongLabel, before.LongLabel, &e.LongLength, before.LongLength},
		{e.HeadingLabel, before.HeadingLabel, &e.HeadingLength, before.HeadingLength},
	} {
		if l.label != l.old && *l.length == l.oldLength {
			*l.length = 0
		}
	}
}

// DDICWrite is what a write of a domain or data element did.
type DDICWrite struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Created    bool              `json:"created,omitempty"`
	Transport  string            `json:"transport,omitempty"`
	Activation *ActivationResult `json:"activation,omitempty"`
	Notes      []string          `json:"notes,omitempty"`
}

// CreateDomain creates a domain and activates it. d.Package and
// d.Description are required.
func (c *Client) CreateDomain(ctx context.Context, d *Domain, transport string) (*DDICWrite, error) {
	d.Name = strings.ToUpper(strings.TrimSpace(d.Name))
	if d.OutputLength == 0 {
		d.OutputLength = d.Length
	}
	if err := d.Check(); err != nil {
		return nil, err
	}
	w, err := c.createDDIC(ctx, ObjectTypeDomain, d.ddicHeader, transport)
	if err != nil {
		return w, err
	}
	err = c.writeDDIC(ctx, w, domainURL(d.Name), func(cur []byte) ([]byte, error) {
		return domainBody(d, c.config.Language), nil
	}, domainAccept)
	return w, err
}

// UpdateDomain reads a domain, lets change edit it and writes it back
// whole, then activates it.
func (c *Client) UpdateDomain(ctx context.Context, name, transport string, change func(*Domain) error) (*DDICWrite, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	w := &DDICWrite{Type: string(ObjectTypeDomain), Name: name, Transport: transport}
	if err := c.checkMutation(ctx, MutationContext{Op: OpUpdate, OpName: "UpdateDomain", ObjectURL: domainURL(name), Transport: transport}); err != nil {
		return nil, err
	}
	err := c.writeDDIC(ctx, w, domainURL(name), func(cur []byte) ([]byte, error) {
		d, err := parseDomain(cur)
		if err != nil {
			return nil, fmt.Errorf("parsing domain %s: %w", name, err)
		}
		d.Name = name
		if err := change(d); err != nil {
			return nil, err
		}
		if err := d.Check(); err != nil {
			return nil, err
		}
		return domainBody(d, c.config.Language), nil
	}, domainAccept)
	return w, err
}

// CreateDataElement creates a data element and activates it. e.Package and
// e.Description are required; a label without a length gets its field's.
func (c *Client) CreateDataElement(ctx context.Context, e *DataElement, transport string) (*DDICWrite, error) {
	e.Name = strings.ToUpper(strings.TrimSpace(e.Name))
	e.fillLabelLengths()
	if err := e.Check(); err != nil {
		return nil, err
	}
	w, err := c.createDDIC(ctx, ObjectTypeDataElement, e.ddicHeader, transport)
	if err != nil {
		return w, err
	}
	err = c.writeDDIC(ctx, w, dataElementURL(e.Name), func(cur []byte) ([]byte, error) {
		return dataElementBody(e, c.config.Language), nil
	}, dataElementAccept)
	return w, err
}

// UpdateDataElement reads a data element, lets change edit it and writes
// it back whole, then activates it.
func (c *Client) UpdateDataElement(ctx context.Context, name, transport string, change func(*DataElement) error) (*DDICWrite, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	w := &DDICWrite{Type: string(ObjectTypeDataElement), Name: name, Transport: transport}
	if err := c.checkMutation(ctx, MutationContext{Op: OpUpdate, OpName: "UpdateDataElement", ObjectURL: dataElementURL(name), Transport: transport}); err != nil {
		return nil, err
	}
	err := c.writeDDIC(ctx, w, dataElementURL(name), func(cur []byte) ([]byte, error) {
		e, err := parseDataElement(cur)
		if err != nil {
			return nil, fmt.Errorf("parsing data element %s: %w", name, err)
		}
		e.Name = name
		before := *e
		if err := change(e); err != nil {
			return nil, err
		}
		e.relengthChangedLabels(&before)
		e.fillLabelLengths()
		if err := e.Check(); err != nil {
			return nil, err
		}
		return dataElementBody(e, c.config.Language), nil
	}, dataElementAccept)
	return w, err
}

func (c *Client) createDDIC(ctx context.Context, typ CreatableObjectType, h ddicHeader, transport string) (*DDICWrite, error) {
	w := &DDICWrite{Type: string(typ), Name: strings.ToUpper(h.Name), Transport: transport}
	if strings.TrimSpace(h.Package) == "" || strings.TrimSpace(h.Description) == "" {
		return nil, fmt.Errorf("%s %s: package and description are required", typ, w.Name)
	}
	var chosen TransportChoice
	if err := c.CreateObject(ctx, CreateObjectOptions{
		ObjectType:  typ,
		Name:        h.Name,
		Description: h.Description,
		PackageName: h.Package,
		Transport:   transport,
		Chosen:      &chosen,
	}); err != nil {
		return nil, err
	}
	w.Created = true
	if w.Transport == "" {
		w.Transport = chosen.Transport
	}
	if chosen.Reason != "" {
		w.Notes = append(w.Notes, chosen.Reason)
	}
	return w, nil
}

// writeDDIC is the write of a whole document: LOCK, a read under the lock
// that build turns into the new document, PUT in the version the system
// served, UNLOCK, activation. The activation's messages are the answer;
// an activation that fails leaves the object inactive and is an error.
func (c *Client) writeDDIC(ctx context.Context, w *DDICWrite, objectURL string, build func(cur []byte) ([]byte, error), accept string) error {
	plan := c.planTransport(ctx, w.Transport, objectURL, "")
	lock, err := c.LockObject(ctx, objectURL, "MODIFY")
	if err != nil {
		return fmt.Errorf("locking %s %s: %w", w.Type, w.Name, err)
	}
	unlocked := false
	unlock := func() {
		if !unlocked {
			unlocked = true
			_ = c.UnlockObject(context.WithoutCancel(ctx), objectURL, lock.LockHandle)
		}
	}
	defer unlock()
	tr, note, err := c.resolveWriteTransportFor(plan, w.Transport, lock.CorrNr, "Write"+w.Type)
	if err != nil {
		return err
	}
	w.Transport = tr
	if note != "" {
		w.Notes = append(w.Notes, note)
	}

	cur, ct, err := c.readDDIC(ctx, objectURL, accept, true)
	if err != nil {
		return fmt.Errorf("reading %s %s under the lock: %w", w.Type, w.Name, err)
	}
	body, err := build(cur)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Set("lockHandle", lock.LockHandle)
	if w.Transport != "" {
		params.Set("corrNr", w.Transport)
	}
	if _, err := c.transport.Request(ctx, objectURL, &RequestOptions{
		Method:      http.MethodPut,
		Query:       params,
		Body:        body,
		ContentType: ct,
		Accept:      accept,
		Stateful:    true,
	}); err != nil {
		return fmt.Errorf("writing %s %s: %w", w.Type, w.Name, err)
	}
	unlock()

	act, err := c.Activate(ctx, objectURL, w.Name)
	if err != nil {
		return fmt.Errorf("activating %s %s: %w", w.Type, w.Name, err)
	}
	w.Activation = act
	if !act.Success {
		return fmt.Errorf("%s %s is written but not active: %s", w.Type, w.Name, activationSummary(act))
	}
	return nil
}

func activationSummary(act *ActivationResult) string {
	var parts []string
	for _, m := range act.Messages {
		if m.Type == "E" || m.Type == "A" {
			parts = append(parts, m.ShortText)
		}
	}
	if len(parts) == 0 {
		return "the activation reported no error message"
	}
	return strings.Join(parts, "; ")
}
