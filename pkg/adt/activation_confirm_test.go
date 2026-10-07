package adt

import (
	"context"
	"strings"
	"testing"
)

// confirmList is SAP's answer to the preaudit of a class in a transport
// (7.50): an entry with only the request, then the parts recorded with it.
const confirmList = `<?xml version="1.0" encoding="utf-8"?><ioc:inactiveObjects xmlns:ioc="http://www.sap.com/abapxml/inactiveCtsObjects">` +
	`<ioc:entry><ioc:object/><ioc:transport ioc:user="DEV" ioc:linked="false"><ioc:ref adtcore:uri="/sap/bc/adt/vit/wb/object_type/%20%20%20%20rq/object_name/DEVK900001" adtcore:type="/RQ" adtcore:name="DEVK900001" xmlns:adtcore="http://www.sap.com/adt/core"/></ioc:transport></ioc:entry>` +
	`<ioc:entry><ioc:object ioc:user="" ioc:deleted="false"><ioc:ref adtcore:uri="/sap/bc/adt/oo/classes/zcl_demo" adtcore:type="CLAS/OC" adtcore:name="ZCL_DEMO" xmlns:adtcore="http://www.sap.com/adt/core"/></ioc:object><ioc:transport/></ioc:entry>` +
	`<ioc:entry><ioc:object ioc:user="" ioc:deleted="false"><ioc:ref adtcore:uri="/sap/bc/adt/oo/classes/zcl_demo/source/main#type=CLAS%2FOM;name=RUN" adtcore:type="CLAS/OM/public" adtcore:name="ZCL_DEMO                      RUN" adtcore:parentUri="/sap/bc/adt/oo/classes/zcl_demo" xmlns:adtcore="http://www.sap.com/adt/core"/></ioc:object><ioc:transport/></ioc:entry>` +
	`<ioc:entry><ioc:object ioc:user="OTHER" ioc:deleted="false"><ioc:ref adtcore:uri="/sap/bc/adt/oo/classes/zcl_demo/includes/macros" adtcore:type="CLAS/OCN/macros" adtcore:name="ZCL_DEMO" adtcore:parentUri="/sap/bc/adt/oo/classes/zcl_demo" xmlns:adtcore="http://www.sap.com/adt/core"/></ioc:object><ioc:transport/></ioc:entry>` +
	`<ioc:entry><ioc:object ioc:user="" ioc:deleted="false"><ioc:ref adtcore:uri="/sap/bc/adt/oo/classes/zcl_demo_other" adtcore:type="CLAS/OC" adtcore:name="ZCL_DEMO_OTHER" xmlns:adtcore="http://www.sap.com/adt/core"/></ioc:object><ioc:transport/></ioc:entry>` +
	`</ioc:inactiveObjects>`

// The list is sent back without the preaudit, with the class and its own
// parts only: not the request entry, a colleague's part or another class.
func TestActivate_ConfirmsTheListWithOwnParts(t *testing.T) {
	m := &activateHTTP{activate: []string{confirmList, ""}}
	res, err := m.client("dev").Activate(context.Background(), "/sap/bc/adt/oo/classes/zcl_demo", "ZCL_DEMO")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	posts := m.posts()
	if len(posts) != 2 {
		t.Fatalf("activation POSTs = %d, want the preaudit and the confirmation", len(posts))
	}
	confirm := posts[1]
	if !strings.Contains(confirm.query, "preauditRequested=false") {
		t.Errorf("confirmation query = %q, want preauditRequested=false", confirm.query)
	}
	for _, want := range []string{
		`adtcore:uri="/sap/bc/adt/oo/classes/zcl_demo"`,
		`adtcore:name="ZCL_DEMO                      RUN"`,
		`adtcore:parentUri="/sap/bc/adt/oo/classes/zcl_demo"`,
		`adtcore:type="CLAS/OM/public"`,
	} {
		if !strings.Contains(confirm.body, want) {
			t.Errorf("confirmation lacks %s:\n%s", want, confirm.body)
		}
	}
	for _, unwanted := range []string{"macros", "ZCL_DEMO_OTHER", "DEVK900001"} {
		if strings.Contains(confirm.body, unwanted) {
			t.Errorf("confirmation takes %s along:\n%s", unwanted, confirm.body)
		}
	}
	if !res.Success {
		t.Errorf("the confirmation went through, result = %+v", res)
	}
	if len(res.Messages) != 1 || !strings.Contains(res.Messages[0].ShortText, "ZCL_DEMO_OTHER") {
		t.Errorf("messages = %+v, want one naming what was left out", res.Messages)
	}
}

// A list with nothing of the object in it is the answer as it stands.
func TestActivate_ListOfOtherObjectsIsNotConfirmed(t *testing.T) {
	m := &activateHTTP{activate: []string{activationInactiveRoot, ""}}
	res, err := m.client("dev").Activate(context.Background(), "/sap/bc/adt/programs/programs/zdemo", "ZDEMO")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if n := len(m.posts()); n != 1 {
		t.Errorf("activation POSTs = %d, want 1", n)
	}
	if res.Success || len(res.Inactive) != 1 {
		t.Errorf("result = %+v, want the listed refusal", res)
	}
}

// The request-only entry is not an inactive object.
func TestParseActivationResult_SkipsTheRequestEntry(t *testing.T) {
	res, err := parseActivationResult([]byte(confirmList))
	if err != nil {
		t.Fatalf("parseActivationResult: %v", err)
	}
	if len(res.Inactive) != 4 {
		t.Fatalf("inactive = %d, want 4: %+v", len(res.Inactive), res.Inactive)
	}
	if res.Inactive[2].User != "OTHER" {
		t.Errorf("user lost: %+v", res.Inactive[2])
	}
}

// ActivateMultiple confirms the same way, for every object it was given.
func TestActivateMultiple_ConfirmsTheList(t *testing.T) {
	m := &activateHTTP{activate: []string{confirmList, ""}}
	res, err := m.client("dev").ActivateMultiple(context.Background(), []ObjectRef{
		{URI: "/sap/bc/adt/oo/classes/zcl_demo", Name: "ZCL_DEMO"},
		{URI: "/sap/bc/adt/oo/classes/zcl_demo_other", Name: "ZCL_DEMO_OTHER"},
	})
	if err != nil {
		t.Fatalf("ActivateMultiple: %v", err)
	}
	posts := m.posts()
	if len(posts) != 2 || !strings.Contains(posts[1].body, "ZCL_DEMO_OTHER") {
		t.Fatalf("posts = %+v, want a confirmation with both classes", posts)
	}
	if !res.Success {
		t.Errorf("result = %+v", res)
	}
}
