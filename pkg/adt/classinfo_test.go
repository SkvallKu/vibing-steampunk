package adt

import (
	"reflect"
	"strings"
	"testing"
)

func TestClassDefinitionPart(t *testing.T) {
	src := "class ZCO_X definition\r\n  public\r\n  inheriting from CL_PROXY_CLIENT\r\n  create public .\r\n" +
		"public section.\r\n  interfaces IF_ONE .\r\n  INTERFACES /ns/if_two.\r\nendclass.\r\n" +
		"CLASS ZCO_X IMPLEMENTATION.\r\n  interfaces if_not_here.\r\nENDCLASS.\r\n"
	def := classDefinitionPart(src, "ZCO_X")
	if !strings.Contains(def, "inheriting from CL_PROXY_CLIENT") || strings.Contains(def, "if_not_here") {
		t.Fatalf("definition part:\n%s", def)
	}
	var got []string
	for _, m := range classInterfacesRe.FindAllStringSubmatch(def, -1) {
		got = append(got, strings.ToUpper(m[1]))
	}
	if !reflect.DeepEqual(got, []string{"IF_ONE", "/NS/IF_TWO"}) {
		t.Errorf("interfaces %v", got)
	}
}
