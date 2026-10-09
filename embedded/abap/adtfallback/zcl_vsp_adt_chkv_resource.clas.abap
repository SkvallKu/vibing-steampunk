"! <p class="shorttext synchronized">vsp ADT fallback: check variants</p>
"! /sap/bc/adt/zvsp/ci/checkvariants/{name}, a global Code Inspector check
"! variant (SCI), the kind ATC takes by name on releases without ATC check
"! variant objects (CHKV, 7.5x and later):
"! <ul>
"! <li>GET: the variant as text lines:
"!     description TAB text, then per check: check TAB class, followed by
"!     its parameters, param TAB class TAB name TAB value(s)
"!     (ZCL_VSP_ADT_CI_PARAMS); # lines are comments;</li>
"! <li>PUT: the same lines; creates the variant or replaces its checks.
"!     A check's parameters not named keep the check's own value. The
"!     answer is the GET one;</li>
"! <li>DELETE: removes the variant.</li>
"! </ul>
"! Only variants that are not transported, in the customer namespace, are
"! written or deleted: a transported one belongs to SAP or to a team, and
"! CL_CI_CHECKVARIANT would take it out of its transport on save. A new
"! variant is saved without transport. The lock (E_SCI_NM) is taken and
"! released within the request. With ?own=X a PUT or DELETE changes only a
"! variant the user changed last (UNAM; SCI keeps no author) and a PUT may
"! create one: vsp sends it when the connection is held to packages, so
"! that a variant of a team is not touched from there.
CLASS zcl_vsp_adt_chkv_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    METHODS get REDEFINITION.
    METHODS put REDEFINITION.
    METHODS delete REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'text/plain; charset=utf-8' ##NO_TEXT.

    DATA name TYPE sci_chkv.
    DATA own TYPE abap_bool.

    METHODS parse_uri
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    "! The variant, or nothing.
    METHODS find
      RETURNING VALUE(result) TYPE REF TO cl_ci_checkvariant.

    METHODS check_writable
      IMPORTING variant TYPE REF TO cl_ci_checkvariant OPTIONAL
      RAISING   cx_adt_rest.

    METHODS parse_body
      IMPORTING request     TYPE REF TO if_adt_rest_request
      EXPORTING description TYPE sci_text
                checks      TYPE string_table
                rows        TYPE zcl_vsp_adt_ci_params=>tt_rows
      RAISING   cx_adt_rest.

    METHODS build
      IMPORTING checks        TYPE string_table
                rows          TYPE zcl_vsp_adt_ci_params=>tt_rows
      RETURNING VALUE(result) TYPE sci_tstvar
      RAISING   cx_adt_rest.

    METHODS respond
      IMPORTING response TYPE REF TO if_adt_rest_response
      RAISING   cx_adt_rest.

    METHODS fail
      IMPORTING text TYPE csequence OPTIONAL
      RAISING   cx_adt_rest.
ENDCLASS.



CLASS zcl_vsp_adt_chkv_resource IMPLEMENTATION.

  METHOD get.
    parse_uri( request ).
    respond( response ).
  ENDMETHOD.


  METHOD put.
    DATA variant TYPE REF TO cl_ci_checkvariant.
    DATA description TYPE sci_text.
    DATA checks TYPE string_table.
    DATA rows TYPE zcl_vsp_adt_ci_params=>tt_rows.
    DATA content TYPE sci_tstvar.

    parse_uri( request ).
    parse_body( EXPORTING request     = request
                IMPORTING description = description
                          checks      = checks
                          rows        = rows ).
    content = build( checks = checks rows = rows ).

    variant = find( ).
    IF variant IS BOUND.
      check_writable( variant ).
      variant->enter_change(
        EXCEPTIONS
          locked           = 1
          error_in_enqueue = 2
          not_authorized   = 3
          OTHERS           = 4 ).
      IF sy-subrc <> 0.
        fail( ).
      ENDIF.
    ELSE.
      check_writable( ).
      cl_ci_checkvariant=>create(
        EXPORTING
          p_user              = space
          p_name              = name
        RECEIVING
          p_ref               = variant
        EXCEPTIONS
          chkv_already_exists = 1
          locked              = 2
          error_in_enqueue    = 3
          not_authorized      = 4
          OTHERS              = 5 ).
      IF sy-subrc <> 0 OR variant IS NOT BOUND.
        fail( ).
      ENDIF.
      IF description IS INITIAL.
        description = name.
      ENDIF.
    ENDIF.

    variant->save(
      EXPORTING
        p_variant       = content
        p_text          = description
        p_transport     = space
      EXCEPTIONS
        empty_variant   = 1
        transport_error = 2
        not_authorized  = 3
        OTHERS          = 4 ).
    IF sy-subrc <> 0.
      DATA(failed) = sy-subrc.
      MESSAGE ID sy-msgid TYPE 'S' NUMBER sy-msgno
        WITH sy-msgv1 sy-msgv2 sy-msgv3 sy-msgv4 INTO DATA(message).
      variant->leave_change( ).
      IF failed = 1.
        fail( |{ name }: a variant needs at least one check| ).
      ENDIF.
      fail( message ).
    ENDIF.
    variant->leave_change( ).
    respond( response ).
  ENDMETHOD.


  METHOD delete.
    DATA variant TYPE REF TO cl_ci_checkvariant.

    parse_uri( request ).
    variant = find( ).
    IF variant IS NOT BOUND.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'CHKV'
          resource_id   = CONV #( name ).
    ENDIF.
    check_writable( variant ).
    variant->delete(
      EXCEPTIONS
        exists_in_insp   = 1
        locked           = 2
        error_in_enqueue = 3
        not_authorized   = 4
        transport_error  = 5
        OTHERS           = 6 ).
    CASE sy-subrc.
      WHEN 0.
      WHEN 1.
        fail( |{ name } is used by a Code Inspector inspection; delete that first| ).
      WHEN OTHERS.
        fail( ).
    ENDCASE.
  ENDMETHOD.


  METHOD parse_uri.
    DATA value TYPE string.

    request->get_uri_attribute(
      EXPORTING name      = 'name'
                mandatory = abap_true
      IMPORTING value     = value ).
    value = to_upper( cl_http_utility=>unescape_url( value ) ).
    IF value IS INITIAL OR strlen( value ) > 30.
      RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDIF.
    name = value.
    CLEAR value.
    request->get_uri_query_parameter(
      EXPORTING name      = 'own'
                mandatory = abap_false
      IMPORTING value     = value ).
    own = boolc( to_upper( condense( value ) ) = 'X' ).
  ENDMETHOD.


  METHOD find.
    cl_ci_checkvariant=>get_ref(
      EXPORTING
        p_user                  = space
        p_name                  = name
        p_dont_look_for_new_chkv = abap_true
      RECEIVING
        p_ref                   = result
      EXCEPTIONS
        OTHERS                  = 1 ).
    IF sy-subrc <> 0.
      CLEAR result.
    ENDIF.
  ENDMETHOD.


  METHOD check_writable.
    IF name(1) <> 'Z' AND name(1) <> 'Y'.
      fail( |{ name }: only variants in the customer namespace (Z*, Y*) are written or deleted here| ).
    ENDIF.
    IF variant IS BOUND AND variant->chkvinf-transport IS NOT INITIAL.
      fail( |{ name } is transported: change it in SCI under a transport request| ).
    ENDIF.
    IF own = abap_true AND variant IS BOUND AND variant->chkvinf-unam <> sy-uname.
      fail( |{ name } was last changed by { variant->chkvinf-unam }: only own variants are written or deleted here| ).
    ENDIF.
  ENDMETHOD.


  METHOD parse_body.
    DATA lines TYPE string_table.
    DATA line TYPE string.
    DATA fields TYPE string_table.
    DATA kind TYPE string.
    DATA check TYPE string.
    DATA row TYPE zcl_vsp_adt_ci_params=>ty_row.
    DATA number TYPE i.

    CLEAR: description, checks, rows.
    lines = zcl_vsp_adt_util=>request_lines( request ).
    LOOP AT lines INTO line.
      number = sy-tabix.
      kind = condense( line ).
      IF kind IS INITIAL OR kind(1) = '#'.
        CONTINUE.
      ENDIF.
      SPLIT line AT cl_abap_char_utilities=>horizontal_tab INTO TABLE fields.
      READ TABLE fields INDEX 1 INTO kind.
      kind = to_lower( condense( kind ) ).
      CASE kind.
        WHEN 'description'.
          READ TABLE fields INDEX 2 INTO line.
          description = condense( line ).
        WHEN 'check'.
          READ TABLE fields INDEX 2 INTO check.
          check = to_upper( condense( check ) ).
          IF check IS INITIAL.
            fail( |Line { number }: check without a class| ).
          ENDIF.
          READ TABLE checks TRANSPORTING NO FIELDS WITH KEY table_line = check.
          IF sy-subrc = 0.
            fail( |Line { number }: { check } is named twice| ).
          ENDIF.
          APPEND check TO checks.
        WHEN 'param'.
          IF lines( fields ) < 3.
            fail( |Line { number }: want param TAB check TAB name TAB value(s)| ).
          ENDIF.
          CLEAR row.
          READ TABLE fields INDEX 2 INTO check.
          row-check = to_upper( condense( check ) ).
          READ TABLE fields INDEX 3 INTO row-name.
          row-name = to_upper( condense( row-name ) ).
          READ TABLE checks TRANSPORTING NO FIELDS WITH KEY table_line = row-check.
          IF sy-subrc <> 0.
            fail( |Line { number }: { row-check } has no check line before it| ).
          ENDIF.
          LOOP AT fields INTO line FROM 4.
            APPEND line TO row-values.
          ENDLOOP.
          IF row-values IS INITIAL.
            APPEND `` TO row-values.
          ENDIF.
          row-line = number.
          APPEND row TO rows.
        WHEN OTHERS.
          fail( |Line { number }: want description, check or param, not "{ kind }"| ).
      ENDCASE.
    ENDLOOP.
  ENDMETHOD.


  METHOD build.
    DATA check TYPE string.
    DATA test TYPE REF TO cl_ci_test_root.
    DATA entry TYPE sci_tstval.
    DATA active TYPE sci_chks.

    SELECT name FROM scitests INTO TABLE active.
    LOOP AT checks INTO check.
      READ TABLE active TRANSPORTING NO FIELDS WITH KEY table_line = check.
      IF sy-subrc <> 0.
        fail( |{ check } is not a check registered in Code Inspector (SCITESTS)| ).
      ENDIF.
      test = zcl_vsp_adt_ci_params=>new_check( check ).
      IF test IS NOT BOUND.
        fail( |{ check } is not a Code Inspector check (no subclass of CL_CI_TEST_ROOT)| ).
      ENDIF.
      CLEAR entry.
      entry-testname = check.
      entry-version = test->version.
      IF test->has_attributes = abap_true.
        entry-attributes = zcl_vsp_adt_ci_params=>apply( check  = check
                                                         buffer = test->get_attributes( )
                                                         rows   = rows ).
      ELSE.
        READ TABLE rows TRANSPORTING NO FIELDS WITH KEY check = check.
        IF sy-subrc = 0.
          fail( |{ check } has no parameters| ).
        ENDIF.
      ENDIF.
      INSERT entry INTO TABLE result.
    ENDLOOP.
  ENDMETHOD.


  METHOD respond.
    DATA variant TYPE REF TO cl_ci_checkvariant.
    DATA entry TYPE sci_tstval.
    DATA lines TYPE string_table.
    DATA line TYPE string.
    DATA body TYPE string.
    DATA tab TYPE c LENGTH 1.

    variant = find( ).
    IF variant IS NOT BOUND.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'CHKV'
          resource_id   = CONV #( name ).
    ENDIF.
    variant->get_info( EXCEPTIONS OTHERS = 1 ).
    IF sy-subrc <> 0.
      fail( ).
    ENDIF.

    tab = cl_abap_char_utilities=>horizontal_tab.
    body = |description{ tab }{ variant->text }\n|.
    IF variant->chkvinf-transport IS NOT INITIAL.
      body = |{ body }# transported: not written here\n|.
    ENDIF.
    LOOP AT variant->variant INTO entry.
      body = |{ body }check{ tab }{ entry-testname }\n|.
      lines = zcl_vsp_adt_ci_params=>to_lines( check  = entry-testname
                                               buffer = entry-attributes ).
      LOOP AT lines INTO line.
        body = |{ body }{ line }\n|.
      ENDLOOP.
    ENDLOOP.
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = co_content_type
                               body         = body ).
  ENDMETHOD.


  METHOD fail.
    zcl_vsp_adt_util=>raise_failure( resource_type = 'CHKV'
                                     resource_id   = name
                                     text          = text ).
  ENDMETHOD.

ENDCLASS.
