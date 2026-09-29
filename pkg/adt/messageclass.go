package adt

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Message classes over /sap/bc/adt/messageclass (SMESSAGE_ADT). The PUT
// takes the whole class document and reads from it what the editor sends:
// the language the texts go in (adtcore:language, straight into
// T100-SPRSL — not the session's language), the package (packageRef) and
// the short text (adtcore:description, rewritten whenever it differs).
// A body with the messages alone was answered 200 and wrote the texts
// under an empty language, where no logon shows them, and blanked the
// class's short text (7.50, 2026-09-29). 7.40 SP06 takes the language
// from the session instead and saves only when adtcore:masterLanguage
// equals it — any other body is answered 200 and nothing is written; a
// translation cannot be written there at all. Messages the body does not
// name are left as they are; a deletion is its own list,
// <mc:deletedmessages>. A message class has no activation.

// ObjectTypeMessageClass is a message class (SE91).
const ObjectTypeMessageClass CreatableObjectType = "MSAG/N"

const messageClassNS = "http://www.sap.com/adt/MessageClass"

// maxMessageText is the length of T100-TEXT.
const maxMessageText = 73

func init() {
	objectTypes[ObjectTypeMessageClass] = objectTypeInfo{
		creationPath: "/sap/bc/adt/messageclass",
		rootName:     "mc:messageClass",
		namespace:    `xmlns:mc="` + messageClassNS + `"`,
		bodyBuilder:  buildMessageClassBody,
	}
}

// buildMessageClassBody is the create document: the class alone, its
// messages are written after with a lock. The language becomes the
// master language; without it the class is created with none.
func buildMessageClassBody(opts CreateObjectOptions, typeInfo objectTypeInfo, responsible string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<mc:messageClass %s xmlns:adtcore="http://www.sap.com/adt/core"
                 adtcore:name="%s"
                 adtcore:type="%s"
                 adtcore:description="%s"
                 adtcore:language="%s"
                 adtcore:masterLanguage="%s"
                 adtcore:responsible="%s">
  <adtcore:packageRef adtcore:name="%s"/>
</mc:messageClass>`, typeInfo.namespace, escapeXML(opts.Name), ObjectTypeMessageClass,
		escapeXML(opts.Description), escapeXML(strings.ToUpper(opts.Language)), escapeXML(strings.ToUpper(opts.Language)),
		escapeXML(responsible), escapeXML(opts.PackageName))
}

func messageClassURL(name string) string {
	return "/sap/bc/adt/messageclass/" + url.PathEscape(strings.ToLower(name))
}

// readMessageClass reads the class in one language. Stateful keeps the
// read on the session of a lock already taken: a stateless request between
// LOCK and PUT retires the lock handle (#91).
func (c *Client) readMessageClass(ctx context.Context, name, lang string, stateful bool) (*MessageClass, error) {
	resp, err := c.transport.Request(ctx, messageClassURL(name), &RequestOptions{
		Method:           http.MethodGet,
		Accept:           "application/vnd.sap.adt.mc.messageclass+xml",
		OverrideLanguage: lang,
		Stateful:         stateful,
	})
	if err != nil {
		return nil, fmt.Errorf("reading message class %s: %w", name, err)
	}
	var mc MessageClass
	if err := xml.Unmarshal(resp.Body, &mc); err != nil {
		return nil, fmt.Errorf("parsing message class %s: %w", name, err)
	}
	mc.Name = strings.ToUpper(name)
	mc.Package = mc.PackageRef.Name
	return &mc, nil
}

type messageClassWrite struct {
	XMLName      xml.Name            `xml:"mc:messageClass"`
	XMLNSmc      string              `xml:"xmlns:mc,attr"`
	XMLNSadtcore string              `xml:"xmlns:adtcore,attr"`
	Name         string              `xml:"adtcore:name,attr"`
	Type         string              `xml:"adtcore:type,attr"`
	Description  string              `xml:"adtcore:description,attr"`
	Language     string              `xml:"adtcore:language,attr"`
	Master       string              `xml:"adtcore:masterLanguage,attr,omitempty"`
	PackageRef   *messageClassPkgRef `xml:"adtcore:packageRef,omitempty"`
	Messages     []messageWrite      `xml:"mc:messages"`
	Deleted      []messageWrite      `xml:"mc:deletedmessages"`
}

type messageClassPkgRef struct {
	Name string `xml:"adtcore:name,attr"`
}

type messageWrite struct {
	Number          string `xml:"mc:msgno,attr"`
	Text            string `xml:"mc:msgtext,attr,omitempty"`
	SelfExplanatory string `xml:"mc:selfexplainatory,attr,omitempty"`
}

// messageClassBody is the PUT document. cur is the class as read: its
// short text and package go back unchanged, so the write touches only the
// messages named. lang is the ISO language the texts are written in.
func messageClassBody(cur *MessageClass, lang string, msgs []MessageClassMessage, deleted []string) ([]byte, error) {
	w := messageClassWrite{
		XMLNSmc:      messageClassNS,
		XMLNSadtcore: "http://www.sap.com/adt/core",
		Name:         strings.ToUpper(cur.Name),
		Type:         string(ObjectTypeMessageClass),
		Description:  cur.Description,
		Language:     strings.ToUpper(lang),
		Master:       strings.ToUpper(cur.MasterLanguage),
	}
	if cur.Package != "" {
		w.PackageRef = &messageClassPkgRef{Name: cur.Package}
	}
	for _, m := range msgs {
		w.Messages = append(w.Messages, messageWrite{Number: m.Number, Text: m.Text, SelfExplanatory: m.SelfExplanatory})
	}
	for _, n := range deleted {
		w.Deleted = append(w.Deleted, messageWrite{Number: n})
	}
	body, err := xml.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("marshal message class XML: %w", err)
	}
	return append([]byte(xml.Header), body...), nil
}

// MessageNumber normalizes a message number to its three digits: "7" is
// "007". Anything else is refused.
func MessageNumber(s string) (string, error) {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 999 || strings.ContainsAny(s, "+-") {
		return "", fmt.Errorf("message number %q: want 000 to 999", s)
	}
	return fmt.Sprintf("%03d", n), nil
}

// MessageClassEdit is what EditMessageClass did, or would do.
type MessageClassEdit struct {
	Name      string                `json:"name"`
	Language  string                `json:"language"`
	Transport string                `json:"transport,omitempty"`
	Added     []MessageClassMessage `json:"added,omitempty"`
	Changed   []MessageChange       `json:"changed,omitempty"`
	Deleted   []string              `json:"deleted,omitempty"`
	Unchanged []string              `json:"unchanged,omitempty"`
	// Absent are numbers named for deletion that the class does not have.
	Absent  []string `json:"absent,omitempty"`
	Notes   []string `json:"notes,omitempty"`
	Applied bool     `json:"applied"`
}

// MessageChange is one message whose text changes.
type MessageChange struct {
	Number string `json:"number"`
	Old    string `json:"old"`
	New    string `json:"new"`
}

func (e *MessageClassEdit) changes() int { return len(e.Added) + len(e.Changed) + len(e.Deleted) }

// EditMessageClass sets and deletes messages of a class in its master
// language, which must be the logon language — a text in another is a
// translation, WriteMessageClassTexts. set maps a number to its text;
// messages not named keep theirs. Nothing is locked when nothing differs.
func (c *Client) EditMessageClass(ctx context.Context, name string, set map[string]string, del []string, transport string, dryRun bool) (*MessageClassEdit, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	objectURL := messageClassURL(name)
	if err := c.checkMutation(ctx, MutationContext{Op: OpUpdate, OpName: "EditMessageClass", ObjectURL: objectURL, Transport: transport}); err != nil {
		return nil, err
	}
	want := map[string]string{}
	for k, v := range set {
		n, err := MessageNumber(k)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("message %s: empty text; to remove a message, delete it", n)
		}
		if l := utf8.RuneCountInString(v); l > maxMessageText {
			return nil, fmt.Errorf("message %s: %d characters, a message text holds %d", n, l, maxMessageText)
		}
		want[n] = v
	}
	var drop []string
	for _, k := range del {
		n, err := MessageNumber(k)
		if err != nil {
			return nil, err
		}
		if _, ok := want[n]; ok {
			return nil, fmt.Errorf("message %s is both set and deleted", n)
		}
		drop = append(drop, n)
	}
	if len(want)+len(drop) == 0 {
		return nil, fmt.Errorf("nothing to write: give NUMBER=TEXT or a number to delete")
	}
	lang := strings.ToUpper(c.config.Language)
	edit := &MessageClassEdit{Name: name, Language: lang}

	cur, err := c.readMessageClass(ctx, name, "", false)
	if err != nil {
		return nil, err
	}
	if cur.MasterLanguage != "" && !strings.EqualFold(cur.MasterLanguage, lang) {
		return edit, fmt.Errorf("%s's master language is %s, the logon language %s; a text in %s is a translation (WriteMessageClassTexts)", name, cur.MasterLanguage, lang, lang)
	}
	planMessageEdit(edit, cur, want, drop)
	if dryRun || edit.changes() == 0 {
		return edit, nil
	}

	trPlan := c.planTransport(ctx, transport, objectURL, "")
	lock, err := c.LockObject(ctx, objectURL, "MODIFY")
	if err != nil {
		return edit, fmt.Errorf("locking message class %s: %w", name, err)
	}
	defer func() { _ = c.UnlockObject(context.WithoutCancel(ctx), objectURL, lock.LockHandle) }()
	var trNote string
	if edit.Transport, trNote, err = c.resolveWriteTransportFor(trPlan, transport, lock.CorrNr, "EditMessageClass"); err != nil {
		return edit, err
	}
	if trNote != "" {
		edit.Notes = append(edit.Notes, trNote)
	}

	// Read again under the lock, so nothing changed in between unseen.
	if cur, err = c.readMessageClass(ctx, name, "", true); err != nil {
		return edit, err
	}
	*edit = MessageClassEdit{Name: name, Language: lang, Transport: edit.Transport, Notes: edit.Notes}
	planMessageEdit(edit, cur, want, drop)
	if edit.changes() == 0 {
		return edit, nil
	}
	msgs := append([]MessageClassMessage{}, edit.Added...)
	for _, ch := range edit.Changed {
		msgs = append(msgs, MessageClassMessage{Number: ch.Number, Text: ch.New})
	}
	body, err := messageClassBody(cur, lang, msgs, edit.Deleted)
	if err != nil {
		return edit, err
	}
	params := url.Values{}
	params.Set("lockHandle", lock.LockHandle)
	if edit.Transport != "" {
		params.Set("corrNr", edit.Transport)
	}
	if _, err := c.transport.Request(ctx, objectURL, &RequestOptions{
		Method:      http.MethodPut,
		Query:       params,
		Body:        body,
		ContentType: "application/vnd.sap.adt.mc.messageclass+xml",
		Stateful:    true,
	}); err != nil {
		return edit, fmt.Errorf("writing message class %s: %w", name, err)
	}
	edit.Applied = true
	return edit, nil
}

func planMessageEdit(edit *MessageClassEdit, cur *MessageClass, want map[string]string, drop []string) {
	have := map[string]string{}
	for _, m := range cur.Messages {
		have[m.Number] = m.Text
	}
	nums := make([]string, 0, len(want))
	for n := range want {
		nums = append(nums, n)
	}
	sort.Strings(nums)
	for _, n := range nums {
		old, ok := have[n]
		switch {
		case !ok:
			edit.Added = append(edit.Added, MessageClassMessage{Number: n, Text: want[n]})
		case old != want[n]:
			edit.Changed = append(edit.Changed, MessageChange{Number: n, Old: old, New: want[n]})
		default:
			edit.Unchanged = append(edit.Unchanged, n)
		}
	}
	sort.Strings(drop)
	for _, n := range drop {
		if _, ok := have[n]; ok {
			edit.Deleted = append(edit.Deleted, n)
		} else {
			edit.Absent = append(edit.Absent, n)
		}
	}
}
