package adt

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const testMessageClassXML = `<?xml version="1.0" encoding="utf-8"?><mc:messageClass adtcore:masterLanguage="RU" adtcore:name="ZDEMO_MC" adtcore:type="MSAG/N" adtcore:description="Demo messages" adtcore:language="RU" xmlns:mc="http://www.sap.com/adt/MessageClass" xmlns:adtcore="http://www.sap.com/adt/core">` +
	`<adtcore:packageRef adtcore:uri="/sap/bc/adt/vit/wb/object_type/devck/object_name/ZDEMO" adtcore:type="DEVC/K" adtcore:name="ZDEMO"/>` +
	`<mc:messages mc:msgno="001" mc:msgtext="First" mc:selfexplainatory="true" mc:documented="false" adtcore:name=""/>` +
	`<mc:messages mc:msgno="002" mc:msgtext="Second" mc:selfexplainatory="true" mc:documented="false" adtcore:name=""/>` +
	`</mc:messageClass>`

// The PUT takes the language of the texts, the package and the short text
// from the body. Without them SAP answered 200, wrote T100 under an empty
// language and blanked the short text (7.50).
func TestMessageClassBodyCarriesWhatThePutReads(t *testing.T) {
	cur := &MessageClass{Name: "zdemo_mc", Description: "Demo & messages", Package: "ZDEMO", MasterLanguage: "RU"}
	body, err := messageClassBody(cur, "ru", []MessageClassMessage{{Number: "003", Text: "Third <&1>"}}, []string{"002"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		`adtcore:name="ZDEMO_MC"`,
		`adtcore:language="RU"`,
		`adtcore:masterLanguage="RU"`, // 7.40 saves nothing without it
		`adtcore:description="Demo &amp; messages"`,
		`<adtcore:packageRef adtcore:name="ZDEMO">`,
		`<mc:messages mc:msgno="003" mc:msgtext="Third &lt;&amp;1&gt;">`,
		`<mc:deletedmessages mc:msgno="002">`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("body lacks %s:\n%s", want, s)
		}
	}
}

func TestReadMessageClassKeepsPackageAndLanguages(t *testing.T) {
	rec := &adtRecorder{}
	client := newStubbedClient(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, testMessageClassXML)
	})
	mc, err := client.GetMessageClass(context.Background(), "ZDEMO_MC")
	if err != nil {
		t.Fatal(err)
	}
	if mc.Package != "ZDEMO" || mc.MasterLanguage != "RU" || mc.Description != "Demo messages" || len(mc.Messages) != 2 {
		t.Errorf("read %+v", mc)
	}
}

func TestEditMessageClassWritesOnlyWhatDiffers(t *testing.T) {
	rec := &adtRecorder{}
	var mu sync.Mutex
	var put string
	client := newStubbedClient(t, rec, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = io.WriteString(w, testLockXML)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/messageclass/"):
			_, _ = io.WriteString(w, testMessageClassXML)
		case r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			put = string(b)
			mu.Unlock()
		default:
			w.WriteHeader(http.StatusOK)
		}
	}, WithLanguage("RU"))

	edit, err := client.EditMessageClass(context.Background(), "zdemo_mc",
		map[string]string{"1": "First", "002": "Second, changed", "3": "Third &1"}, []string{"4"}, "", false)
	if err != nil {
		t.Fatalf("EditMessageClass: %v", err)
	}
	if !edit.Applied || len(edit.Added) != 1 || edit.Added[0].Number != "003" ||
		len(edit.Changed) != 1 || edit.Changed[0].Old != "Second" ||
		len(edit.Unchanged) != 1 || len(edit.Absent) != 1 || edit.Absent[0] != "004" {
		t.Errorf("edit = %+v", edit)
	}
	for _, want := range []string{`adtcore:language="RU"`, `adtcore:description="Demo messages"`, `adtcore:name="ZDEMO"`,
		`mc:msgno="002" mc:msgtext="Second, changed"`, `mc:msgno="003"`} {
		if !strings.Contains(put, want) {
			t.Errorf("PUT lacks %s:\n%s", want, put)
		}
	}
	if strings.Contains(put, `mc:msgno="001"`) || strings.Contains(put, "deletedmessages") {
		t.Errorf("PUT names what does not change:\n%s", put)
	}

	calls := rec.snapshot()
	lockAt := indexOfCall(calls, isLock)
	putAt := indexOfCall(calls, func(c wireCall) bool { return c.method == http.MethodPut })
	if lockAt < 0 || putAt < lockAt || indexOfCall(calls, isUnlock) < putAt {
		t.Fatalf("want LOCK, PUT, UNLOCK in that order")
	}
	assertWindowStateful(t, calls, lockAt, putAt+1)
}

func TestEditMessageClassDeletes(t *testing.T) {
	rec := &adtRecorder{}
	var put string
	client := newStubbedClient(t, rec, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = io.WriteString(w, testLockXML)
		case r.Method == http.MethodGet:
			_, _ = io.WriteString(w, testMessageClassXML)
		case r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			put = string(b)
		}
	}, WithLanguage("RU"))
	if _, err := client.EditMessageClass(context.Background(), "ZDEMO_MC", nil, []string{"001"}, "", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(put, `<mc:deletedmessages mc:msgno="001">`) || strings.Contains(put, "<mc:messages ") {
		t.Errorf("PUT:\n%s", put)
	}
}

func TestEditMessageClassRefusesAnotherLanguageAndLocksNothing(t *testing.T) {
	rec := &adtRecorder{}
	client := newStubbedClient(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, testMessageClassXML)
	}, WithLanguage("EN"))
	if _, err := client.EditMessageClass(context.Background(), "ZDEMO_MC", map[string]string{"001": "x"}, nil, "", false); err == nil ||
		!strings.Contains(err.Error(), "translation") {
		t.Errorf("want a refusal naming a translation, got %v", err)
	}
	if indexOfCall(rec.snapshot(), isLock) >= 0 {
		t.Error("a refused edit took a lock")
	}
}

func TestEditMessageClassRefusesBadInput(t *testing.T) {
	client := newStubbedClient(t, &adtRecorder{}, func(w http.ResponseWriter, r *http.Request) {}, WithLanguage("RU"))
	for name, tc := range map[string]struct {
		set map[string]string
		del []string
	}{
		"number":        {set: map[string]string{"1000": "x"}},
		"not a number":  {set: map[string]string{"A1": "x"}},
		"empty text":    {set: map[string]string{"001": " "}},
		"too long":      {set: map[string]string{"001": strings.Repeat("я", 74)}},
		"set and drop":  {set: map[string]string{"001": "x"}, del: []string{"1"}},
		"nothing asked": {},
	} {
		if _, err := client.EditMessageClass(context.Background(), "ZDEMO_MC", tc.set, tc.del, "", true); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestMessageClassCreateBodyNamesLanguageAndPackage(t *testing.T) {
	body := buildCreateObjectBody(CreateObjectOptions{ObjectType: ObjectTypeMessageClass, Name: "ZDEMO_MC",
		Description: "Demo", PackageName: "ZDEMO", Language: "ru"}, objectTypes[ObjectTypeMessageClass], "TESTUSER")
	for _, want := range []string{`adtcore:type="MSAG/N"`, `adtcore:language="RU"`, `adtcore:masterLanguage="RU"`,
		`<adtcore:packageRef adtcore:name="ZDEMO"/>`, `xmlns:mc="http://www.sap.com/adt/MessageClass"`} {
		if !strings.Contains(body, want) {
			t.Errorf("create body lacks %s:\n%s", want, body)
		}
	}
	if got := GetObjectURL(ObjectTypeMessageClass, "ZDEMO_MC", ""); got != "/sap/bc/adt/messageclass/zdemo_mc" {
		t.Errorf("GetObjectURL = %s", got)
	}
}
