"! <p class="shorttext synchronized">vsp ADT fallback: Code Inspector checks</p>
"! /sap/bc/adt/zvsp/ci/checks, GET: the checks registered in Code
"! Inspector (SCITESTS), what goes into a check variant. One line per
"! check, check TAB class TAB version TAB category TAB title, followed by
"! its parameters with their default values in the lines of
"! ZCL_VSP_ADT_CI_PARAMS. ?check=CLASS gives one check. Categories, which
"! are registered alongside, are left out. The parameters have no texts
"! here: a check gives those only in its own dialog.
CLASS zcl_vsp_adt_cichk_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    METHODS get REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'text/plain; charset=utf-8' ##NO_TEXT.
ENDCLASS.



CLASS zcl_vsp_adt_cichk_resource IMPLEMENTATION.

  METHOD get.
    DATA filter TYPE string.
    DATA names TYPE sci_chks.
    DATA name TYPE seoclsname.
    DATA test TYPE REF TO cl_ci_test_root.
    DATA lines TYPE string_table.
    DATA line TYPE string.
    DATA body TYPE string.
    DATA tab TYPE c LENGTH 1.

    request->get_uri_query_parameter(
      EXPORTING name      = 'check'
                mandatory = abap_false
      IMPORTING value     = filter ).
    filter = to_upper( condense( filter ) ).

    SELECT name FROM scitests INTO TABLE names.
    SORT names.
    IF filter IS NOT INITIAL.
      DELETE names WHERE table_line <> filter.
      IF names IS INITIAL.
        RAISE EXCEPTION TYPE cx_adt_res_not_found
          EXPORTING
            resource_type = 'CHECK'
            resource_id   = CONV #( filter ).
      ENDIF.
    ENDIF.

    tab = cl_abap_char_utilities=>horizontal_tab.
    LOOP AT names INTO name.
      test = zcl_vsp_adt_ci_params=>new_check( name ).
      IF test IS NOT BOUND.
        CONTINUE.
      ENDIF.
      body = |{ body }check{ tab }{ name }{ tab }{ test->version }{ tab }| &&
             |{ test->category }{ tab }{ test->description }\n|.
      IF test->has_attributes = abap_true.
        TRY.
            lines = zcl_vsp_adt_ci_params=>to_lines( check  = name
                                                     buffer = test->get_attributes( ) ).
          CATCH cx_root ##CATCH_ALL.
            CLEAR lines.
            APPEND |# { name }: parameters could not be read| TO lines.
        ENDTRY.
        LOOP AT lines INTO line.
          body = |{ body }{ line }\n|.
        ENDLOOP.
      ENDIF.
    ENDLOOP.
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = co_content_type
                               body         = body ).
  ENDMETHOD.

ENDCLASS.
