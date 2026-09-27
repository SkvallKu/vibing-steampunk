package adt

import (
	"fmt"
	"regexp"
	"strings"
)

// ObjectTypeStructure is a DDIC structure (SE11 "structure", TABL/DS). Its
// source lives at /sap/bc/adt/ddic/structures/{name}/source/main, next to the
// tables but a different resource: /ddic/tables does not serve a structure on
// any release (7.57 answers 404 "error importing object"), and the table
// syntax check refuses its source ("use define table"). Reading it was always
// possible (GetStructure); creating and writing it had no route.
const ObjectTypeStructure CreatableObjectType = "TABL/DS"

func init() {
	objectTypes[ObjectTypeStructure] = objectTypeInfo{
		creationPath: "/sap/bc/adt/ddic/structures",
		rootName:     "blue:blueSource",
		namespace:    `xmlns:blue="http://www.sap.com/wbobj/blue"`,
		bodyBuilder:  buildStructureBody,
	}
}

// buildStructureBody is the document the structure collection takes: the
// same blueSource as a table's, typed TABL/DS. The source is written after,
// with a lock, like every other source type.
func buildStructureBody(opts CreateObjectOptions, typeInfo objectTypeInfo, responsible string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<blue:blueSource %s xmlns:adtcore="http://www.sap.com/adt/core"
                 adtcore:name="%s"
                 adtcore:type="%s"
                 adtcore:description="%s"
                 adtcore:responsible="%s">
  <adtcore:packageRef adtcore:name="%s"/>
</blue:blueSource>`, typeInfo.namespace, escapeXML(opts.Name), ObjectTypeStructure,
		escapeXML(opts.Description), escapeXML(responsible), escapeXML(opts.PackageName))
}

var (
	ddlBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	ddlLineComment  = regexp.MustCompile(`//[^\n]*`)
)

// IsStructureSource tells a structure's source from a table's by its first
// statement after the annotations: "define structure" (7.52 on), "define
// type" (7.50), or an append's "extend type". A table says "define table".
// Anything else is not a structure.
func IsStructureSource(source string) bool {
	text := ddlLineComment.ReplaceAllString(ddlBlockComment.ReplaceAllString(source, " "), " ")
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "@") {
			continue
		}
		words := strings.Fields(strings.ToLower(line))
		if len(words) < 2 {
			return false
		}
		switch {
		case words[0] == "define" && (words[1] == "structure" || words[1] == "type"):
			return true
		case words[0] == "extend" && words[1] == "type":
			return true
		default:
			return false
		}
	}
	return false
}
