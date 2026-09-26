"! <p class="shorttext synchronized">vsp ADT fallback: database tables</p>
"! /sap/bc/adt/zvsp/ddic/tables[/{name}[/source/main]], the source-based
"! table editing of newer releases, for transparent tables:
"! <ul>
"! <li>POST .../tables?corrNr=..: creates an empty table (a blueSource
"!     document names it, its description and package), with default
"!     technical settings, as the standard does;</li>
"! <li>POST .../{name}?_action=LOCK|UNLOCK; DELETE .../{name};</li>
"! <li>GET .../{name}: the object document (name, description, package);</li>
"! <li>GET/PUT .../{name}/source/main: the "define table" source.</li>
"! </ul>
"! The fields are the dictionary's own business: 7.50 already edits
"! structures as source (CL_SBD_STRUCTURE_PARSER, _PERSIST), and a table
"! body is a structure body. Only the header differs: "define table", and
"! the table annotations tableCategory, deliveryClass, dataMaintenance,
"! which are taken off here and set on DD02V. A write stores the inactive
"! version (DDIF_TABL_PUT) for /sap/bc/adt/activation. An activation that
"! needs a table conversion fails with the dictionary's message; nothing
"! here converts a table.
CLASS zcl_vsp_adt_tabl_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'application/vnd.sap.adt.tables.v2+xml'.
    CONSTANTS co_source_type TYPE string VALUE 'text/plain'.

    METHODS get REDEFINITION.
    METHODS post REDEFINITION.
    METHODS put REDEFINITION.
    METHODS delete REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    TYPES:
      BEGIN OF ty_table_props,
        category    TYPE string,
        delivery    TYPE string,
        maintenance TYPE string,
      END OF ty_table_props.

    DATA name TYPE ddobjname.
    DATA is_source TYPE abap_bool.

    METHODS parse_uri
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    METHODS read_header
      IMPORTING state         TYPE ddobjstate DEFAULT 'M'
      RETURNING VALUE(result) TYPE dd02v
      RAISING   cx_adt_rest.

    METHODS get_source
      IMPORTING version       TYPE r3state
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_rest.

    METHODS object_document
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_rest.

    METHODS create
      IMPORTING request  TYPE REF TO if_adt_rest_request
                response TYPE REF TO if_adt_rest_response
      RAISING   cx_adt_rest.

    METHODS lock
      IMPORTING request  TYPE REF TO if_adt_rest_request
                response TYPE REF TO if_adt_rest_response
      RAISING   cx_adt_rest.

    METHODS unlock.

    METHODS check_handle
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    METHODS corr_insert
      IMPORTING request TYPE REF TO if_adt_rest_request
                devclass TYPE devclass OPTIONAL
                mode    TYPE csequence DEFAULT 'MODIFY'
      RAISING   cx_adt_rest.

    METHODS write_source
      IMPORTING lines TYPE string_table
      RAISING   cx_adt_rest.

    CLASS-METHODS to_annotation
      IMPORTING value         TYPE csequence
                kind          TYPE string
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS from_annotation
      IMPORTING value         TYPE csequence
                kind          TYPE string
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS esc
      IMPORTING value         TYPE csequence
      RETURNING VALUE(result) TYPE string.
ENDCLASS.



CLASS zcl_vsp_adt_tabl_resource IMPLEMENTATION.

  METHOD get.
    DATA version_param TYPE if_adt_tools_core_types=>ty_object-version.
    DATA version TYPE r3state.

    parse_uri( request ).
    IF is_source = abap_false.
      zcl_vsp_adt_util=>respond( response     = response
                                 content_type = co_content_type
                                 body         = object_document( ) ).
      RETURN.
    ENDIF.

    request->get_uri_query_parameter(
      EXPORTING name      = 'version'
                mandatory = abap_false
      IMPORTING value     = version_param ).
    TRANSLATE version_param TO LOWER CASE.
    IF version_param = 'active'.
      version = 'A'.
    ELSE.
      " The working area: an inactive version if there is one.
      version = 'I'.
    ENDIF.
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = co_source_type
                               body         = get_source( version ) ).
  ENDMETHOD.


  METHOD post.
    DATA action TYPE string.

    request->get_uri_query_parameter(
      EXPORTING name      = '_action'
                mandatory = abap_false
      IMPORTING value     = action ).
    TRANSLATE action TO UPPER CASE.
    IF action IS INITIAL.
      create( request = request response = response ).
      RETURN.
    ENDIF.

    parse_uri( request ).
    read_header( ).
    CASE action.
      WHEN 'LOCK'.
        lock( request = request response = response ).
      WHEN 'UNLOCK'.
        unlock( ).
      WHEN OTHERS.
        RAISE EXCEPTION TYPE cx_adt_res_meth_not_supported.
    ENDCASE.
  ENDMETHOD.


  METHOD put.
    parse_uri( request ).
    IF is_source = abap_false.
      RAISE EXCEPTION TYPE cx_adt_res_meth_not_supported.
    ENDIF.
    read_header( ).
    check_handle( request ).
    corr_insert( request ).
    write_source( zcl_vsp_adt_util=>request_lines( request ) ).
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = co_source_type
                               body         = get_source( 'I' ) ).
  ENDMETHOD.


  METHOD delete.
    DATA corrnr TYPE trkorr.
    DATA objname TYPE rsedd0-ddobjname.

    parse_uri( request ).
    read_header( ).
    check_handle( request ).
    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).

    " RS_DD_DELETE_OBJ takes the lock itself and would find ours in its way
    " ("already being edited"): the handle has been checked, hand it over.
    unlock( ).
    objname = name.
    CALL FUNCTION 'RS_DD_DELETE_OBJ'
      EXPORTING
        no_ask               = abap_true
        objname              = objname
        objtype              = 'T'
      CHANGING
        corrnum              = corrnr
      EXCEPTIONS
        not_executed         = 1
        object_not_found     = 2
        object_not_specified = 3
        permission_failure   = 4
        dialog_needed        = 5
        OTHERS               = 6.
    IF sy-subrc = 5.
      " The dictionary would ask first: the table holds data or is used.
      " Nothing is deleted without that question being answered in SE11.
      DATA sap_text TYPE string.
      MESSAGE ID sy-msgid TYPE 'S' NUMBER sy-msgno
        WITH sy-msgv1 sy-msgv2 sy-msgv3 sy-msgv4 INTO sap_text.
      zcl_vsp_adt_util=>raise_failure(
        resource_type = 'TABL' resource_id = name
        text = |{ name } was not deleted: it holds data or is in use, | &&
               |and the dictionary asks before deleting it (SE11). { sap_text }| ).
    ELSEIF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = 'TABL' resource_id = name ).
    ENDIF.
    COMMIT WORK.
  ENDMETHOD.


  METHOD parse_uri.
    DATA value TYPE string.
    DATA uri TYPE string.

    request->get_uri_attribute(
      EXPORTING name      = 'name'
                mandatory = abap_true
      IMPORTING value     = value ).
    value = to_upper( cl_http_utility=>unescape_url( value ) ).
    IF strlen( value ) > 30.
      RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDIF.
    name = value.

    uri = request->get_inner_rest_request( )->get_uri( ).
    SPLIT uri AT '?' INTO uri value.
    IF uri CP '*/source/main'.
      is_source = abap_true.
    ENDIF.
  ENDMETHOD.


  METHOD read_header.
    DATA gotstate TYPE ddgotstate.

    CALL FUNCTION 'DDIF_TABL_GET'
      EXPORTING
        name          = name
        state         = state
        langu         = sy-langu
      IMPORTING
        gotstate      = gotstate
        dd02v_wa      = result
      EXCEPTIONS
        illegal_input = 1
        OTHERS        = 2.
    IF sy-subrc <> 0 OR gotstate IS INITIAL.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'TABL'
          resource_id   = CONV #( name ).
    ENDIF.
    IF result-tabclass <> 'TRANSP'.
      zcl_vsp_adt_txt_format=>raise_not_acceptable(
        |{ name } is a { result-tabclass } table; only transparent tables are served here| ).
    ENDIF.
  ENDMETHOD.


  METHOD get_source.
    DATA persist TYPE REF TO cl_sbd_structure_persist.
    DATA lines TYPE string_table.
    DATA line TYPE string.
    DATA header TYPE dd02v.
    DATA key TYPE seu_objkey.
    DATA props TYPE string.
    DATA done TYPE abap_bool.

    IF version = 'A'.
      header = read_header( 'A' ).
    ELSE.
      header = read_header( 'M' ).
    ENDIF.

    CREATE OBJECT persist.
    key = name.
    persist->get_source( EXPORTING i_object_key = key
                                   i_version    = version
                         IMPORTING e_source     = lines ).
    IF lines IS INITIAL AND version = 'I'.
      persist->get_source( EXPORTING i_object_key = key
                                     i_version    = 'A'
                           IMPORTING e_source     = lines ).
    ENDIF.

    props = |@AbapCatalog.tableCategory : #TRANSPARENT{ cl_abap_char_utilities=>newline }| &&
            |@AbapCatalog.deliveryClass : #{ header-contflag }{ cl_abap_char_utilities=>newline }| &&
            |@AbapCatalog.dataMaintenance : #{ to_annotation( value = header-mainflag kind = 'MAINFLAG' ) }|.

    " The structure source of 7.50 with the header of a table: the table
    " annotations go right before "define".
    LOOP AT lines INTO line.
      IF done = abap_false.
        REPLACE FIRST OCCURRENCE OF '@AbapCatalog.enhancementCategory'
          IN line WITH '@AbapCatalog.enhancement.category'.
        IF to_lower( condense( line ) ) CP 'define type *'.
          result = result && props && cl_abap_char_utilities=>newline.
          REPLACE FIRST OCCURRENCE OF REGEX '^(\s*)define\s+type\s' IN line
            WITH `$1define table ` IGNORING CASE.
          done = abap_true.
        ENDIF.
      ENDIF.
      result = result && line && cl_abap_char_utilities=>newline.
    ENDLOOP.
    IF result IS INITIAL.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'TABL'
          resource_id   = CONV #( name ).
    ENDIF.
  ENDMETHOD.


  METHOD object_document.
    DATA header TYPE dd02v.
    DATA devclass TYPE devclass.
    DATA masterlang TYPE masterlang.
    DATA author TYPE responsibl.
    DATA iso TYPE laiso.
    DATA master_iso TYPE laiso.

    header = read_header( ).
    SELECT SINGLE devclass masterlang author FROM tadir
      INTO (devclass, masterlang, author)
      WHERE pgmid = 'R3TR' AND object = 'TABL' AND obj_name = name.
    CALL FUNCTION 'CONVERSION_EXIT_ISOLA_OUTPUT'
      EXPORTING
        input  = sy-langu
      IMPORTING
        output = iso.
    CALL FUNCTION 'CONVERSION_EXIT_ISOLA_OUTPUT'
      EXPORTING
        input  = masterlang
      IMPORTING
        output = master_iso.

    result =
      |<?xml version="1.0" encoding="utf-8"?>| &&
      |<blue:blueSource xmlns:blue="http://www.sap.com/wbobj/blue" | &&
      |xmlns:adtcore="http://www.sap.com/adt/core" | &&
      |adtcore:name="{ esc( header-tabname ) }" adtcore:type="TABL/DT" | &&
      |adtcore:description="{ esc( header-ddtext ) }" adtcore:descriptionTextLimit="60" | &&
      |adtcore:language="{ iso }" adtcore:masterLanguage="{ master_iso }" | &&
      |adtcore:responsible="{ esc( author ) }" adtcore:version="active">| &&
      |<atom:link xmlns:atom="http://www.w3.org/2005/Atom" href="source/main" | &&
      |rel="http://www.sap.com/adt/relations/source" type="text/plain"/>| &&
      |<adtcore:packageRef adtcore:name="{ esc( devclass ) }" adtcore:type="DEVC/K"/>| &&
      |</blue:blueSource>|.
  ENDMETHOD.


  METHOD create.
    DATA xml TYPE string.
    DATA value TYPE string.
    DATA description TYPE string.
    DATA devclass TYPE devclass.
    DATA dd02v TYPE dd02v.
    DATA dd09v TYPE dd09v.
    DATA exists TYPE tabname.

    xml = request->get_inner_rest_request( )->get_entity( )->get_string_data( ).
    FIND REGEX 'adtcore:name="([^"]*)"' IN xml SUBMATCHES value.
    value = to_upper( value ).
    IF value IS INITIAL OR strlen( value ) > 30.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( 'The table has no valid adtcore:name' ).
    ENDIF.
    name = value.
    FIND REGEX 'adtcore:description="([^"]*)"' IN xml SUBMATCHES description.
    FIND REGEX 'packageRef[^>]*adtcore:name="([^"]*)"' IN xml SUBMATCHES value.
    devclass = to_upper( value ).
    IF devclass IS INITIAL.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( 'The table names no package (adtcore:packageRef)' ).
    ENDIF.

    SELECT SINGLE tabname FROM dd02l INTO exists WHERE tabname = name.
    IF sy-subrc = 0.
      zcl_vsp_adt_txt_format=>set_message( |Table { name } already exists| ).
      RAISE EXCEPTION TYPE cx_adt_res_already_exists
        EXPORTING
          textid = cx_adt_rest=>create_textid_from_msg_params( ).
    ENDIF.

    corr_insert( request = request devclass = devclass mode = 'INSERT' ).

    dd02v-tabname = name.
    dd02v-ddlanguage = sy-langu.
    dd02v-tabclass = 'TRANSP'.
    dd02v-ddtext = description.
    dd02v-contflag = 'A'.
    dd02v-exclass = '1'.
    dd09v-tabname = name.
    dd09v-tabkat = '0'.
    dd09v-tabart = 'APPL0'.
    dd09v-bufallow = 'N'.
    CALL FUNCTION 'DDIF_TABL_PUT'
      EXPORTING
        name              = name
        dd02v_wa          = dd02v
        dd09l_wa          = dd09v
      EXCEPTIONS
        tabl_not_found    = 1
        name_inconsistent = 2
        tabl_inconsistent = 3
        put_failure       = 4
        put_refused       = 5
        OTHERS            = 6.
    IF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = 'TABL' resource_id = name ).
    ENDIF.
    CALL FUNCTION 'RS_INSERT_INTO_WORKING_AREA'
      EXPORTING
        object   = 'TABL'
        obj_name = CONV trobj_name( name )
      EXCEPTIONS
        OTHERS   = 1.
    COMMIT WORK.

    response->set_status( cl_rest_status_code=>gc_success_created ).
    response->set_location( |{ zif_vsp_adt_feature=>co_root }/ddic/tables/| &&
                            |{ cl_http_utility=>escape_url( to_lower( name ) ) }| ).
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = co_content_type
                               body         = object_document( ) ).
  ENDMETHOD.


  METHOD lock.
    DATA result TYPE sadt_object_lock_result.
    DATA transport TYPE sadt_object_lock_result.
    DATA object TYPE c LENGTH 34.

    object = |TABL{ name }|.
    CALL FUNCTION 'RS_ACCESS_PERMISSION'
      EXPORTING
        authority_check                = abap_true
        global_lock                    = abap_true
        mode                           = 'MODIFY'
        object                         = object
        object_class                   = 'DICT'
        suppress_corr_check            = abap_true
        suppress_corr_check_altogether = abap_true
        suppress_language_check        = abap_true
        suppress_extend_dialog         = abap_true
      EXCEPTIONS
        canceled_in_corr               = 1
        enqueued_by_user               = 2
        enqueue_system_failure         = 3
        illegal_parameter_values       = 4
        locked_by_author               = 5
        no_modify_permission           = 6
        no_show_permission             = 7
        permission_failure             = 8
        OTHERS                         = 9.
    IF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = 'TABL' resource_id = name ).
    ENDIF.

    result-lock_handle = zcl_vsp_adt_util=>lock_handle( name ).
    IF result-lock_handle IS INITIAL.
      unlock( ).
      zcl_vsp_adt_util=>raise_failure( resource_type = 'TABL' resource_id = name
                                       text = |No lock entry for { name } after locking it| ).
    ENDIF.
    transport = cl_adt_cts_management=>get_transport_info(
      pgmid    = 'R3TR'
      obj_type = 'TABL'
      obj_name = CONV #( name ) ).
    result-corrnr = transport-corrnr.
    result-corruser = transport-corruser.
    result-corrtext = transport-corrtext.
    result-is_link_up = transport-is_link_up.
    result-is_local = cl_adt_cts_management=>is_transport_disabled_for_obj(
      object   = 'TABL'
      obj_name = CONV #( name ) ).
    response->set_body_data(
      content_handler = cl_adt_rest_comp_cnt_handler=>create(
        request         = request
        content_handler = cl_adt_rest_cnt_hdl_factory=>get_instance( )->get_handler_for_asxml( ) )
      data            = result ).
  ENDMETHOD.


  METHOD unlock.
    DATA object TYPE c LENGTH 34.

    object = |TABL{ name }|.
    CALL FUNCTION 'RS_ACCESS_PERMISSION'
      EXPORTING
        authority_check = space
        global_lock     = abap_true
        mode            = 'FREE'
        object          = object
        object_class    = 'DICT'
      EXCEPTIONS
        OTHERS          = 1.
  ENDMETHOD.


  METHOD check_handle.
    DATA handle TYPE string.

    request->get_uri_query_parameter(
      EXPORTING name      = 'lockHandle'
                mandatory = abap_true
      IMPORTING value     = handle ).
    IF handle IS INITIAL OR handle <> zcl_vsp_adt_util=>lock_handle( name ).
      RAISE EXCEPTION TYPE cx_adt_res_invalid_lock_handle
        EXPORTING
          resource_type = 'TABL'
          resource_id   = CONV #( name )
          lock_handle   = handle.
    ENDIF.
  ENDMETHOD.


  METHOD corr_insert.
    DATA corrnr TYPE trkorr.
    DATA package TYPE devclass.
    DATA object TYPE c LENGTH 34.

    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).
    package = devclass.
    IF package IS INITIAL.
      SELECT SINGLE devclass FROM tadir INTO package
        WHERE pgmid = 'R3TR' AND object = 'TABL' AND obj_name = name.
    ENDIF.

    object = |TABL{ name }|.
    CALL FUNCTION 'RS_CORR_INSERT'
      EXPORTING
        object              = object
        object_class        = 'DICT'
        mode                = mode
        global_lock         = abap_true
        devclass            = package
        korrnum             = corrnr
        master_language     = sy-langu
        suppress_dialog     = abap_true
      EXCEPTIONS
        cancelled           = 1
        permission_failure  = 2
        unknown_objectclass = 3
        OTHERS              = 4.
    IF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = 'TABL' resource_id = name ).
    ENDIF.
  ENDMETHOD.


  METHOD write_source.
    DATA body TYPE string_table.
    DATA line TYPE string.
    DATA key TYPE string.
    DATA value TYPE string.
    DATA props TYPE ty_table_props.
    DATA in_header TYPE abap_bool VALUE abap_true.
    DATA logger TYPE REF TO cl_wb_checklist.
    DATA parser TYPE REF TO cl_sbd_structure_parser.
    DATA message TYPE rsfb_source.
    DATA message_line TYPE LINE OF rsfb_source.
    DATA text TYPE string.
    DATA error_line TYPE i.
    DATA dd02v TYPE dd02v.
    DATA old TYPE dd02v.
    DATA dd09v TYPE dd09v.
    DATA dd03p TYPE dd03ptab.
    DATA dd05m TYPE dd05mttyp.
    DATA dd08v TYPE dd08vttyp.
    DATA dd35v TYPE dd35vttyp.
    DATA dd36m TYPE dd36mttyp.

    " Take the table header off: the table annotations become DD02V
    " fields, "define table" becomes the "define type" of a structure.
    LOOP AT lines INTO line.
      IF in_header = abap_true.
        IF line CS '@AbapCatalog.tableCategory' OR line CS '@AbapCatalog.deliveryClass'
            OR line CS '@AbapCatalog.dataMaintenance' OR line CS '@AbapCatalog.activationType'.
          key = condense( substring_before( val = line sub = ':' ) ).
          value = condense( substring_after( val = line sub = ':' ) ).
          SHIFT value LEFT DELETING LEADING '#'.
          value = to_upper( value ).
          CASE key.
            WHEN '@AbapCatalog.tableCategory'.
              props-category = value.
            WHEN '@AbapCatalog.deliveryClass'.
              props-delivery = value.
            WHEN '@AbapCatalog.dataMaintenance'.
              props-maintenance = value.
          ENDCASE.
          CONTINUE.
        ENDIF.
        REPLACE FIRST OCCURRENCE OF '@AbapCatalog.enhancement.category'
          IN line WITH '@AbapCatalog.enhancementCategory'.
        IF to_lower( condense( line ) ) CP 'define table *'.
          REPLACE FIRST OCCURRENCE OF REGEX '^(\s*)define\s+table\s' IN line
            WITH `$1define type ` IGNORING CASE.
          in_header = abap_false.
        ENDIF.
      ENDIF.
      APPEND line TO body.
    ENDLOOP.

    IF in_header = abap_true.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( 'The source has no "define table" statement' ).
    ENDIF.
    IF props-category IS NOT INITIAL AND props-category <> 'TRANSPARENT'.
      zcl_vsp_adt_txt_format=>raise_not_acceptable(
        |Only transparent tables are served here, not #{ props-category }| ).
    ENDIF.

    CREATE OBJECT logger.
    parser = cl_sbd_structure_parser=>create( i_logger = logger i_object_name = name ).
    parser->scan_source_table( body ).
    IF logger->has_error_messages( ) = abap_true.
      logger->get_error_message( EXPORTING  p_index        = 1
                                 IMPORTING  p_message_text = message
                                            p_line         = error_line
                                 EXCEPTIONS OTHERS         = 1 ).
      LOOP AT message INTO message_line.
        text = text && message_line && ` `.
      ENDLOOP.
      IF error_line > 0.
        text = |Line { error_line }: { text }|.
      ENDIF.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( text ).
    ENDIF.

    dd02v = parser->get_header( ).
    dd03p = parser->get_fields( ).
    dd08v = parser->get_foreign_keys( ).
    " As the structure editor saves: the dictionary expects every key
    " field of the check table, generic ones included.
    dd05m = cl_sbd_structure_persist=>adjust_foreign_key_fields(
      i_foreign_key_fields = parser->get_foreign_key_fields( )
      i_foreign_keys       = dd08v ).
    dd35v = parser->get_value_helps( ).
    dd36m = parser->get_value_help_fields( ).

    " What the structure parser does not know is kept from the table.
    old = read_header( ).
    dd02v-tabname = name.
    dd02v-tabclass = 'TRANSP'.
    dd02v-ddlanguage = sy-langu.
    dd02v-contflag = old-contflag.
    dd02v-mainflag = old-mainflag.
    IF props-delivery IS NOT INITIAL.
      dd02v-contflag = props-delivery.
    ENDIF.
    IF props-maintenance IS NOT INITIAL.
      dd02v-mainflag = from_annotation( value = props-maintenance kind = 'MAINFLAG' ).
    ENDIF.
    IF dd02v-ddtext IS INITIAL.
      dd02v-ddtext = old-ddtext.
    ENDIF.

    CALL FUNCTION 'DDIF_TABL_GET'
      EXPORTING
        name     = name
        state    = 'M'
      IMPORTING
        dd09l_wa = dd09v
      EXCEPTIONS
        OTHERS   = 1.
    IF dd09v IS INITIAL.
      dd09v-tabname = name.
      dd09v-tabkat = '0'.
      dd09v-tabart = 'APPL0'.
      dd09v-bufallow = 'N'.
    ENDIF.

    CALL FUNCTION 'DDIF_TABL_PUT'
      EXPORTING
        name              = name
        dd02v_wa          = dd02v
        dd09l_wa          = dd09v
      TABLES
        dd03p_tab         = dd03p
        dd05m_tab         = dd05m
        dd08v_tab         = dd08v
        dd35v_tab         = dd35v
        dd36m_tab         = dd36m
      EXCEPTIONS
        tabl_not_found    = 1
        name_inconsistent = 2
        tabl_inconsistent = 3
        put_failure       = 4
        put_refused       = 5
        OTHERS            = 6.
    IF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = 'TABL' resource_id = name ).
    ENDIF.
    CALL FUNCTION 'RS_INSERT_INTO_WORKING_AREA'
      EXPORTING
        object   = 'TABL'
        obj_name = CONV trobj_name( name )
      EXCEPTIONS
        OTHERS   = 1.
    COMMIT WORK.
  ENDMETHOD.


  METHOD to_annotation.
    CASE kind.
      WHEN 'MAINFLAG'.
        CASE value.
          WHEN 'X'.
            result = 'ALLOWED'.
          WHEN 'N'.
            result = 'NOT_ALLOWED'.
          WHEN OTHERS.
            result = 'RESTRICTED'.
        ENDCASE.
    ENDCASE.
  ENDMETHOD.


  METHOD from_annotation.
    CASE kind.
      WHEN 'MAINFLAG'.
        CASE value.
          WHEN 'ALLOWED'.
            result = 'X'.
          WHEN 'NOT_ALLOWED'.
            result = 'N'.
          WHEN OTHERS.
            result = space.
        ENDCASE.
    ENDCASE.
  ENDMETHOD.


  METHOD esc.
    result = escape( val = value format = cl_abap_format=>e_xml_attr ).
  ENDMETHOD.

ENDCLASS.
