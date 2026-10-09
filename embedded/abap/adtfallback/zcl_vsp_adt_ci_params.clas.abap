"! <p class="shorttext synchronized">vsp ADT fallback: check parameters</p>
"! The parameters of a Code Inspector check as text lines, and back. A
"! check keeps them in a data cluster (EXPORT .. TO DATA BUFFER in its
"! GET_ATTRIBUTES), one named data object each: SELECT = 'X', ANY = a
"! table of patterns. The cluster carries the types, so it is read without
"! knowing the check (CL_ABAP_EXPIMP_UTILITIES=>DBUF_IMPORT_CREATE_DATA),
"! and written back the same way with the values changed.
"! <p>One line is one row: param TAB check TAB name TAB value(s). An
"! elementary parameter has one row of one value; a flat structure one row
"! of its components in order; a table one row per line, in the same way,
"! and a single row with one empty value for an empty table. Parameters of
"! other types are not shown and keep the check's own value.</p>
CLASS zcl_vsp_adt_ci_params DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    TYPES:
      BEGIN OF ty_row,
        check  TYPE seoclsname,
        name   TYPE string,
        values TYPE string_table,
        line   TYPE i,
      END OF ty_row,
      tt_rows TYPE STANDARD TABLE OF ty_row WITH DEFAULT KEY.

    "! A new instance of a check, or nothing when the class is not one.
    CLASS-METHODS new_check
      IMPORTING name          TYPE csequence
      RETURNING VALUE(result) TYPE REF TO cl_ci_test_root.

    "! The lines of a parameter cluster: param lines and, for what is
    "! not shown, a # comment.
    CLASS-METHODS to_lines
      IMPORTING check         TYPE csequence
                buffer        TYPE xstring
      RETURNING VALUE(result) TYPE string_table.

    "! The check's own cluster with the rows applied. Raises with a text
    "! naming the first row that does not fit.
    CLASS-METHODS apply
      IMPORTING check         TYPE csequence
                buffer        TYPE xstring
                rows          TYPE tt_rows
      RETURNING VALUE(result) TYPE xstring
      RAISING   cx_adt_rest.

  PROTECTED SECTION.
  PRIVATE SECTION.
    TYPES:
      BEGIN OF ty_bind,
        name  TYPE c LENGTH 30,
        value TYPE c LENGTH 30,
      END OF ty_bind,
      tt_binds TYPE STANDARD TABLE OF ty_bind WITH DEFAULT KEY.

    CLASS-METHODS decode
      IMPORTING buffer        TYPE xstring
      RETURNING VALUE(result) TYPE tab_cpar.

    "! The values of one row of the data object: one for an elementary
    "! one, the components of a flat structure. False when it is neither.
    CLASS-METHODS row_values
      IMPORTING data          TYPE any
      EXPORTING values        TYPE string_table
      RETURNING VALUE(result) TYPE abap_bool.

    "! Fills one row of a data object from values; the text of what does
    "! not fit, or nothing.
    CLASS-METHODS fill_row
      IMPORTING values        TYPE string_table
      CHANGING  data          TYPE any
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS is_flat_row
      IMPORTING type          TYPE REF TO cl_abap_datadescr
      RETURNING VALUE(result) TYPE abap_bool.

    CLASS-METHODS clean
      IMPORTING value         TYPE csequence
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS fail
      IMPORTING text TYPE csequence
      RAISING   cx_adt_rest.
ENDCLASS.



CLASS zcl_vsp_adt_ci_params IMPLEMENTATION.

  METHOD new_check.
    DATA object TYPE REF TO object.

    TRY.
        CREATE OBJECT object TYPE (name).
        result ?= object.
      CATCH cx_root ##CATCH_ALL.
        CLEAR result.
    ENDTRY.
  ENDMETHOD.


  METHOD to_lines.
    DATA params TYPE tab_cpar.
    DATA param TYPE cpar.
    DATA values TYPE string_table.
    DATA tab TYPE c LENGTH 1.
    DATA table_type TYPE REF TO cl_abap_tabledescr.
    FIELD-SYMBOLS <data> TYPE any.
    FIELD-SYMBOLS <table> TYPE ANY TABLE.
    FIELD-SYMBOLS <row> TYPE any.

    tab = cl_abap_char_utilities=>horizontal_tab.
    params = decode( buffer ).
    SORT params BY name.
    LOOP AT params INTO param.
      ASSIGN param-dref->* TO <data>.
      CASE cl_abap_typedescr=>describe_by_data( <data> )->kind.
        WHEN cl_abap_typedescr=>kind_table.
          table_type ?= cl_abap_typedescr=>describe_by_data( <data> ).
          IF is_flat_row( table_type->get_table_line_type( ) ) = abap_false.
            APPEND |# { check } { param-name }: a table of deep rows, not shown| TO result.
            CONTINUE.
          ENDIF.
          ASSIGN <data> TO <table>.
          IF <table> IS INITIAL.
            APPEND |param{ tab }{ check }{ tab }{ param-name }{ tab }| TO result.
          ENDIF.
          LOOP AT <table> ASSIGNING <row>.
            row_values( EXPORTING data = <row> IMPORTING values = values ).
            APPEND |param{ tab }{ check }{ tab }{ param-name }{ tab }| &&
                   concat_lines_of( table = values sep = tab ) TO result.
          ENDLOOP.
        WHEN OTHERS.
          IF row_values( EXPORTING data = <data> IMPORTING values = values ) = abap_false.
            APPEND |# { check } { param-name }: not an elementary value or flat structure, not shown| TO result.
            CONTINUE.
          ENDIF.
          APPEND |param{ tab }{ check }{ tab }{ param-name }{ tab }| &&
                 concat_lines_of( table = values sep = tab ) TO result.
      ENDCASE.
    ENDLOOP.
  ENDMETHOD.


  METHOD apply.
    DATA params TYPE tab_cpar.
    DATA param TYPE cpar.
    DATA binds TYPE tt_binds.
    DATA bind TYPE ty_bind.
    DATA row TYPE ty_row.
    DATA names TYPE string.
    DATA name TYPE string.
    DATA problem TYPE string.
    DATA count TYPE i.
    DATA empty TYPE abap_bool.
    DATA value TYPE string.
    DATA line_ref TYPE REF TO data.
    DATA table_type TYPE REF TO cl_abap_tabledescr.
    DATA components TYPE cl_abap_structdescr=>component_table.
    DATA component TYPE abap_componentdescr.
    DATA all_ref TYPE REF TO data.
    DATA all_type TYPE REF TO cl_abap_structdescr.
    FIELD-SYMBOLS <all> TYPE any.
    FIELD-SYMBOLS <component> TYPE any.
    FIELD-SYMBOLS <param> TYPE cpar.
    FIELD-SYMBOLS <data> TYPE any.
    FIELD-SYMBOLS <table> TYPE ANY TABLE.
    FIELD-SYMBOLS <line> TYPE any.

    params = decode( buffer ).
    LOOP AT params INTO param.
      names = |{ names } { param-name }|.
    ENDLOOP.

    LOOP AT rows INTO row WHERE check = check.
      name = to_upper( row-name ).
      READ TABLE params ASSIGNING <param> WITH KEY name = name.
      IF sy-subrc <> 0.
        IF names IS INITIAL.
          fail( |Line { row-line }: { check } has no parameters| ).
        ENDIF.
        fail( |Line { row-line }: { check } has no parameter { name }; it has{ names }| ).
      ENDIF.
      ASSIGN <param>-dref->* TO <data>.

      IF cl_abap_typedescr=>describe_by_data( <data> )->kind = cl_abap_typedescr=>kind_table.
        table_type ?= cl_abap_typedescr=>describe_by_data( <data> ).
        IF is_flat_row( table_type->get_table_line_type( ) ) = abap_false.
          fail( |Line { row-line }: { check } { name } is a table of deep rows and cannot be set| ).
        ENDIF.
        ASSIGN <data> TO <table>.
        " The first row for a table replaces what the check had.
        count = 0.
        LOOP AT rows TRANSPORTING NO FIELDS
             WHERE check = check AND name = row-name AND line < row-line.
          count = count + 1.
        ENDLOOP.
        IF count = 0.
          CLEAR <table>.
        ENDIF.
        empty = abap_false.
        IF lines( row-values ) = 1.
          READ TABLE row-values INDEX 1 INTO value.
          IF value IS INITIAL.
            empty = abap_true.
          ENDIF.
        ENDIF.
        IF empty = abap_true.
          CONTINUE.
        ENDIF.
        CREATE DATA line_ref LIKE LINE OF <table>.
        ASSIGN line_ref->* TO <line>.
        problem = fill_row( EXPORTING values = row-values CHANGING data = <line> ).
        IF problem IS NOT INITIAL.
          fail( |Line { row-line }: { check } { name }: { problem }| ).
        ENDIF.
        INSERT <line> INTO TABLE <table>.
      ELSE.
        count = 0.
        LOOP AT rows TRANSPORTING NO FIELDS WHERE check = check AND name = row-name.
          count = count + 1.
        ENDLOOP.
        IF count > 1.
          fail( |Line { row-line }: { check } { name } is not a table and takes one line| ).
        ENDIF.
        problem = fill_row( EXPORTING values = row-values CHANGING data = <data> ).
        IF problem IS NOT INITIAL.
          fail( |Line { row-line }: { check } { name }: { problem }| ).
        ENDIF.
      ENDIF.
    ENDLOOP.

    " EXPORT takes the data objects of a parameter list by name, not by
    " reference: the values go into one structure built for them, and
    " each name points at a component of it.
    IF params IS INITIAL.
      result = buffer.
      RETURN.
    ENDIF.
    LOOP AT params INTO param.
      component-name = |C{ sy-tabix }|.
      component-type ?= cl_abap_typedescr=>describe_by_data_ref( param-dref ).
      APPEND component TO components.
    ENDLOOP.
    all_type = cl_abap_structdescr=>create( components ).
    CREATE DATA all_ref TYPE HANDLE all_type.
    ASSIGN all_ref->* TO <all>.
    LOOP AT params INTO param.
      ASSIGN COMPONENT sy-tabix OF STRUCTURE <all> TO <component>.
      ASSIGN param-dref->* TO <data>.
      <component> = <data>.
      bind-name = param-name.
      bind-value = |<ALL>-C{ sy-tabix }|.
      APPEND bind TO binds.
    ENDLOOP.
    EXPORT (binds) TO DATA BUFFER result.
  ENDMETHOD.


  METHOD decode.
    IF buffer IS INITIAL.
      RETURN.
    ENDIF.
    TRY.
        result = cl_abap_expimp_utilities=>dbuf_import_create_data( dbuf = buffer ).
      CATCH cx_root ##CATCH_ALL.
        CLEAR result.
    ENDTRY.
  ENDMETHOD.


  METHOD row_values.
    DATA struct_type TYPE REF TO cl_abap_structdescr.
    DATA component TYPE abap_compdescr.
    FIELD-SYMBOLS <component> TYPE any.

    CLEAR values.
    CASE cl_abap_typedescr=>describe_by_data( data )->kind.
      WHEN cl_abap_typedescr=>kind_elem.
        APPEND clean( |{ data }| ) TO values.
        result = abap_true.
      WHEN cl_abap_typedescr=>kind_struct.
        struct_type ?= cl_abap_typedescr=>describe_by_data( data ).
        IF is_flat_row( struct_type ) = abap_false.
          RETURN.
        ENDIF.
        LOOP AT struct_type->components INTO component.
          ASSIGN COMPONENT sy-tabix OF STRUCTURE data TO <component>.
          APPEND clean( |{ <component> }| ) TO values.
        ENDLOOP.
        result = abap_true.
    ENDCASE.
  ENDMETHOD.


  METHOD fill_row.
    DATA struct_type TYPE REF TO cl_abap_structdescr.
    DATA value TYPE string.
    DATA index TYPE i.
    FIELD-SYMBOLS <component> TYPE any.

    TRY.
        CASE cl_abap_typedescr=>describe_by_data( data )->kind.
          WHEN cl_abap_typedescr=>kind_elem.
            IF lines( values ) <> 1.
              result = |want one value, got { lines( values ) }|.
              RETURN.
            ENDIF.
            READ TABLE values INDEX 1 INTO value.
            data = value.
          WHEN cl_abap_typedescr=>kind_struct.
            struct_type ?= cl_abap_typedescr=>describe_by_data( data ).
            IF lines( values ) <> lines( struct_type->components ).
              result = |want { lines( struct_type->components ) } values, got { lines( values ) }|.
              RETURN.
            ENDIF.
            CLEAR data.
            LOOP AT values INTO value.
              index = sy-tabix.
              ASSIGN COMPONENT index OF STRUCTURE data TO <component>.
              <component> = value.
            ENDLOOP.
          WHEN OTHERS.
            result = 'cannot be set'.
        ENDCASE.
      CATCH cx_sy_conversion_error INTO DATA(error).
        result = |"{ value }" does not fit: { error->get_text( ) }|.
    ENDTRY.
  ENDMETHOD.


  METHOD is_flat_row.
    DATA struct_type TYPE REF TO cl_abap_structdescr.
    DATA component TYPE abap_compdescr.

    CASE type->kind.
      WHEN cl_abap_typedescr=>kind_elem.
        result = abap_true.
      WHEN cl_abap_typedescr=>kind_struct.
        struct_type ?= type.
        LOOP AT struct_type->components INTO component.
          IF struct_type->get_component_type( component-name )->kind <> cl_abap_typedescr=>kind_elem.
            RETURN.
          ENDIF.
        ENDLOOP.
        result = abap_true.
    ENDCASE.
  ENDMETHOD.


  METHOD clean.
    result = value.
    REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>horizontal_tab IN result WITH ` `.
    REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>newline IN result WITH ` `.
    REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>cr_lf(1) IN result WITH ` `.
  ENDMETHOD.


  METHOD fail.
    zcl_vsp_adt_util=>raise_failure( resource_type = 'CHKV'
                                     resource_id   = ''
                                     text          = text ).
  ENDMETHOD.

ENDCLASS.
