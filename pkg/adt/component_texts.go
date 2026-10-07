package adt

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/vibing-steampunk/pkg/abaplint"
)

// Component texts: the short texts of the methods, attributes, types and
// events of a class or interface, and of their parameters and exceptions
// (SEOCOMPOTX, SEOSUBCOTX). From 7.50 SAP takes them from ABAP Doc when the
// source is saved:
//
//	"! <p class="shorttext synchronized">Adds two numbers</p>
//	"! @parameter iv_a | <p class="shorttext synchronized">First summand</p>
//	METHODS add IMPORTING iv_a TYPE i.
//
// 7.40 has no such step, so a source written through ADT leaves them empty.
// There the adt-fallback set offers /sap/bc/adt/zvsp/oo/componenttexts in
// discovery, and vsp writes the texts it finds in the source after the
// source itself. Where discovery does not offer it, the system writes them.
//
// 7.40's class check does not know the shorttext tag either: it warns on
// every line that has one. So there plain ABAP Doc is read as well, and is
// what to write:
//
//	"! Adds two numbers
//	"! @parameter iv_a | First summand
//
// The text of a component is the first paragraph of its ABAP Doc, that of a
// parameter or exception the first paragraph after the bar. A shorttext tag
// still wins over the plain text of the same section.

// componentTextsPath is the stand-in's collection. No release has a
// standard one: the set offers it in discovery only where saving the source
// does not write the texts.
const componentTextsPath = FallbackRoot + "/oo/componenttexts"

// componentTextMax is the length of SEOCOMPOTX-DESCRIPT and SEOSUBCOTX-DESCRIPT.
const componentTextMax = 60

// ComponentText is one short text: of a component, or of a parameter or
// exception of it when Subcomponent is set.
type ComponentText struct {
	Component    string `json:"component"`
	Subcomponent string `json:"subcomponent,omitempty"`
	Text         string `json:"text"`
}

func (t ComponentText) key() string {
	if t.Subcomponent == "" {
		return t.Component
	}
	return t.Component + "." + t.Subcomponent
}

// ComponentTextsResult says what became of the ABAP Doc short texts of a
// source written to a release that does not take them itself.
type ComponentTextsResult struct {
	Written int `json:"written"`
	// Skipped names texts that were not sent, and why.
	Skipped []string `json:"skipped,omitempty"`
	// Error is set when the texts were not written; the source was.
	Error string `json:"error,omitempty"`
	// Note points out what the release makes of the source, written or not.
	Note string `json:"note,omitempty"`
}

// componentKeywords are the declarations whose names are components.
var componentKeywords = map[string]bool{
	"DATA": true, "CLASS-DATA": true, "CONSTANTS": true, "TYPES": true,
	"METHODS": true, "CLASS-METHODS": true, "EVENTS": true, "CLASS-EVENTS": true,
}

// structureKeywords may open a BEGIN OF ... END OF block, whose fields are
// not components.
var structureKeywords = map[string]bool{
	"DATA": true, "CLASS-DATA": true, "CONSTANTS": true, "TYPES": true,
}

var (
	abapDocTagRe   = regexp.MustCompile(`(?im)^\s*@(parameter|raising|exception)\s+!?([A-Za-z0-9_/]+)\s*\|`)
	abapDocShortRe = regexp.MustCompile(`(?is)<p\s+class\s*=\s*"shorttext\s+synchronized"[^>]*>(.*?)</p>`)
	whitespaceRe   = regexp.MustCompile(`\s+`)
	htmlTagRe      = regexp.MustCompile(`<[^>]*>`)
)

// docSegment is one element of a declaration: the tokens of `name TYPE i`
// in `DATA: a TYPE i, name TYPE i.`, with the ABAP Doc lines before it.
type docSegment struct {
	doc    []string
	tokens []abaplint.Token
}

// ParseComponentTexts returns the synchronized ABAP Doc short texts of the
// components of the global class or interface name declares in source.
// The text of the class or interface itself is not among them: that is the
// object's description.
func ParseComponentTexts(source, name string) []ComponentText {
	return parseComponentTexts(source, name, false)
}

// ParsePlainComponentTexts is ParseComponentTexts that also reads plain
// ABAP Doc, for a release that does not know the shorttext tag.
func ParsePlainComponentTexts(source, name string) []ComponentText {
	return parseComponentTexts(source, name, true)
}

func parseComponentTexts(source, name string, plain bool) []ComponentText {
	name = strings.ToUpper(name)
	tokens := (&abaplint.Lexer{}).Run(source)

	var texts []ComponentText
	inDef := false
	depth := 0 // BEGIN OF nesting of the statement chain being read
	var doc []string
	var stmt []docSegment
	parens := 0

	flush := func() {
		defer func() { stmt = nil; parens = 0 }()
		if len(stmt) == 0 || len(stmt[0].tokens) == 0 {
			return
		}
		words := keywordOf(stmt[0].tokens)
		if !inDef {
			inDef = opensDefinition(stmt[0].tokens, name)
			return
		}
		if words.keyword == "ENDCLASS" || words.keyword == "ENDINTERFACE" {
			inDef = false
			return
		}
		if !componentKeywords[words.keyword] {
			return
		}
		var elements []docSegment
		if len(stmt) > 1 || len(stmt[0].tokens) > words.length {
			// `METHODS: a, b.` has the keyword alone in the first segment;
			// `METHODS a.` has the element after it.
			first := docSegment{doc: stmt[0].doc, tokens: stmt[0].tokens[words.length:]}
			if len(first.tokens) == 0 {
				elements = stmt[1:]
				if len(elements) > 0 && len(elements[0].doc) == 0 {
					elements[0].doc = stmt[0].doc
				}
			} else {
				elements = append([]docSegment{first}, stmt[1:]...)
			}
		}
		for _, el := range elements {
			cmp, ok := elementName(el.tokens, words.keyword, &depth)
			if !ok {
				continue
			}
			texts = append(texts, docTexts(cmp, el.doc, plain)...)
		}
	}

	for _, tok := range tokens {
		if tok.Type == abaplint.TokenComment {
			if strings.HasPrefix(tok.Str, `"!`) {
				doc = append(doc, strings.TrimPrefix(tok.Str, `"!`))
			} else {
				doc = nil
			}
			continue
		}
		if len(stmt) == 0 {
			stmt = []docSegment{{}}
		}
		cur := &stmt[len(stmt)-1]
		switch {
		case tok.Type == abaplint.TokenPunctuation && tok.Str == ".":
			flush()
			doc = nil
			continue
		case tok.Str == ":" && len(stmt) == 1: // the lexer gives the colon as an identifier
			stmt = append(stmt, docSegment{})
			continue
		case tok.Type == abaplint.TokenPunctuation && tok.Str == "," && parens == 0 && len(stmt) > 1:
			stmt = append(stmt, docSegment{})
			continue
		}
		if isParenLeft(tok.Type) {
			parens++
		} else if isParenRight(tok.Type) && parens > 0 {
			parens--
		}
		// ABAP Doc stands before the statement, or before the element in a
		// chain or after the keyword: METHODS "! ... name.
		if len(cur.doc) == 0 && len(doc) > 0 {
			cur.doc = doc
		}
		doc = nil
		cur.tokens = append(cur.tokens, tok)
	}
	return texts
}

type statementKeyword struct {
	keyword string
	length  int // tokens it takes: CLASS-DATA is CLASS, -, DATA
}

func keywordOf(tokens []abaplint.Token) statementKeyword {
	first := strings.ToUpper(tokens[0].Str)
	if first == "CLASS" && len(tokens) > 2 && isDash(tokens[1].Type) {
		return statementKeyword{"CLASS-" + strings.ToUpper(tokens[2].Str), 3}
	}
	return statementKeyword{first, 1}
}

// opensDefinition tells whether a statement is `CLASS name DEFINITION` or
// `INTERFACE name`, other than a DEFERRED or LOAD one.
func opensDefinition(tokens []abaplint.Token, name string) bool {
	if len(tokens) < 2 || strings.ToUpper(tokens[1].Str) != name {
		return false
	}
	switch strings.ToUpper(tokens[0].Str) {
	case "CLASS":
		if len(tokens) < 3 || strings.ToUpper(tokens[2].Str) != "DEFINITION" {
			return false
		}
	case "INTERFACE":
	default:
		return false
	}
	for _, t := range tokens[2:] {
		switch strings.ToUpper(t.Str) {
		case "DEFERRED", "LOAD":
			return false
		}
	}
	return true
}

// elementName returns the component an element declares, keeping track of
// BEGIN OF ... END OF blocks, whose fields are not components.
func elementName(tokens []abaplint.Token, keyword string, depth *int) (string, bool) {
	if len(tokens) == 0 {
		return "", false
	}
	word := func(i int) string {
		if i < len(tokens) {
			return strings.ToUpper(tokens[i].Str)
		}
		return ""
	}
	if structureKeywords[keyword] {
		switch {
		case word(0) == "BEGIN" && word(1) == "OF":
			i := 2
			if word(i) == "ENUM" || word(i) == "MESH" {
				i++
			}
			*depth++
			if *depth > 1 {
				return "", false
			}
			return componentName(word(i))
		case word(0) == "END" && word(1) == "OF":
			if *depth > 0 {
				*depth--
			}
			return "", false
		case *depth > 0:
			return "", false
		}
	}
	for _, t := range tokens[1:] {
		if strings.EqualFold(t.Str, "REDEFINITION") {
			// A redefined method has its text in the class that declares it.
			return "", false
		}
	}
	return componentName(word(0))
}

func componentName(w string) (string, bool) {
	w = strings.TrimPrefix(w, "!")
	if w == "" || strings.Contains(w, "~") {
		return "", false
	}
	return w, true
}

// docTexts reads the short texts of one component from its ABAP Doc: its
// own, and those of @parameter, @raising and @exception. Without plain only
// the synchronized ones.
func docTexts(component string, doc []string, plain bool) []ComponentText {
	if len(doc) == 0 {
		return nil
	}
	body := strings.Join(doc, "\n")
	var texts []ComponentText
	tags := abapDocTagRe.FindAllStringSubmatchIndex(body, -1)
	head := body
	if len(tags) > 0 {
		head = body[:tags[0][0]]
	}
	if text, ok := sectionText(head, plain); ok {
		texts = append(texts, ComponentText{Component: component, Text: text})
	}
	for i, m := range tags {
		end := len(body)
		if i+1 < len(tags) {
			end = tags[i+1][0]
		}
		if text, ok := sectionText(body[m[1]:end], plain); ok {
			texts = append(texts, ComponentText{
				Component:    component,
				Subcomponent: strings.ToUpper(body[m[4]:m[5]]),
				Text:         text,
			})
		}
	}
	return texts
}

// sectionText is the shorttext of a section, or with plain, failing that,
// its first paragraph.
func sectionText(section string, plain bool) (string, bool) {
	if text, ok := shortText(section); ok || !plain {
		return text, ok
	}
	return plainText(section)
}

// plainText is the first paragraph of a section: its lines up to an empty
// one or one that starts another tag (@see and the like), without markup.
func plainText(section string) (string, bool) {
	var lines []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "@") {
			break
		}
		lines = append(lines, line)
	}
	text := html.UnescapeString(htmlTagRe.ReplaceAllString(strings.Join(lines, " "), " "))
	text = strings.TrimSpace(whitespaceRe.ReplaceAllString(text, " "))
	return text, text != ""
}

func shortText(section string) (string, bool) {
	m := abapDocShortRe.FindStringSubmatch(section)
	if m == nil {
		return "", false
	}
	text := html.UnescapeString(whitespaceRe.ReplaceAllString(m[1], " "))
	return strings.TrimSpace(text), true
}

func componentTextsURL(name string) string {
	return componentTextsPath + "/" + url.PathEscape(strings.ToUpper(name))
}

// componentTextsForced is VSP_COMPONENT_TEXTS=force: the texts of a source
// go to the stand-in even where the system writes them itself, to try it.
func componentTextsForced() bool {
	return strings.EqualFold(os.Getenv("VSP_COMPONENT_TEXTS"), "force")
}

// componentTextsOffered tells whether discovery offers the stand-in, i.e.
// the system does not take the texts from the source.
func (c *Client) componentTextsOffered(ctx context.Context) bool {
	return c.FallbackPath(ctx, adtRoot+"/oo/componenttexts") != adtRoot+"/oo/componenttexts"
}

// GetComponentTexts reads the component texts of a class or interface in
// its master language through the stand-in.
func (c *Client) GetComponentTexts(ctx context.Context, name string) ([]ComponentText, error) {
	resp, err := c.transport.Request(ctx, componentTextsURL(name), &RequestOptions{
		Method: http.MethodGet,
		Accept: "text/plain",
	})
	if err != nil {
		return nil, err
	}
	return parseComponentTextLines(string(resp.Body)), nil
}

func parseComponentTextLines(body string) []ComponentText {
	var texts []ComponentText
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		f := strings.SplitN(line, "\t", 3)
		if len(f) < 3 || f[0] == "" {
			continue
		}
		texts = append(texts, ComponentText{Component: f[0], Subcomponent: f[1], Text: f[2]})
	}
	return texts
}

// SetComponentTexts writes component texts of a class or interface through
// the stand-in: each sets one text, an empty one deletes it, the rest stay.
// SAP checks them all before writing any.
func (c *Client) SetComponentTexts(ctx context.Context, name string, texts []ComponentText, transport string) error {
	if err := c.checkSafety(OpUpdate, "SetComponentTexts"); err != nil {
		return err
	}
	var body strings.Builder
	for _, t := range texts {
		fmt.Fprintf(&body, "%s\t%s\t%s\n", t.Component, t.Subcomponent, strings.ReplaceAll(t.Text, "\t", " "))
	}
	objectURL := componentTextsURL(name)
	lock, err := c.LockObject(ctx, objectURL, "MODIFY", transport)
	if err != nil {
		return fmt.Errorf("locking the texts of %s: %w", name, err)
	}
	defer func() {
		_ = c.UnlockObject(context.WithoutCancel(ctx), objectURL, lock.LockHandle)
	}()
	params := url.Values{}
	params.Set("lockHandle", lock.LockHandle)
	if transport != "" {
		params.Set("corrNr", transport)
	}
	if _, err := c.transport.Request(ctx, objectURL, &RequestOptions{
		Method:      http.MethodPut,
		Query:       params,
		Body:        []byte(body.String()),
		ContentType: "text/plain; charset=utf-8",
		Accept:      "text/plain",
		Stateful:    true,
	}); err != nil {
		return fmt.Errorf("writing the texts of %s: %w", name, err)
	}
	return nil
}

// SyncComponentTexts writes the ABAP Doc short texts of a class or
// interface source just written, where the system does not. It returns nil
// when there was nothing to do. WriteSource and DeployFromFile call it
// themselves; WriteClass, which WriteSource builds on, does not.
func (c *Client) SyncComponentTexts(ctx context.Context, objectType, name, source, transport string) *ComponentTextsResult {
	objectType, _, _ = strings.Cut(strings.ToUpper(objectType), "/") // CLAS/OC
	if objectType != "CLAS" && objectType != "INTF" {
		return nil
	}
	tagged := ParseComponentTexts(source, name)
	texts := ParsePlainComponentTexts(source, name)
	if len(texts) == 0 {
		return nil
	}
	offered := c.componentTextsOffered(ctx)
	if !offered && !componentTextsForced() {
		texts = tagged
		if len(texts) == 0 {
			return nil
		}
		// A set that has the feature offers it wherever the system does not
		// write the texts; an older set (or none) says nothing either way.
		if c.fallbackHasFeature(ctx, "/oo/componenttexts") {
			return nil
		}
		if old, release := c.releaseBefore750(ctx); old {
			return &ComponentTextsResult{Error: fmt.Sprintf(
				"%d ABAP Doc short texts not written: release %s does not take them from the source, "+
					"and the adt-fallback set (0.8.0 or later: vsp install adt-fallback) is not installed",
				len(texts), release)}
		}
		return nil
	}
	result := &ComponentTextsResult{}
	if offered && len(tagged) > 0 {
		result.Note = "this release's class check warns on every <p class=\"shorttext synchronized\"> line: " +
			"write plain ABAP Doc instead (\"! Text, \"! @parameter name | Text), which is read the same way here"
	}
	send := texts[:0:0]
	seen := map[string]bool{}
	for _, t := range texts {
		switch {
		case len([]rune(t.Text)) > componentTextMax:
			result.Skipped = append(result.Skipped, fmt.Sprintf("%s: longer than %d characters", t.key(), componentTextMax))
		case t.Text == "":
			result.Skipped = append(result.Skipped, t.key()+": empty")
		case seen[t.key()]:
			result.Skipped = append(result.Skipped, t.key()+": given twice")
		default:
			seen[t.key()] = true
			send = append(send, t)
		}
	}
	sort.Strings(result.Skipped)
	if len(send) == 0 {
		return result
	}
	if err := c.SetComponentTexts(ctx, name, send, transport); err != nil {
		result.Error = err.Error()
		return result
	}
	result.Written = len(send)
	return result
}

// fallbackHasFeature tells whether the installed adt-fallback set has a
// feature, offered in discovery or not, from GET /info:
// "features=/textelements,/oo/componenttexts".
func (c *Client) fallbackHasFeature(ctx context.Context, path string) bool {
	if !c.FallbackInstalled(ctx) {
		return false
	}
	resp, err := c.transport.Request(ctx, FallbackRoot+"/info", &RequestOptions{
		Method: http.MethodGet,
		Accept: "text/plain",
	})
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(resp.Body), "\n") {
		if features, ok := strings.CutPrefix(strings.TrimSpace(line), "features="); ok {
			for _, f := range strings.Split(features, ",") {
				if strings.EqualFold(strings.TrimSpace(f), path) {
					return true
				}
			}
		}
	}
	return false
}

// releaseBefore750 tells whether the system is known to be older than 7.50.
func (c *Client) releaseBefore750(ctx context.Context) (bool, string) {
	info, err := c.GetSystemInfo(ctx)
	if err != nil || info == nil {
		return false, ""
	}
	n, err := strconv.Atoi(strings.TrimSpace(info.SAPRelease))
	return err == nil && n < 750, info.SAPRelease
}

func isParenLeft(t abaplint.TokenType) bool {
	return t == abaplint.TokenParenLeft || t == abaplint.TokenParenLeftW ||
		t == abaplint.TokenWParenLeft || t == abaplint.TokenWParenLeftW
}

func isParenRight(t abaplint.TokenType) bool {
	return t == abaplint.TokenParenRight || t == abaplint.TokenParenRightW ||
		t == abaplint.TokenWParenRight || t == abaplint.TokenWParenRightW
}

func isDash(t abaplint.TokenType) bool {
	return t == abaplint.TokenDash
}
