"! <p class="shorttext synchronized">vsp ADT fallback: structure source check</p>
"! GET /sap/bc/adt/zvsp/ddic/check?names=A,B | ?like=Z%&amp;max=100: reads
"! structures and tables and says, per object, whether our structure
"! source agrees. Where the release has the standard's (7.50), ours is
"! held against it: the same text written, the same dictionary tables
"! read from that text. Where not (7.40), ours is held against the
"! dictionary: what it writes reads back to the fields the object has
"! (?mode=dict asks for that on 7.50 too). Reads only; nothing is written.
CLASS zcl_vsp_ddic_src_check DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    METHODS get REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_max_diffs TYPE i VALUE 5.
    "! Which checks run: r the text, s the standard's reading, p ours.
    DATA steps TYPE string VALUE 'rsp'.
    "! dict: ours against the dictionary even where the standard is there.
    DATA mode TYPE string.

    METHODS check_one
      IMPORTING name          TYPE ddobjname
      RETURNING VALUE(result) TYPE string.
    "! "FIELD CHECKTABLE" of the keys set on this level, sorted; the
    "! same for value helps.
    CLASS-METHODS key_list
      IMPORTING foreign_keys  TYPE dd08vttyp OPTIONAL
                value_helps   TYPE dd35vttyp OPTIONAL
      RETURNING VALUE(result) TYPE string_table.
    METHODS compare_lines
      IMPORTING expected      TYPE string_table
                actual        TYPE string_table
      RETURNING VALUE(result) TYPE string.
    METHODS compare_tables
      IMPORTING what          TYPE string
                expected      TYPE ANY TABLE
                actual        TYPE ANY TABLE
                only          TYPE string_table OPTIONAL
      CHANGING  diffs         TYPE string_table.
    METHODS compare_rows
      IMPORTING what     TYPE string
                expected TYPE any
                actual   TYPE any
                only     TYPE string_table OPTIONAL
      CHANGING  diffs    TYPE string_table.
ENDCLASS.



CLASS zcl_vsp_ddic_src_check IMPLEMENTATION.

  METHOD get.
    DATA names_param TYPE string.
    DATA like_param TYPE string.
    DATA max_param TYPE string.
    DATA max TYPE i VALUE 100.
    DATA names TYPE string_table.
    DATA name TYPE string.
    DATA objname TYPE ddobjname.
    DATA out TYPE string.
    DATA tabnames TYPE STANDARD TABLE OF tabname.
    DATA tabname TYPE tabname.
    DATA ok TYPE i.
    DATA bad TYPE i.
    DATA line TYPE string.

    request->get_uri_query_parameter( EXPORTING name = 'names' mandatory = abap_false
                                      IMPORTING value = names_param ).
    request->get_uri_query_parameter( EXPORTING name = 'like' mandatory = abap_false
                                      IMPORTING value = like_param ).
    request->get_uri_query_parameter( EXPORTING name = 'max' mandatory = abap_false
                                      IMPORTING value = max_param ).
    request->get_uri_query_parameter( EXPORTING name = 'steps' mandatory = abap_false
                                      IMPORTING value = steps ).
    IF steps IS INITIAL.
      steps = 'rsp'.
    ENDIF.
    request->get_uri_query_parameter( EXPORTING name = 'mode' mandatory = abap_false
                                      IMPORTING value = mode ).
    IF max_param CO '0123456789' AND max_param IS NOT INITIAL.
      max = max_param.
    ENDIF.
    IF names_param IS NOT INITIAL.
      SPLIT to_upper( names_param ) AT ',' INTO TABLE names.
    ELSEIF like_param IS NOT INITIAL.
      like_param = to_upper( like_param ).
      SELECT tabname FROM dd02l INTO TABLE tabnames UP TO max ROWS
        WHERE tabname LIKE like_param AND as4local = 'A'
          AND ( tabclass = 'INTTAB' OR tabclass = 'TRANSP' OR tabclass = 'APPEND' )
        ORDER BY tabname.
      LOOP AT tabnames INTO tabname.
        APPEND tabname TO names.
      ENDLOOP.
    ELSE.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( 'names=A,B or like=Z%' ).
    ENDIF.

    LOOP AT names INTO name.
      objname = condense( name ).
      line = check_one( objname ).
      IF line CP '*: OK'.
        ok = ok + 1.
      ELSE.
        bad = bad + 1.
      ENDIF.
      out = out && line && cl_abap_char_utilities=>newline.
    ENDLOOP.
    out = |{ out }{ ok } OK, { bad } not{ cl_abap_char_utilities=>newline }|.
    zcl_vsp_adt_util=>respond( response = response content_type = 'text/plain' body = out ).
  ENDMETHOD.


  METHOD check_one.
    DATA own TYPE REF TO zif_vsp_ddic_source.
    DATA sap TYPE REF TO zif_vsp_ddic_source.
    DATA own_source TYPE string_table.
    DATA sap_source TYPE string_table.
    DATA own_def TYPE zif_vsp_ddic_source=>ty_definition.
    DATA sap_def TYPE zif_vsp_ddic_source=>ty_definition.
    DATA dd02v TYPE dd02v.
    DATA dd03p TYPE dd03ptab.
    DATA dd08v TYPE dd08vttyp.
    DATA dd05m TYPE dd05mttyp.
    DATA dd35v TYPE dd35vttyp.
    DATA dd36m TYPE dd36mttyp.
    DATA diffs TYPE string_table.
    DATA diff TYPE string.
    DATA only TYPE string_table.
    DATA error TYPE REF TO cx_root.

    TRY.
        own = zcl_vsp_ddic_src=>get( zcl_vsp_ddic_src=>co_own ).
        own_source = own->render( name = name version = 'A' ).
        IF own_source IS INITIAL.
          result = |{ name }: not found|.
          RETURN.
        ENDIF.

        IF zcl_vsp_ddic_src_sap=>is_available( ) = abap_true AND mode <> 'dict'.
          " Ours against the standard's: the text, then the reading of it.
          sap = zcl_vsp_ddic_src=>get( zcl_vsp_ddic_src=>co_sap ).
          sap_source = sap->render( name = name version = 'A' ).
          diff = compare_lines( expected = sap_source actual = own_source ).
          IF diff IS NOT INITIAL AND steps CS 'r'.
            APPEND |source { diff }| TO diffs.
          ENDIF.
          IF steps CS 's'.
            sap_def = sap->parse( name = name source = sap_source ).
          ENDIF.
          IF steps CS 'p'.
            TRY.
                own_def = own->parse( name = name source = sap_source ).
              CATCH cx_root INTO error.
                APPEND |parse: { error->get_text( ) }| TO diffs.
            ENDTRY.
          ENDIF.
          IF own_def IS NOT INITIAL AND sap_def IS NOT INITIAL.
            compare_rows( EXPORTING what = 'header' expected = sap_def-header actual = own_def-header
                          CHANGING diffs = diffs ).
            compare_tables( EXPORTING what = 'fields' expected = sap_def-fields actual = own_def-fields
                            CHANGING diffs = diffs ).
            compare_tables( EXPORTING what = 'foreign keys' expected = sap_def-foreign_keys
                                      actual = own_def-foreign_keys
                            CHANGING diffs = diffs ).
            compare_tables( EXPORTING what = 'foreign key fields' expected = sap_def-foreign_key_fields
                                      actual = own_def-foreign_key_fields
                            CHANGING diffs = diffs ).
            compare_tables( EXPORTING what = 'value helps' expected = sap_def-value_helps
                                      actual = own_def-value_helps
                            CHANGING diffs = diffs ).
            compare_tables( EXPORTING what = 'value help fields' expected = sap_def-value_help_fields
                                      actual = own_def-value_help_fields
                            CHANGING diffs = diffs ).
          ENDIF.
        ELSE.
          " Ours against the dictionary: what it writes reads back.
          CALL FUNCTION 'DDIF_TABL_GET'
            EXPORTING
              name      = name
              state     = 'A'
              langu     = sy-langu
            IMPORTING
              dd02v_wa  = dd02v
            TABLES
              dd03p_tab = dd03p
              dd05m_tab = dd05m
              dd08v_tab = dd08v
              dd35v_tab = dd35v
              dd36m_tab = dd36m
            EXCEPTIONS
              OTHERS    = 1.
          own_def = own->parse( name = name source = own_source ).
          " The components of a nested structure are the dictionary's
          " expansion, not lines of the source.
          DELETE dd03p WHERE depth IS NOT INITIAL AND depth <> '00'.
          " What the source says of a field; the rest (check tables,
          " inherited keys, all search help parameters) the dictionary
          " derives, so it is not the reading's to agree with.
          APPEND 'FIELDNAME' TO only.
          APPEND 'POSITION' TO only.
          APPEND 'KEYFLAG' TO only.
          APPEND 'ROLLNAME' TO only.
          APPEND 'DATATYPE' TO only.
          APPEND 'LENG' TO only.
          APPEND 'DECIMALS' TO only.
          APPEND 'COMPTYPE' TO only.
          APPEND 'REFTYPE' TO only.
          APPEND 'PRECFIELD' TO only.
          APPEND 'GROUPNAME' TO only.
          APPEND 'ADMINFIELD' TO only.
          compare_tables( EXPORTING what = 'fields' expected = dd03p actual = own_def-fields only = only
                          CHANGING diffs = diffs ).
          diff = compare_lines( expected = key_list( foreign_keys = dd08v )
                                actual   = key_list( foreign_keys = own_def-foreign_keys ) ).
          IF diff IS NOT INITIAL.
            APPEND |foreign keys { diff }| TO diffs.
          ENDIF.
          diff = compare_lines( expected = key_list( value_helps = dd35v )
                                actual   = key_list( value_helps = own_def-value_helps ) ).
          IF diff IS NOT INITIAL.
            APPEND |value helps { diff }| TO diffs.
          ENDIF.
        ENDIF.
      CATCH cx_root INTO error.
        APPEND |error: { error->get_text( ) }| TO diffs.
    ENDTRY.

    IF diffs IS INITIAL.
      result = |{ name }: OK|.
    ELSE.
      CONCATENATE LINES OF diffs INTO diff SEPARATED BY ` \| `.
      result = |{ name }: { diff }|.
    ENDIF.
  ENDMETHOD.


  METHOD key_list.
    DATA fk TYPE dd08v.
    DATA vh TYPE dd35v.
    LOOP AT foreign_keys INTO fk
        WHERE ( noinherit = space OR noinherit = 'Y' OR noinherit = 'E' ) AND checktable <> '*'.
      APPEND |{ fk-fieldname } { fk-checktable }| TO result.
    ENDLOOP.
    " A search help of the table itself (no field) is not in the source.
    LOOP AT value_helps INTO vh
        WHERE ( shlpinher = space OR shlpinher = 'E' ) AND shlpname <> '*' AND fieldname IS NOT INITIAL.
      APPEND |{ vh-fieldname } { vh-shlpname }| TO result.
    ENDLOOP.
    SORT result.
  ENDMETHOD.


  METHOD compare_lines.
    DATA count TYPE i.
    DATA i TYPE i.
    DATA e TYPE string.
    DATA a TYPE string.

    count = lines( expected ).
    IF lines( actual ) > count.
      count = lines( actual ).
    ENDIF.
    DO count TIMES.
      i = sy-index.
      CLEAR: e, a.
      READ TABLE expected INDEX i INTO e.
      READ TABLE actual INDEX i INTO a.
      IF e <> a.
        result = |line { i }: "{ e }" / ours "{ a }"|.
        RETURN.
      ENDIF.
    ENDDO.
  ENDMETHOD.


  METHOD compare_tables.
    DATA count TYPE i.
    DATA i TYPE i.
    FIELD-SYMBOLS <e> TYPE any.
    FIELD-SYMBOLS <a> TYPE any.
    FIELD-SYMBOLS <e_tab> TYPE INDEX TABLE.
    FIELD-SYMBOLS <a_tab> TYPE INDEX TABLE.

    ASSIGN expected TO <e_tab>.
    ASSIGN actual TO <a_tab>.
    IF lines( <e_tab> ) <> lines( <a_tab> ).
      APPEND |{ what }: { lines( <e_tab> ) } rows / ours { lines( <a_tab> ) }| TO diffs.
    ENDIF.
    count = lines( <e_tab> ).
    IF lines( <a_tab> ) < count.
      count = lines( <a_tab> ).
    ENDIF.
    DO count TIMES.
      i = sy-index.
      READ TABLE <e_tab> INDEX i ASSIGNING <e>.
      READ TABLE <a_tab> INDEX i ASSIGNING <a>.
      compare_rows( EXPORTING what = |{ what } row { i }| expected = <e> actual = <a> only = only
                    CHANGING diffs = diffs ).
      IF lines( diffs ) >= co_max_diffs.
        RETURN.
      ENDIF.
    ENDDO.
  ENDMETHOD.


  METHOD compare_rows.
    DATA descr TYPE REF TO cl_abap_structdescr.
    DATA component TYPE abap_compdescr.
    DATA e_text TYPE string.
    DATA a_text TYPE string.
    FIELD-SYMBOLS <e> TYPE any.
    FIELD-SYMBOLS <a> TYPE any.

    descr ?= cl_abap_typedescr=>describe_by_data( expected ).
    LOOP AT descr->components INTO component.
      IF only IS NOT INITIAL.
        READ TABLE only TRANSPORTING NO FIELDS WITH KEY table_line = component-name.
        IF sy-subrc <> 0.
          CONTINUE.
        ENDIF.
      ENDIF.
      ASSIGN COMPONENT component-name OF STRUCTURE expected TO <e>.
      CHECK sy-subrc = 0.
      ASSIGN COMPONENT component-name OF STRUCTURE actual TO <a>.
      CHECK sy-subrc = 0.
      IF <e> <> <a>.
        e_text = <e>.
        a_text = <a>.
        APPEND |{ what } { component-name }: "{ e_text }" / ours "{ a_text }"| TO diffs.
        IF lines( diffs ) >= co_max_diffs.
          RETURN.
        ENDIF.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.
ENDCLASS.
