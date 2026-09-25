package adt

import (
	"reflect"
	"testing"
)

// An abridged answer of ERH (7.50) to GET /sap/bc/adt/system/components.
const componentsFeed = `<?xml version="1.0" encoding="utf-8"?><atom:feed xmlns:atom="http://www.w3.org/2005/Atom"><atom:author><atom:name>SAP SE</atom:name></atom:author><atom:title>Installed Components</atom:title><atom:updated>2026-09-25T16:02:05Z</atom:updated><atom:entry><atom:id>EA-DFPS</atom:id><atom:title>618;SAPK-61804INEADFPS;0004;SAP Enterprise Extension Defense Forces &amp; Public Security</atom:title></atom:entry><atom:entry><atom:id>EA-HRCAR</atom:id><atom:title>608;SAPK-60833INEAHRCAR;0033;Подкомпонент EA-HRCAR для EA-HR</atom:title></atom:entry><atom:entry><atom:id>ZNOSP</atom:id><atom:title>100</atom:title></atom:entry></atom:feed>`

func TestParseInstalledComponents_AtomFeed(t *testing.T) {
	got, err := parseInstalledComponents([]byte(componentsFeed))
	if err != nil {
		t.Fatal(err)
	}
	want := []InstalledComponent{
		{Name: "EA-DFPS", Release: "618", Package: "SAPK-61804INEADFPS", SupportPack: "0004", Description: "SAP Enterprise Extension Defense Forces & Public Security"},
		{Name: "EA-HRCAR", Release: "608", Package: "SAPK-60833INEAHRCAR", SupportPack: "0033", Description: "Подкомпонент EA-HRCAR для EA-HR"},
		{Name: "ZNOSP", Release: "100"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseInstalledComponents_ComponentsElement(t *testing.T) {
	got, err := parseInstalledComponents([]byte(`<components><component name="SAP_BASIS" release="750" supportPack="0025" description="SAP Basis"/></components>`))
	if err != nil {
		t.Fatal(err)
	}
	want := []InstalledComponent{{Name: "SAP_BASIS", Release: "750", SupportPack: "0025", Description: "SAP Basis"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
