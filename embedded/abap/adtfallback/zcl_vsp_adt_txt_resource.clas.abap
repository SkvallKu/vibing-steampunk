"! <p class="shorttext synchronized">vsp ADT fallback: text elements</p>
"! /sap/bc/adt/zvsp/textelements/{programs|classes}/{name}[/source/{kind}],
"! the protocol of the standard /sap/bc/adt/textelements of newer releases:
"! <ul>
"! <li>GET .../source/{symbols|selections|headings}: the document, with an
"!     etag, and 304 for a matching If-None-Match;</li>
"! <li>POST ...?_action=LOCK|UNLOCK on the object;</li>
"! <li>PUT .../source/{kind}?lockHandle=..&amp;corrNr=..: writes the
"!     inactive version and answers with the document as it now is.</li>
"! </ul>
"! The object itself (its properties document) is not served. Texts are
"! read and written in the object's master language, as by the standard.
CLASS zcl_vsp_adt_txt_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    METHODS get REDEFINITION.
    METHODS post REDEFINITION.
    METHODS put REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    DATA object_type TYPE trobjtype.
    DATA object_name TYPE string.
    DATA rept_name TYPE syrepid.
    DATA kind TYPE string.
    DATA pool TYPE REF TO zcl_vsp_adt_txt_pool.
    DATA access TYPE REF TO zcl_vsp_adt_txt_lock.

    METHODS parse_uri
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    METHODS require_kind
      RAISING cx_adt_rest.

    METHODS respond
      IMPORTING request     TYPE REF TO if_adt_rest_request
                response    TYPE REF TO if_adt_rest_response
                client_etag TYPE string OPTIONAL
      RAISING   cx_adt_rest.
ENDCLASS.



CLASS zcl_vsp_adt_txt_resource IMPLEMENTATION.

  METHOD get.
    DATA version_param TYPE if_adt_tools_core_types=>ty_object-version.
    DATA requested TYPE r3state.

    parse_uri( request ).
    require_kind( ).
    access->check_show( ).

    request->get_uri_query_parameter(
      EXPORTING name      = 'version'
                mandatory = abap_false
      IMPORTING value     = version_param ).
    IF version_param IS NOT INITIAL.
      TRANSLATE version_param TO LOWER CASE.
      requested = cl_adt_utility=>get_wb_version( version_param ).
    ENDIF.

    pool->read( pool->resolve_version( requested ) ).
    respond( request     = request
             response    = response
             client_etag = request->get_if_none_match( ) ).
  ENDMETHOD.


  METHOD post.
    DATA action TYPE string.
    DATA lock_result TYPE sadt_object_lock_result.

    parse_uri( request ).
    request->get_uri_query_parameter(
      EXPORTING name      = '_action'
                mandatory = abap_true
      IMPORTING value     = action ).
    TRANSLATE action TO UPPER CASE.

    CASE action.
      WHEN 'LOCK'.
        lock_result = access->lock( ).
        zcl_vsp_adt_util=>respond_lock( response = response result = lock_result ).
      WHEN 'UNLOCK'.
        access->unlock( ).
      WHEN OTHERS.
        RAISE EXCEPTION TYPE cx_adt_res_meth_not_supported.
    ENDCASE.
  ENDMETHOD.


  METHOD put.
    DATA lock_handle TYPE string.
    DATA corrnr TYPE trkorr.
    DATA document TYPE zcl_vsp_adt_txt_format=>ty_document.
    DATA format TYPE REF TO zcl_vsp_adt_txt_format.

    parse_uri( request ).
    require_kind( ).

    request->get_uri_query_parameter(
      EXPORTING name      = 'lockHandle'
                mandatory = abap_true
      IMPORTING value     = lock_handle ).
    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).
    access->check_handle( lock_handle ).

    CREATE OBJECT format EXPORTING kind = kind.
    request->get_body_data(
      EXPORTING content_handler = cl_adt_rest_comp_cnt_handler=>create(
                  request         = request
                  content_handler = format )
      IMPORTING data            = document ).

    pool->record_in_transport( CHANGING corrnr = corrnr ).
    pool->save( kind = kind document = document ).
    respond( request = request response = response ).
  ENDMETHOD.


  METHOD parse_uri.
    DATA uri TYPE string.
    DATA collection TYPE string.
    DATA name TYPE string.
    DATA class_name TYPE seoclsname.

    uri = request->get_inner_rest_request( )->get_uri( ).
    FIND REGEX '/textelements/(programs|classes)/' IN uri IGNORING CASE
      SUBMATCHES collection.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDIF.
    TRANSLATE collection TO LOWER CASE.

    request->get_uri_attribute(
      EXPORTING name      = 'name'
                mandatory = abap_true
      IMPORTING value     = name ).
    request->get_uri_attribute(
      EXPORTING name      = 'sourcename'
                mandatory = abap_false
      IMPORTING value     = kind ).
    TRANSLATE kind TO LOWER CASE.

    name = cl_http_utility=>unescape_url( name ).
    TRANSLATE name TO UPPER CASE.
    object_name = name.

    IF collection = 'classes'.
      IF strlen( name ) > 30.
        RAISE EXCEPTION TYPE cx_adt_res_not_found.
      ENDIF.
      object_type = 'CLAS'.
      class_name = name.
      rept_name = cl_oo_classname_service=>get_classpool_name( class_name ).
    ELSE.
      IF strlen( name ) > 40.
        RAISE EXCEPTION TYPE cx_adt_res_not_found.
      ENDIF.
      object_type = 'PROG'.
      rept_name = name.
    ENDIF.

    CREATE OBJECT pool
      EXPORTING
        object_type = object_type
        object_name = object_name
        rept_name   = rept_name.
    IF pool->exists( ) = abap_false.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'REPT'
          resource_id   = CONV #( rept_name ).
    ENDIF.

    CREATE OBJECT access EXPORTING rept_name = rept_name.
  ENDMETHOD.


  METHOD require_kind.
    IF zcl_vsp_adt_txt_format=>is_kind( kind ) = abap_false.
      zcl_vsp_adt_txt_format=>raise_not_acceptable(
        'Only .../source/symbols, selections or headings is served here' ).
    ENDIF.
    IF object_type = 'CLAS' AND kind <> zcl_vsp_adt_txt_format=>co_kind-symbols.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'REPT'
          resource_id   = CONV #( rept_name ).
    ENDIF.
  ENDMETHOD.


  METHOD respond.
    DATA etag TYPE string.
    DATA format TYPE REF TO zcl_vsp_adt_txt_format.

    etag = pool->get_etag( kind ).
    IF client_etag IS NOT INITIAL AND client_etag = etag.
      response->set_status( cl_rest_status_code=>gc_redirection_not_modified ).
      RETURN.
    ENDIF.

    CREATE OBJECT format EXPORTING kind = kind.
    response->set_etag( etag ).
    response->set_body_data(
      content_handler = cl_adt_rest_comp_cnt_handler=>create(
        request         = request
        content_handler = format )
      data            = pool->get_document( kind ) ).
  ENDMETHOD.

ENDCLASS.
