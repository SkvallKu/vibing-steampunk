package embedded

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The adt-fallback set stands in for ADT resources a release lacks. These
// checks read the source vsp install adt-fallback deploys and fail on any
// statement that would let it do more than that: call a function module,
// write a database table or make a dynamic call not listed here, create an
// object without the S_DEVELOP check, write texts without the lock, or
// delete ATC data or check variants past the refusals that keep it to the
// caller's own. A new entry in a list is a decision, taken in the commit
// that adds it.

// adtFallbackFunctions are the only function modules the set may call.
var adtFallbackFunctions = map[string]bool{
	// Dictionary reads and writes: the stand-ins for /ddic/*.
	"DDIF_DOMA_GET": true, "DDIF_DOMA_PUT": true,
	"DDIF_DTEL_GET": true, "DDIF_DTEL_PUT": true,
	"DDIF_TABL_GET": true, "DDIF_TABL_PUT": true,
	"DDIF_TTYP_GET": true, "DDIF_TTYP_PUT": true,
	"DDIF_TYPEINFO_GET": true, "DD_INT_DD03P_ACCUMULATE": true,
	// Locks, transport entries, the inactive worklist and deletion, as the
	// editor does them.
	"RS_ACCESS_PERMISSION": true, "RS_CORR_INSERT": true,
	"RS_INSERT_INTO_WORKING_AREA": true, "RS_DD_DELETE_OBJ": true,
	"SEO_CLIF_ACCESS_PERMISSION": true, "SEO_CLIF_CORR_INSERT": true,
	"ENQUEUE_READ": true,
	// GUI statuses and titles.
	"RS_CUA_INTERNAL_FETCH": true, "RS_CUA_INTERNAL_WRITE": true,
	// Text pools.
	"RS_TEXTPOOL_READ": true, "SELOPTS_AND_PARAMS": true,
	// Formatting.
	"CONVERSION_EXIT_ISOLA_OUTPUT": true, "GET_SYSTEM_TIMEZONE": true,
}

// adtFallbackWrites are the only INSERT/UPDATE/MODIFY/DELETE statements of
// the set. Two database tables are written directly, the class component
// texts, under the class lock (TestADTFallbackComponentTextsUnderTheLock);
// INSERT TEXTPOOL writes the inactive text pool. The rest are internal
// tables.
var adtFallbackWrites = map[string]bool{
	"DELETE FROM SEOCOMPOTX WHERE CLSNAME = NAME AND CMPNAME = TEXT-CMPNAME AND LANGU = LANGUAGE":                            true,
	"DELETE FROM SEOSUBCOTX WHERE CLSNAME = NAME AND CMPNAME = TEXT-CMPNAME AND SCONAME = TEXT-SCONAME AND LANGU = LANGUAGE": true,
	"MODIFY SEOCOMPOTX FROM COMPOTX":                                   true,
	"MODIFY SEOSUBCOTX FROM SUBCOTX":                                   true,
	"INSERT TEXTPOOL REPT_NAME FROM TPOOL LANGUAGE LANGUAGE STATE 'I'": true,
	"DELETE DD03P WHERE DEPTH IS NOT INITIAL AND DEPTH <> '00'":        true,
	"DELETE INC_FKS": true,
	"DELETE INC_FK_FIELDS WHERE TABNAME = <FK>-TABNAME AND FIELDNAME = <FK>-FIELDNAME": true,
	"DELETE INC_VHS": true,
	"DELETE INC_VH_FIELDS WHERE TABNAME = <VH>-TABNAME AND FIELDNAME = <VH>-FIELDNAME": true,
	"DELETE KEYS WHERE SECKEYNAME IS INITIAL":                                          true,
	"DELETE KEYS WHERE SECKEYNAME IS NOT INITIAL":                                      true,
	"DELETE NAMES WHERE TABLE_LINE <> FILTER":                                          true,
	"DELETE TPOOL INDEX SY-TABIX":                                                      true,
	"DELETE TPOOL WHERE ID = ID":                                                       true,
	"INSERT <LINE> INTO TABLE <TABLE>":                                                 true,
	"INSERT ENTRY INTO TABLE RESULT":                                                   true,
	"INSERT ENTRY INTO TABLE TYPES":                                                    true,
	"INSERT OWNER INTO TABLE OWNERS":                                                   true,
	"MODIFY OWN FROM VALUE":                                                            true,
	"MODIFY SECKEYS FROM SECKEY":                                                       true,
}

// adtFallbackDynamic are the only dynamic calls and instantiations. The ATC
// and DDL classes are named at run time because not every release has them;
// the feature classes are the implementations of ZIF_VSP_ADT_FEATURE, and
// the Code Inspector checks the entries of SCITESTS
// (TestADTFallbackChecksComeFromSCITESTS).
var adtFallbackDynamic = map[string]bool{
	"CALL METHOD ('CL_SATC_WL_MANAGER')=>('CREATE_MANAGER') RECEIVING RESULT = MANAGER":                                                                                                      true,
	"CALL METHOD MANAGER->('IF_SATC_WL_MANAGER~DELETE_WORKLIST') EXPORTING I_WORKLIST_ID = ID":                                                                                               true,
	"CREATE OBJECT CLEANER TYPE ('CL_SATC_AC_HOUSE_KEEPING')":                                                                                                                                true,
	"CREATE OBJECT CLEANER TYPE ('CL_SATC_HOUSE_KEEPING')":                                                                                                                                   true,
	"CALL METHOD CLEANER->('REMOVE_DISPLAY_LOAD') EXPORTING I_DISPLAY_ID = RUN-DISPLAY_ID":                                                                                                   true,
	"CALL METHOD CLEANER->('REMOVE_PROJECT_DATA') EXPORTING I_PROJECT_ID = RUN-PROJ_ID":                                                                                                      true,
	"CALL METHOD CLEANER->('REMOVE_PROJECT_EXECUTION_DATA') EXPORTING I_EXECUTION_ID = RUN-EXEC_ID":                                                                                          true,
	"CALL METHOD (CO_PARSER)=>('CREATE') EXPORTING I_LOGGER = LOGGER I_OBJECT_NAME = NAME RECEIVING R_RESULT = PARSER":                                                                       true,
	"CALL METHOD (CO_PERSIST)=>('ADJUST_FOREIGN_KEY_FIELDS') EXPORTING I_FOREIGN_KEY_FIELDS = FK_FIELDS I_FOREIGN_KEYS = RESULT-FOREIGN_KEYS RECEIVING R_RESULT = RESULT-FOREIGN_KEY_FIELDS": true,
	"CALL METHOD LOGGER->('GET_ERROR_MESSAGE') EXPORTING P_INDEX = 1 IMPORTING P_MESSAGE_TEXT = MESSAGE P_LINE = ERROR_LINE EXCEPTIONS OTHERS = 1":                                           true,
	"CALL METHOD LOGGER->('HAS_ERROR_MESSAGES') RECEIVING RESULT = HAS_ERRORS":                                                                                                               true,
	"CALL METHOD PARSER->('GET_FIELDS') RECEIVING R_RESULT = RESULT-FIELDS":                                                                                                                  true,
	"CALL METHOD PARSER->('GET_FOREIGN_KEYS') RECEIVING R_RESULT = RESULT-FOREIGN_KEYS":                                                                                                      true,
	"CALL METHOD PARSER->('GET_FOREIGN_KEY_FIELDS') RECEIVING R_RESULT = FK_FIELDS":                                                                                                          true,
	"CALL METHOD PARSER->('GET_HEADER') RECEIVING R_RESULT = RESULT-HEADER":                                                                                                                  true,
	"CALL METHOD PARSER->('GET_VALUE_HELPS') RECEIVING R_RESULT = RESULT-VALUE_HELPS":                                                                                                        true,
	"CALL METHOD PARSER->('GET_VALUE_HELP_FIELDS') RECEIVING R_RESULT = RESULT-VALUE_HELP_FIELDS":                                                                                            true,
	"CALL METHOD PARSER->('SCAN_SOURCE_TABLE') EXPORTING I_SOURCE_CODE = SOURCE":                                                                                                             true,
	"CALL METHOD PERSIST->('GET_SOURCE') EXPORTING I_OBJECT_KEY = KEY I_VERSION = VERSION IMPORTING E_SOURCE = RESULT":                                                                       true,
	"CREATE OBJECT FEATURE TYPE (NAME)":       true,
	"CREATE OBJECT OBJECT TYPE (NAME)":        true,
	"CREATE OBJECT PERSIST TYPE (CO_PERSIST)": true,
}

var (
	afFunctionRe = regexp.MustCompile(`^CALL FUNCTION '([^']+)'`)
	afDynamicRe  = regexp.MustCompile(`(->|=>)\(|^CALL METHOD \(|TYPE \(`)
	// What the set must never do: reach another system or task, run or
	// generate a program, or bypass Open SQL.
	afForbiddenRe = regexp.MustCompile(`^(SUBMIT|GENERATE SUBROUTINE|INSERT REPORT|EXEC SQL|CALL TRANSACTION|LEAVE TO TRANSACTION)\b|\bDESTINATION\b|\bSTARTING NEW TASK\b|\bIN BACKGROUND TASK\b|^CALL FUNCTION \(`)
)

// adtFallbackSources is every class and interface of the set, by file name,
// as statements. The install report is left out: it creates the BAdI
// implementations and is checked by its own tests.
func adtFallbackSources(t *testing.T) map[string][]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("adtfallback", "*.abap"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no adt-fallback sources: %v", err)
	}
	out := map[string][]string{}
	for _, f := range files {
		if strings.HasSuffix(f, ".prog.abap") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var stmts []string
		for _, st := range abapStatements(string(b)) {
			stmts = append(stmts, normStmt(st))
		}
		out[filepath.Base(f)] = stmts
	}
	return out
}

// abapMethodBody is the statements of METHOD name in stmts, without the
// METHOD and ENDMETHOD lines; nil if there is no such method.
func abapMethodBody(stmts []string, name string) []string {
	head := "METHOD " + strings.ToUpper(name)
	for i, st := range stmts {
		if st != head {
			continue
		}
		for j := i + 1; j < len(stmts); j++ {
			if stmts[j] == "ENDMETHOD" {
				return stmts[i+1 : j]
			}
		}
	}
	return nil
}

func indexOfPrefix(stmts []string, prefix string) int {
	for i, st := range stmts {
		if strings.HasPrefix(st, prefix) {
			return i
		}
	}
	return -1
}

func TestADTFallbackCallsOnlyListedFunctions(t *testing.T) {
	for file, stmts := range adtFallbackSources(t) {
		for _, st := range stmts {
			if m := afFunctionRe.FindStringSubmatch(st); m != nil && !adtFallbackFunctions[m[1]] {
				t.Errorf("%s calls %s, which is not in adtFallbackFunctions", file, m[1])
			}
			if afForbiddenRe.MatchString(st) {
				t.Errorf("%s: forbidden statement %q", file, st)
			}
		}
	}
}

func TestADTFallbackWritesOnlyListedTables(t *testing.T) {
	for file, stmts := range adtFallbackSources(t) {
		for _, st := range stmts {
			if dbWriteRe.MatchString(st) && !adtFallbackWrites[st] {
				t.Errorf("%s: %q is not in adtFallbackWrites", file, st)
			}
		}
	}
}

func TestADTFallbackDynamicCallsAreListed(t *testing.T) {
	for file, stmts := range adtFallbackSources(t) {
		for _, st := range stmts {
			if afDynamicRe.MatchString(st) && !adtFallbackDynamic[st] {
				t.Errorf("%s: dynamic %q is not in adtFallbackDynamic", file, st)
			}
		}
	}
}

// A create checks S_DEVELOP with activity 01 for the package before the
// transport entry and the dictionary write, neither of which checks it.
func TestADTFallbackCreateChecksTheAuthorization(t *testing.T) {
	src := adtFallbackSources(t)
	for _, file := range []string{"zcl_vsp_adt_dd_resource.clas.abap", "zcl_vsp_adt_tabl_resource.clas.abap"} {
		body := abapMethodBody(src[file], "create")
		check := indexOfPrefix(body, "ZCL_VSP_ADT_UTIL=>CHECK_CREATE(")
		corr := indexOfPrefix(body, "CORR_INSERT(")
		if check < 0 || corr < 0 || check > corr {
			t.Errorf("%s: create must call check_create before corr_insert (check %d, corr_insert %d)", file, check, corr)
		}
	}
	util := abapMethodBody(src["zcl_vsp_adt_util.clas.abap"], "check_create")
	auth := indexOfPrefix(util, "AUTHORITY-CHECK OBJECT 'S_DEVELOP'")
	if auth < 0 || !strings.Contains(util[auth], "ID 'ACTVT' FIELD '01'") || !strings.Contains(util[auth], "ID 'DEVCLASS' FIELD DEVCLASS") ||
		indexOfPrefix(util, "RAISE EXCEPTION TYPE CX_ADT_RES_NO_AUTHORIZATION") < 0 {
		t.Errorf("check_create must check S_DEVELOP 01 for the package and refuse: %q", util)
	}
}

// The component texts are written straight into SEOCOMPOTX and SEOSUBCOTX.
// The one way there is PUT, which checks the class lock first.
func TestADTFallbackComponentTextsUnderTheLock(t *testing.T) {
	stmts := adtFallbackSources(t)["zcl_vsp_adt_cltx_resource.clas.abap"]
	put := abapMethodBody(stmts, "put")
	check, save := indexOfPrefix(put, "CHECK_HANDLE("), indexOfPrefix(put, "SAVE(")
	if check < 0 || save < 0 || check > save {
		t.Errorf("put must check the lock handle before save: %q", put)
	}
	for i, st := range stmts {
		if strings.HasPrefix(st, "SAVE(") && !contains(put, st) {
			t.Errorf("save is called outside put (statement %d): %q", i, st)
		}
	}
}

// ATC data is deleted only for the caller's own runs and worklists, never a
// run series or a central run.
func TestADTFallbackDeletesOnlyOwnATCData(t *testing.T) {
	stmts := adtFallbackSources(t)["zcl_vsp_adt_atcd_resource.clas.abap"]
	own := strings.Join(abapMethodBody(stmts, "own_runs"), " ")
	if !strings.Contains(own, "SCHEDULED_BY = '{ SY-UNAME }'") || !strings.Contains(own, "SCHEDULED_SYS = '{ SY-SYSID }'") {
		t.Errorf("own_runs must select the caller's runs in this system: %q", own)
	}
	src := strings.Join(stmts, "\n")
	for _, want := range []string{
		"{ OWN_RUNS( ) }",                    // a run is looked up among the caller's own
		"IS NOT ONE OF YOURS IN THIS SYSTEM", // and refused otherwise
		"AND CREATED_BY = '{ SY-UNAME }'",    // worklists: the caller's
	} {
		if !strings.Contains(src, want) {
			t.Errorf("atcd resource lost %q", want)
		}
	}
}

// Check variants are written and deleted only in the customer namespace and
// only when not transported; every write path checks.
func TestADTFallbackWritesOnlyOwnCheckVariants(t *testing.T) {
	stmts := adtFallbackSources(t)["zcl_vsp_adt_chkv_resource.clas.abap"]
	writable := strings.Join(abapMethodBody(stmts, "check_writable"), "\n")
	for _, want := range []string{"IF NAME(1) <> 'Z' AND NAME(1) <> 'Y'", "CHKVINF-TRANSPORT IS NOT INITIAL"} {
		if !strings.Contains(writable, want) {
			t.Errorf("check_writable lost %q", want)
		}
	}
	for _, method := range []string{"put", "delete"} {
		if indexOfPrefix(abapMethodBody(stmts, method), "CHECK_WRITABLE(") < 0 {
			t.Errorf("%s does not call check_writable", method)
		}
	}
}

// A Code Inspector check is instantiated by name only from SCITESTS.
func TestADTFallbackChecksComeFromSCITESTS(t *testing.T) {
	for file, stmts := range adtFallbackSources(t) {
		for i, st := range stmts {
			if !strings.Contains(st, "ZCL_VSP_ADT_CI_PARAMS=>NEW_CHECK(") {
				continue
			}
			found := false
			for j := i - 1; j >= 0 && !strings.HasPrefix(stmts[j], "METHOD "); j-- {
				found = found || strings.HasPrefix(stmts[j], "SELECT NAME FROM SCITESTS")
			}
			if !found {
				t.Errorf("%s: new_check without the names of SCITESTS before it: %q", file, st)
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// The set has to compile on 7.40 SP06, the oldest release it serves. A
// predicate method call (IF obj->is_x( ).), which is SP08, reached a live
// SP06 system once after a check on SP19 had passed; these are the SP08+
// and 7.50+ constructs to keep out. @ host variables are SP05 and absent,
// kept out so that nothing is near the line.
var (
	afNewerSyntaxRe = regexp.MustCompile(`\bXSDBOOL\(|\bIS INSTANCE OF\b|\bREDUCE\b|\bFILTER #|\bFOR \S+ IN\b|\bLET \S+ =|#\( BASE\b|\bGROUP BY\b.*\bINTO\b|^LOOP AT .* GROUP BY|\bENUM\b|@DATA\(|@\w|\bSWITCH #\(.*\bTHROW\b`)
	afConditionRe   = regexp.MustCompile(`^(IF|ELSEIF|WHILE|CHECK) (.+)$`)
	afInnerParens   = regexp.MustCompile(`\([^()]*\)`)
	afComparisonRe  = regexp.MustCompile(`\s(=|<>|<|>|<=|>=|EQ|NE|LT|GT|LE|GE|IS|CO|CN|CA|NA|CS|NS|CP|NP|BETWEEN|IN)\s`)
	afCallRe        = regexp.MustCompile(`^(NOT )?([A-Z0-9_/~]+((->|=>)[A-Z0-9_/~]+)*)§$`)
	// Predicate functions, not method calls: these are older than 7.40.
	afPredicateFunctions = map[string]bool{"LINE_EXISTS": true, "CONTAINS": true, "CONTAINS_ANY_OF": true, "CONTAINS_ANY_NOT_OF": true, "MATCHES": true}
)

// predicateCalls are the parts of a condition that are a method call standing
// alone, without a comparison.
func predicateCalls(stmt string) []string {
	m := afConditionRe.FindStringSubmatch(stmt)
	if m == nil {
		return nil
	}
	cond := m[2]
	// Arguments folded away, innermost first: a named argument's "=" is not
	// a comparison.
	for afInnerParens.MatchString(cond) {
		cond = afInnerParens.ReplaceAllString(cond, "§")
	}
	var out []string
	for _, part := range regexp.MustCompile(` (AND|OR) `).Split(cond, -1) {
		part = strings.TrimSpace(part)
		if afComparisonRe.MatchString(" " + part + " ") {
			continue
		}
		if c := afCallRe.FindStringSubmatch(part); c != nil && !afPredicateFunctions[c[2]] {
			out = append(out, part)
		}
	}
	return out
}

// afLiteralRe is a text literal, a string literal or a string template:
// their content is data, an "@" in it an annotation vsp writes.
var afLiteralRe = regexp.MustCompile("'[^']*'|`[^`]*`|\\|[^|]*\\|")

func TestADTFallbackCompilesOn740SP06(t *testing.T) {
	for file, stmts := range adtFallbackSources(t) {
		for _, st := range stmts {
			st = afLiteralRe.ReplaceAllString(st, "''")
			if afNewerSyntaxRe.MatchString(st) {
				t.Errorf("%s: newer than 7.40 SP06: %q", file, st)
			}
			if calls := predicateCalls(st); len(calls) > 0 {
				t.Errorf("%s: predicate method call %v (SP08) in %q; compare with abap_true", file, calls, st)
			}
		}
	}
}

func TestPredicateCallsFindsTheSP06Break(t *testing.T) {
	for stmt, want := range map[string]int{
		"IF ZCL_VSP_DDIC_SRC_SAP=>IS_AVAILABLE( )":             1,
		"IF NOT ME->EXISTS( NAME = X ) AND A = B":              1,
		"IF ZCL_VSP_DDIC_SRC_SAP=>IS_AVAILABLE( ) = ABAP_TRUE": 0,
		"IF LINE_EXISTS( ITAB[ A = 1 ] )":                      0,
		"WHILE I < LEN AND TEXT+I(1) <> NEWLINE":               0,
		"CHECK SY-SUBRC = 0":                                   0,
		"IF FOO IS BOUND OR BAR->IS_SET( )":                    1,
	} {
		if got := predicateCalls(stmt); len(got) != want {
			t.Errorf("%s: %v, want %d", stmt, got, want)
		}
	}
}
