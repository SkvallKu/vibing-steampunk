"! <p class="shorttext synchronized">vsp ADT fallback: own ATC data</p>
"! What the ATC runs of the logged-on user leave behind, and its removal.
"! A run from ADT leaves its results (SATC_AC_RESULTH and the tables under
"! it, kept 3 days), its execution (SATC_RT_RUN*, 6 days) and its project
"! (SATC_MD_DIR, 17 days); from 7.50 on also the worklist it was merged
"! into (SATC_WL_*, 7 days). The garbage collector of ATC removes only what
"! has expired, in the whole system, and does not run everywhere. ADT has
"! no DELETE for any of it.
"! <ul>
"! <li>GET /sap/bc/adt/zvsp/atcdata: one line per run and per worklist of
"!     the user in this system and client:
"!     run TAB id TAB variant TAB scheduled TAB valid to TAB prio 1 TAB
"!     prio 2 TAB prio 3 TAB title, and
"!     worklist TAB id TAB variant TAB changed TAB expires TAB objects TAB
"!     first object. Times are UTC time stamps. ?variant=, ?title= (CP
"!     patterns; title of a worklist: one of its objects), ?from= and ?to=
"!     (time stamps) narrow it;</li>
"! <li>DELETE /sap/bc/adt/zvsp/atcdata/runs/{id}: the results, the
"!     execution and, when nothing else refers to it, the project of a run,
"!     by the methods ATC's garbage collector uses. The answer names what
"!     went;</li>
"! <li>DELETE /sap/bc/adt/zvsp/atcdata/worklists/{id}: a worklist.</li>
"! </ul>
"! Only the user's own runs (SCHEDULED_BY) and worklists (CREATED_BY),
"! never one of a run series or a central run. ATC is reached by name
"! only: a release that lacks a class or table answers so, and nothing
"! here fails to activate for it.
CLASS zcl_vsp_adt_atcd_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    METHODS get REDEFINITION.
    METHODS delete REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'text/plain; charset=utf-8' ##NO_TEXT.

    TYPES ty_id TYPE x LENGTH 16.

    TYPES:
      BEGIN OF ty_run,
        display_id       TYPE ty_id,
        title            TYPE c LENGTH 70,
        chk_profile_name TYPE c LENGTH 30,
        scheduled_on_ts  TYPE p LENGTH 8 DECIMALS 0,
        valid_to         TYPE p LENGTH 8 DECIMALS 0,
        count_prio1      TYPE i,
        count_prio2      TYPE i,
        count_prio3      TYPE i,
        proj_id          TYPE ty_id,
        exec_id          TYPE ty_id,
      END OF ty_run,
      tt_runs TYPE STANDARD TABLE OF ty_run WITH DEFAULT KEY.

    TYPES:
      BEGIN OF ty_worklist,
        worklist_id      TYPE ty_id,
        chk_profile_name TYPE c LENGTH 30,
        changed_on       TYPE p LENGTH 8 DECIMALS 0,
        expires_on       TYPE p LENGTH 8 DECIMALS 0,
      END OF ty_worklist,
      tt_worklists TYPE STANDARD TABLE OF ty_worklist WITH DEFAULT KEY.

    TYPES:
      BEGIN OF ty_object,
        worklist_id TYPE ty_id,
        obj_type    TYPE c LENGTH 4,
        obj_name    TYPE c LENGTH 40,
      END OF ty_object,
      tt_objects TYPE STANDARD TABLE OF ty_object WITH DEFAULT KEY.

    DATA id TYPE ty_id.
    DATA id_text TYPE string.

    "! The runs under the condition, those of run series and central runs
    "! left out.
    METHODS read_runs
      IMPORTING where         TYPE string
      RETURNING VALUE(result) TYPE tt_runs
      RAISING   cx_adt_rest.

    METHODS read_worklists
      IMPORTING where         TYPE string
      RETURNING VALUE(result) TYPE tt_worklists.

    METHODS read_objects
      IMPORTING worklist_id   TYPE ty_id
      RETURNING VALUE(result) TYPE tt_objects.

    "! Whether a row is left in a table under the condition.
    METHODS exists
      IMPORTING table         TYPE csequence
                where         TYPE string
      RETURNING VALUE(result) TYPE abap_bool.

    METHODS delete_run
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_rest.

    METHODS delete_worklist
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_rest.

    "! ID = 'hex': a condition naming the ID of this request.
    METHODS id_is
      IMPORTING field         TYPE csequence
                value         TYPE ty_id
      RETURNING VALUE(result) TYPE string.

    METHODS own_runs
      RETURNING VALUE(result) TYPE string.

    METHODS parameter
      IMPORTING request       TYPE REF TO if_adt_rest_request
                name          TYPE string
      RETURNING VALUE(result) TYPE string.

    METHODS fail
      IMPORTING text TYPE csequence OPTIONAL
      RAISING   cx_adt_rest.
ENDCLASS.



CLASS zcl_vsp_adt_atcd_resource IMPLEMENTATION.

  METHOD get.
    DATA variant TYPE string.
    DATA title TYPE string.
    DATA from TYPE string.
    DATA to TYPE string.
    DATA where TYPE string.
    DATA runs TYPE tt_runs.
    DATA run TYPE ty_run.
    DATA worklists TYPE tt_worklists.
    DATA worklist TYPE ty_worklist.
    DATA objects TYPE tt_objects.
    DATA object TYPE ty_object.
    DATA first TYPE string.
    DATA matches TYPE abap_bool.
    DATA body TYPE string.
    DATA tab TYPE c LENGTH 1.

    variant = to_upper( parameter( request = request name = 'variant' ) ).
    title = to_upper( parameter( request = request name = 'title' ) ).
    from = parameter( request = request name = 'from' ).
    to = parameter( request = request name = 'to' ).
    IF from CN '0123456789' OR to CN '0123456789'.
      fail( |from and to are time stamps, YYYYMMDDhhmmss in UTC| ).
    ENDIF.
    tab = cl_abap_char_utilities=>horizontal_tab.

    where = own_runs( ).
    IF from IS NOT INITIAL.
      where = |{ where } AND SCHEDULED_ON_TS >= { from }|.
    ENDIF.
    IF to IS NOT INITIAL.
      where = |{ where } AND SCHEDULED_ON_TS <= { to }|.
    ENDIF.
    runs = read_runs( where ).
    SORT runs BY scheduled_on_ts display_id.
    LOOP AT runs INTO run.
      IF variant IS NOT INITIAL AND run-chk_profile_name NP variant.
        CONTINUE.
      ENDIF.
      IF title IS NOT INITIAL AND run-title NP title.
        CONTINUE.
      ENDIF.
      body = |{ body }run{ tab }{ run-display_id }{ tab }{ run-chk_profile_name }{ tab }| &&
             |{ run-scheduled_on_ts }{ tab }{ run-valid_to }{ tab }| &&
             |{ run-count_prio1 }{ tab }{ run-count_prio2 }{ tab }{ run-count_prio3 }{ tab }| &&
             |{ run-title }\n|.
    ENDLOOP.

    where = |CREATED_BY = '{ sy-uname }'|.
    IF from IS NOT INITIAL.
      where = |{ where } AND CHANGED_ON >= { from }|.
    ENDIF.
    IF to IS NOT INITIAL.
      where = |{ where } AND CHANGED_ON <= { to }|.
    ENDIF.
    worklists = read_worklists( where ).
    SORT worklists BY changed_on worklist_id.
    LOOP AT worklists INTO worklist.
      IF variant IS NOT INITIAL AND worklist-chk_profile_name NP variant.
        CONTINUE.
      ENDIF.
      objects = read_objects( worklist-worklist_id ).
      CLEAR first.
      matches = boolc( title IS INITIAL ).
      LOOP AT objects INTO object.
        IF first IS INITIAL.
          first = |{ object-obj_type } { object-obj_name }|.
        ENDIF.
        IF matches = abap_false AND object-obj_name CP title.
          matches = abap_true.
        ENDIF.
      ENDLOOP.
      IF matches = abap_false.
        CONTINUE.
      ENDIF.
      body = |{ body }worklist{ tab }{ worklist-worklist_id }{ tab }{ worklist-chk_profile_name }{ tab }| &&
             |{ worklist-changed_on }{ tab }{ worklist-expires_on }{ tab }| &&
             |{ lines( objects ) }{ tab }{ first }\n|.
    ENDLOOP.

    zcl_vsp_adt_util=>respond( response     = response
                               content_type = co_content_type
                               body         = body ).
  ENDMETHOD.


  METHOD delete.
    DATA kind TYPE string.
    DATA body TYPE string.

    request->get_uri_attribute(
      EXPORTING name      = 'kind'
                mandatory = abap_true
      IMPORTING value     = kind ).
    request->get_uri_attribute(
      EXPORTING name      = 'id'
                mandatory = abap_true
      IMPORTING value     = id_text ).
    id_text = to_upper( condense( id_text ) ).
    IF strlen( id_text ) <> 32 OR id_text CN '0123456789ABCDEF'.
      RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDIF.
    id = id_text.

    CASE to_lower( kind ).
      WHEN 'runs'.
        body = delete_run( ).
      WHEN 'worklists'.
        body = delete_worklist( ).
      WHEN OTHERS.
        RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDCASE.

    zcl_vsp_adt_util=>respond( response     = response
                               content_type = co_content_type
                               body         = body ).
  ENDMETHOD.


  METHOD delete_run.
    DATA runs TYPE tt_runs.
    DATA run TYPE ty_run.
    DATA cleaner TYPE REF TO object.

    IF NOT exists( table = 'SATC_AC_RESULTH' where = id_is( field = 'DISPLAY_ID' value = id ) ).
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'ATC_RUN'
          resource_id   = CONV #( id_text ).
    ENDIF.
    runs = read_runs( |{ id_is( field = 'DISPLAY_ID' value = id ) } AND { own_runs( ) }| ).
    READ TABLE runs INDEX 1 INTO run.
    IF sy-subrc <> 0.
      fail( |ATC run { id_text } is not one of yours in this system and client,| &&
            | or belongs to a run series or is a central run (transaction ATC)| ).
    ENDIF.

    TRY.
        " results: takes the run's lock and does nothing if it cannot
        CREATE OBJECT cleaner TYPE ('CL_SATC_AC_HOUSE_KEEPING').
        CALL METHOD cleaner->('REMOVE_DISPLAY_LOAD')
          EXPORTING
            i_display_id = run-display_id.
      CATCH cx_sy_create_object_error cx_sy_dyn_call_error.
        fail( |This release has no CL_SATC_AC_HOUSE_KEEPING->REMOVE_DISPLAY_LOAD| ).
    ENDTRY.
    IF exists( table = 'SATC_AC_RESULTH' where = id_is( field = 'DISPLAY_ID' value = id ) ).
      fail( |ATC run { id_text } is locked: is it still going? Nothing was removed| ).
    ENDIF.
    result = |results{ cl_abap_char_utilities=>horizontal_tab }{ id_text }\n|.

    TRY.
        CREATE OBJECT cleaner TYPE ('CL_SATC_HOUSE_KEEPING').
        IF run-exec_id IS NOT INITIAL
            AND exists( table = 'SATC_RT_RUN'
                        where = |{ id_is( field = 'EXEC_ID' value = run-exec_id ) } AND CREATED_BY = '{ sy-uname }'| ).
          CALL METHOD cleaner->('REMOVE_PROJECT_EXECUTION_DATA')
            EXPORTING
              i_execution_id = run-exec_id.
          IF exists( table = 'SATC_RT_RUN' where = id_is( field = 'EXEC_ID' value = run-exec_id ) ).
            result = |{ result }# execution { run-exec_id } is locked: left to ATC's garbage collector\n|.
          ELSE.
            result = |{ result }execution{ cl_abap_char_utilities=>horizontal_tab }{ run-exec_id }\n|.
          ENDIF.
        ENDIF.
        " the project, when no other run refers to it and it is a plain one
        IF run-proj_id IS NOT INITIAL
            AND NOT exists( table = 'SATC_AC_RESULTH' where = id_is( field = 'PROJ_ID' value = run-proj_id ) )
            AND NOT exists( table = 'SATC_RT_RUN' where = id_is( field = 'PROJECT_ID' value = run-proj_id ) )
            AND exists( table = 'SATC_MD_DIR'
                        where = |{ id_is( field = 'REPOSITORY_ID' value = run-proj_id ) } AND KIND = 'PRJ'| &&
                                | AND CREATED_BY = '{ sy-uname }' AND IS_PERMANENT = ' '| ).
          CALL METHOD cleaner->('REMOVE_PROJECT_DATA')
            EXPORTING
              i_project_id = run-proj_id.
          IF exists( table = 'SATC_MD_DIR' where = id_is( field = 'REPOSITORY_ID' value = run-proj_id ) ).
            result = |{ result }# project { run-proj_id } was not removed: left to ATC's garbage collector\n|.
          ELSE.
            result = |{ result }project{ cl_abap_char_utilities=>horizontal_tab }{ run-proj_id }\n|.
          ENDIF.
        ENDIF.
      CATCH cx_sy_create_object_error cx_sy_dyn_call_error.
        result = |{ result }# this release has no CL_SATC_HOUSE_KEEPING: execution and project left to ATC's garbage collector\n|.
    ENDTRY.
  ENDMETHOD.


  METHOD delete_worklist.
    DATA manager TYPE REF TO object.
    DATA failure TYPE REF TO cx_root.

    IF NOT exists( table = 'SATC_WL_HEAD' where = id_is( field = 'WORKLIST_ID' value = id ) ).
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'ATC_WORKLIST'
          resource_id   = CONV #( id_text ).
    ENDIF.
    IF NOT exists( table = 'SATC_WL_HEAD'
                   where = |{ id_is( field = 'WORKLIST_ID' value = id ) } AND CREATED_BY = '{ sy-uname }'| ).
      fail( |ATC worklist { id_text } is not one of yours| ).
    ENDIF.

    TRY.
        CALL METHOD ('CL_SATC_WL_MANAGER')=>('CREATE_MANAGER')
          RECEIVING
            result = manager.
        CALL METHOD manager->('IF_SATC_WL_MANAGER~DELETE_WORKLIST')
          EXPORTING
            i_worklist_id = id.
      CATCH cx_sy_dyn_call_error.
        fail( |This release has no IF_SATC_WL_MANAGER~DELETE_WORKLIST| ).
      CATCH cx_root INTO failure ##CATCH_ALL.
        fail( |ATC worklist { id_text }: { failure->get_text( ) }| ).
    ENDTRY.
    IF exists( table = 'SATC_WL_HEAD' where = id_is( field = 'WORKLIST_ID' value = id ) ).
      fail( |ATC worklist { id_text } is locked: nothing was removed| ).
    ENDIF.
    result = |worklist{ cl_abap_char_utilities=>horizontal_tab }{ id_text }\n|.
  ENDMETHOD.


  METHOD read_runs.
    DATA fields TYPE string.
    DATA plain TYPE string.
    DATA table TYPE string VALUE 'SATC_AC_RESULTH'.

    fields = 'DISPLAY_ID TITLE CHK_PROFILE_NAME SCHEDULED_ON_TS VALID_TO' &&
             ' COUNT_PRIO1 COUNT_PRIO2 COUNT_PRIO3 PROJ_ID EXEC_ID'.
    TRY.
        plain = |{ where } AND IS_CENTRAL_RUN = ' ' AND RUN_SERIES_NAME = ' '|.
        SELECT (fields) FROM (table) INTO CORRESPONDING FIELDS OF TABLE result
          WHERE (plain).
      CATCH cx_sy_dynamic_osql_error.
        " a release without these fields has neither run series nor central runs
        TRY.
            SELECT (fields) FROM (table) INTO CORRESPONDING FIELDS OF TABLE result
              WHERE (where).
          CATCH cx_sy_dynamic_osql_error.
            fail( |This release keeps no ATC results in SATC_AC_RESULTH as known here| ).
        ENDTRY.
    ENDTRY.
  ENDMETHOD.


  METHOD read_worklists.
    DATA table TYPE string VALUE 'SATC_WL_HEAD'.

    " none before 7.50: the older protocol runs without worklists
    TRY.
        SELECT worklist_id chk_profile_name changed_on expires_on
          FROM (table) INTO CORRESPONDING FIELDS OF TABLE result
          WHERE (where).
      CATCH cx_sy_dynamic_osql_error.
        CLEAR result.
    ENDTRY.
  ENDMETHOD.


  METHOD read_objects.
    DATA table TYPE string VALUE 'SATC_WL_OBJECT'.
    DATA where TYPE string.

    where = id_is( field = 'WORKLIST_ID' value = worklist_id ).
    TRY.
        SELECT worklist_id obj_type obj_name
          FROM (table) INTO CORRESPONDING FIELDS OF TABLE result
          WHERE (where)
          ORDER BY obj_type obj_name.
      CATCH cx_sy_dynamic_osql_error.
        CLEAR result.
    ENDTRY.
  ENDMETHOD.


  METHOD exists.
    DATA count TYPE i.

    TRY.
        SELECT COUNT(*) FROM (table) INTO count WHERE (where).
      CATCH cx_sy_dynamic_osql_error.
        CLEAR count.
    ENDTRY.
    result = boolc( count > 0 ).
  ENDMETHOD.


  METHOD id_is.
    DATA hex TYPE c LENGTH 32.

    hex = value.
    result = |{ field } = '{ hex }'|.
  ENDMETHOD.


  METHOD own_runs.
    result = |SCHEDULED_BY = '{ sy-uname }' AND SCHEDULED_SYS = '{ sy-sysid }'| &&
             | AND SCHEDULED_CLT = '{ sy-mandt }'|.
  ENDMETHOD.


  METHOD parameter.
    request->get_uri_query_parameter(
      EXPORTING name      = name
                mandatory = abap_false
      IMPORTING value     = result ).
    result = condense( result ).
  ENDMETHOD.


  METHOD fail.
    zcl_vsp_adt_util=>raise_failure( resource_type = 'ATC_RUN'
                                     resource_id   = id_text
                                     text          = text ).
  ENDMETHOD.

ENDCLASS.
