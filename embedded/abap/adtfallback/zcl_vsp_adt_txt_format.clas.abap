"! <p class="shorttext synchronized">vsp ADT fallback: text pool formats</p>
"! The three plain-text documents of /sap/bc/adt/textelements, in the form
"! the standard resource of newer releases reads and writes them:
"! <ul>
"! <li>symbols: "@MaxLength:N" then "KEY=text", a blank line between;</li>
"! <li>selections: "KEY     =text", "@DDICReference" above a text taken
"!     from the dictionary, a blank line between;</li>
"! <li>headings: "listHeader=" and "columnHeader_1=" .. "_4=".</li>
"! </ul>
CLASS zcl_vsp_adt_txt_format DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES if_adt_rest_content_handler.

    CONSTANTS:
      BEGIN OF co_kind,
        symbols    TYPE string VALUE 'symbols',
        selections TYPE string VALUE 'selections',
        headings   TYPE string VALUE 'headings',
      END OF co_kind.

    "! The first 8 characters of a selection text say where it comes from.
    CONSTANTS co_entry_key_ddic TYPE c LENGTH 8 VALUE 'D       '.
    CONSTANTS co_entry_key_non_ddic TYPE c LENGTH 8 VALUE '        '.

    TYPES:
      BEGIN OF ty_entry,
        key    TYPE textpool-key,
        entry  TYPE textpool-entry,
        length TYPE textpool-length,
      END OF ty_entry,
      tt_entries TYPE STANDARD TABLE OF ty_entry WITH DEFAULT KEY,
      BEGIN OF ty_document,
        symbols         TYPE tt_entries,
        selections      TYPE tt_entries,
        list_header     TYPE string,
        column_header_1 TYPE string,
        column_header_2 TYPE string,
        column_header_3 TYPE string,
        column_header_4 TYPE string,
      END OF ty_document.

    CLASS-METHODS is_kind
      IMPORTING kind          TYPE csequence
      RETURNING VALUE(result) TYPE abap_bool.

    CLASS-METHODS content_type
      IMPORTING kind          TYPE csequence
      RETURNING VALUE(result) TYPE string.

    "! Puts a plain text into SY-MSG* (message 00/001), for
    "! CX_ADT_REST=>CREATE_TEXTID_FROM_MSG_PARAMS.
    CLASS-METHODS set_message
      IMPORTING text TYPE csequence.

    "! Raises CX_ADT_RES_NOT_ACCEPTABLE with a plain text.
    CLASS-METHODS raise_not_acceptable
      IMPORTING text TYPE csequence
      RAISING   cx_adt_res_not_acceptable.

    METHODS constructor
      IMPORTING kind TYPE csequence.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_max_length TYPE string VALUE '@MaxLength:'.
    CONSTANTS co_ddic_reference TYPE string VALUE '@DDICReference'.

    DATA kind TYPE string.

    METHODS lines_of
      IMPORTING entity        TYPE REF TO if_rest_entity
      RETURNING VALUE(result) TYPE string_table.

    METHODS parse_symbols
      IMPORTING lines         TYPE string_table
      RETURNING VALUE(result) TYPE tt_entries
      RAISING   cx_adt_res_not_acceptable.

    METHODS parse_selections
      IMPORTING lines         TYPE string_table
      RETURNING VALUE(result) TYPE tt_entries
      RAISING   cx_adt_res_not_acceptable.

    METHODS parse_headings
      IMPORTING lines    TYPE string_table
      CHANGING  document TYPE ty_document
      RAISING   cx_adt_res_not_acceptable.

    METHODS render
      IMPORTING document      TYPE ty_document
      RETURNING VALUE(result) TYPE string.
ENDCLASS.



CLASS zcl_vsp_adt_txt_format IMPLEMENTATION.

  METHOD is_kind.
    IF kind = co_kind-symbols OR kind = co_kind-selections OR kind = co_kind-headings.
      result = abap_true.
    ENDIF.
  ENDMETHOD.


  METHOD content_type.
    result = |application/vnd.sap.adt.textelements.{ kind }.v1|.
  ENDMETHOD.


  METHOD set_message.
    DATA rest TYPE string.
    DATA v1 TYPE symsgv.
    DATA v2 TYPE symsgv.
    DATA v3 TYPE symsgv.
    DATA v4 TYPE symsgv.
    DATA dummy TYPE string ##NEEDED.

    rest = text.
    v1 = rest.
    IF strlen( rest ) > 50.
      rest = rest+50.
      v2 = rest.
      IF strlen( rest ) > 50.
        rest = rest+50.
        v3 = rest.
        IF strlen( rest ) > 50.
          rest = rest+50.
          v4 = rest.
        ENDIF.
      ENDIF.
    ENDIF.
    MESSAGE e001(00) WITH v1 v2 v3 v4 INTO dummy.
  ENDMETHOD.


  METHOD raise_not_acceptable.
    set_message( text ).
    RAISE EXCEPTION TYPE cx_adt_res_not_acceptable
      EXPORTING
        textid = cx_adt_rest=>create_textid_from_msg_params( ).
  ENDMETHOD.


  METHOD constructor.
    me->kind = kind.
  ENDMETHOD.


  METHOD if_adt_rest_content_handler~get_supported_content_type.
    result = content_type( kind ).
  ENDMETHOD.


  METHOD if_adt_rest_content_handler~serialize.
    DATA document TYPE ty_document.
    DATA body TYPE xstring.
    DATA params TYPE tihttpnvp.
    DATA param TYPE ihttpnvp.

    document = data.
    body = cl_abap_codepage=>convert_to( render( document ) ).

    param-name = 'charset'.
    param-value = 'UTF-8'.
    APPEND param TO params.
    response_entity->set_content_type(
      iv_media_type = content_type( kind )
      it_parameter  = params ).
    response_entity->set_binary_data( body ).
  ENDMETHOD.


  METHOD if_adt_rest_content_handler~deserialize.
    DATA document TYPE ty_document.
    DATA lines TYPE string_table.

    lines = lines_of( request_entity ).
    CASE kind.
      WHEN co_kind-symbols.
        document-symbols = parse_symbols( lines ).
      WHEN co_kind-selections.
        document-selections = parse_selections( lines ).
      WHEN co_kind-headings.
        parse_headings( EXPORTING lines = lines CHANGING document = document ).
    ENDCASE.
    data = document.
  ENDMETHOD.


  METHOD lines_of.
    DATA text TYPE string.

    text = cl_abap_codepage=>convert_from(
      source      = entity->get_binary_data( )
      codepage    = 'UTF-8'
      ignore_cerr = abap_true ).
    REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>cr_lf IN text
      WITH cl_abap_char_utilities=>newline.
    REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>cr_lf(1) IN text
      WITH cl_abap_char_utilities=>newline.
    SPLIT text AT cl_abap_char_utilities=>newline INTO TABLE result.
  ENDMETHOD.


  METHOD parse_symbols.
    DATA line TYPE string.
    DATA key TYPE string.
    DATA entry TYPE ty_entry.
    DATA length TYPE i.
    DATA number TYPE string.

    LOOP AT lines INTO line.
      IF line IS INITIAL.
        CONTINUE.
      ENDIF.

      IF find( val = line sub = co_max_length ) = 0.
        number = condense( substring_after( val = line sub = ':' ) ).
        IF number IS INITIAL OR number CN '0123456789'.
          raise_not_acceptable( |Not a length: { line }| ).
        ENDIF.
        length = number.
        IF length > 132.
          raise_not_acceptable( |@MaxLength must not exceed 132: { line }| ).
        ENDIF.
        CONTINUE.
      ENDIF.

      IF line(1) = '@'.
        raise_not_acceptable( |Invalid annotation { line }; only @MaxLength:n is allowed| ).
      ENDIF.

      IF line NS '='.
        raise_not_acceptable( |Cannot parse the line { line }; KEY=text expected| ).
      ENDIF.

      key = condense( to_upper( substring_before( val = line sub = '=' ) ) ).
      IF key IS INITIAL OR strlen( key ) > 3.
        raise_not_acceptable( |Text symbol key { key } must have 1 to 3 characters| ).
      ENDIF.

      CLEAR entry.
      entry-key = key.
      entry-entry = substring_after( val = line sub = '=' ).
      IF length = 0.
        length = strlen( entry-entry ).
      ENDIF.
      IF strlen( entry-entry ) > length.
        raise_not_acceptable( |Text symbol { key } is longer than its @MaxLength { length }| ).
      ENDIF.
      entry-length = length.
      APPEND entry TO result.
      CLEAR length.
    ENDLOOP.
  ENDMETHOD.


  METHOD parse_selections.
    DATA line TYPE string.
    DATA key TYPE string.
    DATA text TYPE string.
    DATA entry TYPE ty_entry.
    DATA from_ddic TYPE abap_bool.

    LOOP AT lines INTO line.
      IF line IS INITIAL.
        CONTINUE.
      ENDIF.

      IF line = co_ddic_reference.
        from_ddic = abap_true.
        CONTINUE.
      ENDIF.

      IF line NS '='.
        raise_not_acceptable( |Cannot parse the line { line }; KEY=text expected| ).
      ENDIF.

      key = condense( to_upper( substring_before( val = line sub = '=' ) ) ).
      IF key IS INITIAL OR strlen( key ) > 8.
        raise_not_acceptable( |Selection text key { key } must have 1 to 8 characters| ).
      ENDIF.
      text = substring_after( val = line sub = '=' ).
      IF strlen( text ) > 30.
        raise_not_acceptable( |Selection text { key } is longer than 30 characters| ).
      ENDIF.

      CLEAR entry.
      entry-key = key.
      IF from_ddic = abap_true.
        CONCATENATE co_entry_key_ddic text INTO entry-entry RESPECTING BLANKS.
      ELSE.
        CONCATENATE co_entry_key_non_ddic text INTO entry-entry RESPECTING BLANKS.
      ENDIF.
      APPEND entry TO result.
      from_ddic = abap_false.
    ENDLOOP.
  ENDMETHOD.


  METHOD parse_headings.
    DATA line TYPE string.
    DATA key TYPE string.
    DATA text TYPE string.

    LOOP AT lines INTO line.
      IF line IS INITIAL OR line NS '='.
        CONTINUE.
      ENDIF.
      key = condense( to_upper( substring_before( val = line sub = '=' ) ) ).
      text = substring_after( val = line sub = '=' ).
      CASE key.
        WHEN 'LISTHEADER'.
          document-list_header = text.
        WHEN 'COLUMNHEADER_1'.
          document-column_header_1 = text.
        WHEN 'COLUMNHEADER_2'.
          document-column_header_2 = text.
        WHEN 'COLUMNHEADER_3'.
          document-column_header_3 = text.
        WHEN 'COLUMNHEADER_4'.
          document-column_header_4 = text.
        WHEN OTHERS.
          raise_not_acceptable( |Invalid heading key { key }| ).
      ENDCASE.
    ENDLOOP.
  ENDMETHOD.


  METHOD render.
    DATA crlf TYPE string.
    DATA entry TYPE ty_entry.
    DATA key8 TYPE c LENGTH 8.
    DATA text TYPE string.

    crlf = cl_abap_char_utilities=>cr_lf.

    CASE kind.
      WHEN co_kind-symbols.
        LOOP AT document-symbols INTO entry.
          IF result IS NOT INITIAL.
            result = result && crlf && crlf.
          ENDIF.
          result = |{ result }{ co_max_length }{ entry-length }{ crlf }| &&
                   |{ to_upper( entry-key ) }={ entry-entry }|.
        ENDLOOP.

      WHEN co_kind-selections.
        LOOP AT document-selections INTO entry.
          IF result IS NOT INITIAL.
            result = result && crlf && crlf.
          ENDIF.
          IF entry-entry(8) = co_entry_key_ddic.
            result = result && co_ddic_reference && crlf.
          ENDIF.
          key8 = to_upper( entry-key ).
          text = entry-entry+8.
          CONCATENATE result key8 '=' text INTO result RESPECTING BLANKS.
        ENDLOOP.

      WHEN co_kind-headings.
        result = |listHeader={ document-list_header }{ crlf }{ crlf }| &&
                 |columnHeader_1={ document-column_header_1 }{ crlf }| &&
                 |columnHeader_2={ document-column_header_2 }{ crlf }| &&
                 |columnHeader_3={ document-column_header_3 }{ crlf }| &&
                 |columnHeader_4={ document-column_header_4 }|.
    ENDCASE.
  ENDMETHOD.

ENDCLASS.
