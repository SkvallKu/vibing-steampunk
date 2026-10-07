"! <p class="shorttext synchronized">vsp ADT fallback: packages</p>
"! /sap/bc/adt/zvsp/packages[/{name}], the part of /sap/bc/adt/packages of
"! newer releases a client needs to find, create and delete a package:
"! <ul>
"! <li>GET .../{name}: the package document (pak:package), 404 if none;</li>
"! <li>POST .../packages?corrNr=..: creates the package the body
"!     describes and answers 201 with its document;</li>
"! <li>POST .../{name}?_action=LOCK|UNLOCK, and
"!     DELETE .../{name}?lockHandle=..&amp;corrNr=..: deletes it.</li>
"! </ul>
"! The work is done by CL_PACKAGE_FACTORY, as SE21 does it. The lock is
"! the one CL_PACKAGE takes (ENQUEUE_ESCOMPKENQ); the lock handle is a
"! hash over the enqueue entries holding the package name.
CLASS zcl_vsp_adt_pkg_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'application/vnd.sap.adt.packages.v1+xml'.

    METHODS get REDEFINITION.
    METHODS post REDEFINITION.
    METHODS delete REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    TYPES:
      BEGIN OF ty_request,
        name        TYPE devclass,
        description TYPE string,
        responsible TYPE string,
        parent      TYPE devclass,
        component   TYPE string,
        layer       TYPE string,
      END OF ty_request.

    METHODS name_from_uri
      IMPORTING request       TYPE REF TO if_adt_rest_request
      RETURNING VALUE(result) TYPE devclass
      RAISING   cx_adt_rest.

    METHODS load
      IMPORTING name          TYPE devclass
      RETURNING VALUE(result) TYPE REF TO if_package
      RAISING   cx_adt_rest.

    METHODS document
      IMPORTING name          TYPE devclass
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_rest.

    METHODS respond
      IMPORTING response TYPE REF TO if_adt_rest_response
                body     TYPE string.

    METHODS create
      IMPORTING request  TYPE REF TO if_adt_rest_request
                response TYPE REF TO if_adt_rest_response
      RAISING   cx_adt_rest.

    METHODS lock_handle
      IMPORTING name          TYPE devclass
      RETURNING VALUE(result) TYPE string.

    METHODS raise_failure
      IMPORTING name TYPE devclass
      RAISING   cx_adt_rest.

    CLASS-METHODS attr
      IMPORTING tag           TYPE string
                name          TYPE string
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS element
      IMPORTING xml           TYPE string
                name          TYPE string
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS esc
      IMPORTING value         TYPE csequence
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS uri
      IMPORTING name          TYPE csequence
      RETURNING VALUE(result) TYPE string.
ENDCLASS.



CLASS zcl_vsp_adt_pkg_resource IMPLEMENTATION.

  METHOD get.
    respond( response = response
             body     = document( name_from_uri( request ) ) ).
  ENDMETHOD.


  METHOD post.
    DATA action TYPE string.
    DATA name TYPE devclass.
    DATA package TYPE REF TO if_package.
    DATA lock_result TYPE sadt_object_lock_result.

    request->get_uri_query_parameter(
      EXPORTING name      = '_action'
                mandatory = abap_false
      IMPORTING value     = action ).
    TRANSLATE action TO UPPER CASE.

    IF action IS INITIAL.
      create( request = request response = response ).
      RETURN.
    ENDIF.

    name = name_from_uri( request ).
    package = load( name ).
    CASE action.
      WHEN 'LOCK'.
        package->set_changeable(
          EXPORTING  i_changeable      = abap_true
                     i_suppress_dialog = abap_true
          EXCEPTIONS object_already_changeable = 0
                     OTHERS                    = 1 ).
        IF sy-subrc <> 0.
          raise_failure( name ).
        ENDIF.
        lock_result-lock_handle = lock_handle( name ).
        lock_result-is_local = cl_adt_cts_management=>is_transport_disabled_for_obj(
          object   = 'DEVC'
          obj_name = CONV #( name ) ).
        zcl_vsp_adt_util=>respond_lock( response = response result = lock_result ).
      WHEN 'UNLOCK'.
        package->set_changeable(
          EXPORTING  i_changeable      = abap_false
                     i_suppress_dialog = abap_true
          EXCEPTIONS OTHERS            = 1 ).
      WHEN OTHERS.
        RAISE EXCEPTION TYPE cx_adt_res_meth_not_supported.
    ENDCASE.
  ENDMETHOD.


  METHOD delete.
    DATA name TYPE devclass.
    DATA handle TYPE string.
    DATA corrnr TYPE trkorr.
    DATA package TYPE REF TO if_package.

    name = name_from_uri( request ).
    request->get_uri_query_parameter(
      EXPORTING name      = 'lockHandle'
                mandatory = abap_true
      IMPORTING value     = handle ).
    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).

    package = load( name ).
    IF handle <> lock_handle( name ).
      RAISE EXCEPTION TYPE cx_adt_res_invalid_lock_handle
        EXPORTING
          resource_type = 'DEVC'
          resource_id   = CONV #( name )
          lock_handle   = handle.
    ENDIF.

    package->set_changeable(
      EXPORTING  i_changeable      = abap_true
                 i_suppress_dialog = abap_true
      EXCEPTIONS object_already_changeable = 0
                 OTHERS                    = 1 ).
    IF sy-subrc = 0.
      package->delete(
        EXPORTING  i_suppress_dialog = abap_true
        EXCEPTIONS OTHERS            = 1 ).
    ENDIF.
    IF sy-subrc = 0.
      package->save(
        EXPORTING  i_transport_request = corrnr
                   i_suppress_dialog   = abap_true
        EXCEPTIONS OTHERS              = 1 ).
    ENDIF.
    IF sy-subrc <> 0.
      raise_failure( name ).
    ENDIF.
    COMMIT WORK.
  ENDMETHOD.


  METHOD create.
    DATA xml TYPE string.
    DATA root TYPE string.
    DATA wanted TYPE ty_request.
    DATA pkg_data TYPE scompkdtln.
    DATA package TYPE REF TO if_package.
    DATA corrnr TYPE trkorr.
    DATA location TYPE string.

    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).

    xml = request->get_inner_rest_request( )->get_entity( )->get_string_data( ).
    root = element( xml = xml name = 'package' ).
    wanted-name = to_upper( attr( tag = root name = 'adtcore:name' ) ).
    wanted-description = attr( tag = root name = 'adtcore:description' ).
    wanted-responsible = to_upper( attr( tag = root name = 'adtcore:responsible' ) ).
    wanted-parent = to_upper( attr( tag = element( xml = xml name = 'superPackage' )
                                    name = 'adtcore:name' ) ).
    wanted-component = to_upper( attr( tag = element( xml = xml name = 'softwareComponent' )
                                       name = 'name' ) ).
    wanted-layer = to_upper( attr( tag = element( xml = xml name = 'transportLayer' )
                                   name = 'name' ) ).
    IF wanted-name IS INITIAL.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( 'The package has no adtcore:name' ).
    ENDIF.
    IF wanted-description IS INITIAL.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( 'The package has no adtcore:description' ).
    ENDIF.

    pkg_data-devclass = wanted-name.
    pkg_data-ctext = wanted-description.
    pkg_data-as4user = wanted-responsible.
    IF pkg_data-as4user IS INITIAL.
      pkg_data-as4user = sy-uname.
    ENDIF.
    pkg_data-parentcl = wanted-parent.
    pkg_data-pdevclass = wanted-layer.
    pkg_data-dlvunit = wanted-component.
    IF wanted-name(1) = '$'.
      pkg_data-dlvunit = 'LOCAL'.
      CLEAR pkg_data-pdevclass.
    ENDIF.
    pkg_data-language = sy-langu.
    pkg_data-masterlang = sy-langu.

    cl_package_factory=>create_new_package(
      EXPORTING  i_suppress_dialog = abap_true
      IMPORTING  e_package         = package
      CHANGING   c_package_data    = pkg_data
      EXCEPTIONS object_already_existing = 1
                 OTHERS                  = 2 ).
    IF sy-subrc = 1.
      zcl_vsp_adt_txt_format=>set_message( |Package { wanted-name } already exists| ).
      RAISE EXCEPTION TYPE cx_adt_res_already_exists
        EXPORTING
          textid = cx_adt_rest=>create_textid_from_msg_params( ).
    ELSEIF sy-subrc <> 0.
      raise_failure( wanted-name ).
    ENDIF.

    package->save(
      EXPORTING  i_transport_request = corrnr
                 i_suppress_dialog   = abap_true
      EXCEPTIONS OTHERS              = 1 ).
    IF sy-subrc <> 0.
      raise_failure( wanted-name ).
    ENDIF.
    package->set_changeable(
      EXPORTING  i_changeable      = abap_false
                 i_suppress_dialog = abap_true
      EXCEPTIONS OTHERS            = 1 ).
    COMMIT WORK.

    location = uri( wanted-name ).
    response->set_status( cl_rest_status_code=>gc_success_created ).
    response->set_location( location ).
    respond( response = response
             body     = document( wanted-name ) ).
  ENDMETHOD.


  METHOD name_from_uri.
    DATA name TYPE string.

    request->get_uri_attribute(
      EXPORTING name      = 'name'
                mandatory = abap_true
      IMPORTING value     = name ).
    name = cl_http_utility=>unescape_url( name ).
    IF strlen( name ) > 30.
      RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDIF.
    result = to_upper( name ).
  ENDMETHOD.


  METHOD load.
    cl_package_factory=>load_package(
      EXPORTING  i_package_name = name
                 i_force_reload = abap_true
      IMPORTING  e_package      = result
      EXCEPTIONS object_not_existing = 1
                 OTHERS              = 2 ).
    IF sy-subrc = 1.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'DEVC'
          resource_id   = CONV #( name ).
    ELSEIF sy-subrc <> 0.
      raise_failure( name ).
    ENDIF.
  ENDMETHOD.


  METHOD document.
    DATA tdevc TYPE tdevc.
    DATA text TYPE as4text.
    DATA parent_text TYPE as4text.
    DATA masterlang TYPE masterlang.
    DATA language TYPE laiso.
    DATA master_iso TYPE laiso.
    DATA package_type TYPE string.
    DATA super_ref TYPE string.
    DATA subs TYPE string.
    DATA sub TYPE devclass.
    DATA subnames TYPE STANDARD TABLE OF devclass WITH DEFAULT KEY.

    SELECT SINGLE * FROM tdevc INTO tdevc WHERE devclass = name.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'DEVC'
          resource_id   = CONV #( name ).
    ENDIF.

    SELECT SINGLE ctext FROM tdevct INTO text
      WHERE devclass = name AND spras = sy-langu.
    IF sy-subrc <> 0.
      SELECT SINGLE ctext FROM tdevct INTO text WHERE devclass = name.
    ENDIF.
    SELECT SINGLE masterlang FROM tadir INTO masterlang
      WHERE pgmid = 'R3TR' AND object = 'DEVC' AND obj_name = name.
    IF masterlang IS INITIAL.
      masterlang = sy-langu.
    ENDIF.
    CALL FUNCTION 'CONVERSION_EXIT_ISOLA_OUTPUT'
      EXPORTING
        input  = masterlang
      IMPORTING
        output = master_iso.
    CALL FUNCTION 'CONVERSION_EXIT_ISOLA_OUTPUT'
      EXPORTING
        input  = sy-langu
      IMPORTING
        output = language.

    IF tdevc-mainpack = 'X'.
      package_type = 'main'.
    ELSEIF tdevc-mainpack = 'S'.
      package_type = 'structure'.
    ELSE.
      package_type = 'development'.
    ENDIF.

    IF tdevc-parentcl IS NOT INITIAL.
      SELECT SINGLE ctext FROM tdevct INTO parent_text
        WHERE devclass = tdevc-parentcl AND spras = sy-langu.
      super_ref = |<pak:superPackage adtcore:uri="{ uri( tdevc-parentcl ) }" | &&
              |adtcore:type="DEVC/K" adtcore:name="{ esc( tdevc-parentcl ) }" | &&
              |adtcore:description="{ esc( parent_text ) }"/>|.
    ELSE.
      super_ref = '<pak:superPackage/>'.
    ENDIF.

    SELECT devclass FROM tdevc INTO TABLE subnames
      WHERE parentcl = name ORDER BY devclass.
    LOOP AT subnames INTO sub.
      subs = |{ subs }<pak:packageRef adtcore:uri="{ uri( sub ) }" | &&
             |adtcore:type="DEVC/K" adtcore:name="{ esc( sub ) }"/>|.
    ENDLOOP.

    result =
      |<?xml version="1.0" encoding="utf-8"?>| &&
      |<pak:package adtcore:responsible="{ esc( tdevc-as4user ) }" | &&
      |adtcore:masterLanguage="{ master_iso }" adtcore:name="{ esc( tdevc-devclass ) }" | &&
      |adtcore:type="DEVC/K" adtcore:version="active" | &&
      |adtcore:createdBy="{ esc( tdevc-created_by ) }" | &&
      |adtcore:changedBy="{ esc( tdevc-changed_by ) }" | &&
      |adtcore:description="{ esc( text ) }" adtcore:descriptionTextLimit="60" | &&
      |adtcore:language="{ language }" | &&
      |xmlns:pak="http://www.sap.com/adt/packages" | &&
      |xmlns:adtcore="http://www.sap.com/adt/core">| &&
      |<adtcore:packageRef adtcore:uri="{ uri( tdevc-devclass ) }" | &&
      |adtcore:type="DEVC/K" adtcore:name="{ esc( tdevc-devclass ) }" | &&
      |adtcore:description="{ esc( text ) }"/>| &&
      |<pak:attributes pak:packageType="{ package_type }"/>| &&
      super_ref &&
      |<pak:applicationComponent pak:name="{ esc( tdevc-component ) }"/>| &&
      |<pak:transport>| &&
      |<pak:softwareComponent pak:name="{ esc( tdevc-dlvunit ) }"/>| &&
      |<pak:transportLayer pak:name="{ esc( tdevc-pdevclass ) }"/>| &&
      |</pak:transport>| &&
      |<pak:useAccesses/><pak:packageInterfaces/>| &&
      |<pak:subPackages>{ subs }</pak:subPackages>| &&
      |</pak:package>|.
  ENDMETHOD.


  METHOD respond.
    DATA entity TYPE REF TO if_rest_entity.

    entity = response->get_inner_rest_response( )->create_entity( ).
    entity->set_content_type( iv_media_type = co_content_type ).
    entity->set_string_data( body ).
  ENDMETHOD.


  METHOD lock_handle.
    DATA entries TYPE STANDARD TABLE OF seqg3.
    DATA entry TYPE seqg3.
    DATA text TYPE string.

    CALL FUNCTION 'ENQUEUE_READ'
      EXPORTING
        gname                 = space
        guname                = sy-uname
      TABLES
        enq                   = entries
      EXCEPTIONS
        OTHERS                = 1.
    IF sy-subrc <> 0.
      RETURN.
    ENDIF.
    SORT entries BY gname garg.
    LOOP AT entries INTO entry WHERE garg CS name.
      text = |{ text }{ entry-gname }{ entry-garg }{ entry-gtdate }{ entry-gttime }| &&
             |{ entry-gtusec }{ entry-gtwp }{ entry-gtsysnr }{ entry-gthost }|.
    ENDLOOP.
    IF text IS NOT INITIAL.
      result = cl_adt_lock_hasher=>get( text ).
    ENDIF.
  ENDMETHOD.


  METHOD raise_failure.
    DATA text TYPE string.

    IF sy-msgid IS NOT INITIAL.
      MESSAGE ID sy-msgid TYPE 'S' NUMBER sy-msgno
        WITH sy-msgv1 sy-msgv2 sy-msgv3 sy-msgv4 INTO text.
    ELSE.
      text = |Package { name }: failed with sy-subrc { sy-subrc }|.
    ENDIF.
    zcl_vsp_adt_txt_format=>set_message( text ).
    RAISE EXCEPTION TYPE cx_adt_res_no_access
      EXPORTING
        textid        = cx_adt_rest=>create_textid_from_msg_params( )
        resource_type = 'DEVC'
        resource_id   = CONV #( name ).
  ENDMETHOD.


  METHOD attr.
    DATA pattern TYPE string.

    pattern = |{ name }="([^"]*)"|.
    FIND REGEX pattern IN tag SUBMATCHES result.
    REPLACE ALL OCCURRENCES OF '&quot;' IN result WITH '"'.
    REPLACE ALL OCCURRENCES OF '&apos;' IN result WITH ''''.
    REPLACE ALL OCCURRENCES OF '&lt;' IN result WITH '<'.
    REPLACE ALL OCCURRENCES OF '&gt;' IN result WITH '>'.
    REPLACE ALL OCCURRENCES OF '&amp;' IN result WITH '&'.
  ENDMETHOD.


  METHOD element.
    DATA pattern TYPE string.
    DATA offset TYPE i.
    DATA length TYPE i.

    " The opening tag of <prefix:name ...>, whatever the prefix.
    pattern = |<[A-Za-z0-9_]+:{ name }(\\s[^>]*)?/?>|.
    FIND REGEX pattern IN xml MATCH OFFSET offset MATCH LENGTH length.
    IF sy-subrc = 0.
      result = xml+offset(length).
    ENDIF.
  ENDMETHOD.


  METHOD esc.
    result = escape( val = value format = cl_abap_format=>e_xml_attr ).
  ENDMETHOD.


  METHOD uri.
    result = zif_vsp_adt_feature=>co_root && '/packages/' &&
             cl_http_utility=>escape_url( to_lower( condense( name ) ) ).
  ENDMETHOD.

ENDCLASS.
