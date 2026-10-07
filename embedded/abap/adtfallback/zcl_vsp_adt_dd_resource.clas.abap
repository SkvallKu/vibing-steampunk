"! <p class="shorttext synchronized">vsp ADT fallback: DDIC object resource</p>
"! What the form-based dictionary objects share — domains, data elements
"! and table types, whose ADT resource is one XML document per object, not
"! a source:
"! <ul>
"! <li>POST {collection}?corrNr=..: creates the object from a document
"!     that names it, its description and package, as the standard's
"!     create does; whatever else the document holds is taken too;</li>
"! <li>POST .../{name}?_action=LOCK|UNLOCK; DELETE .../{name};</li>
"! <li>GET .../{name}[?version=active]: the document;</li>
"! <li>PUT .../{name}?lockHandle=..[&corrNr=..]: the document, stored as
"!     the inactive version for /sap/bc/adt/activation.</li>
"! </ul>
"! The document is the standard's, element for element, so vsp speaks
"! the same to both. A write takes the elements the document has and keeps
"! the rest of the object. Texts are the master language's, read and
"! written, as the standard's (7.50: a German original is served in German
"! to a Russian logon); a new object's master language is the logon's.
"! A subclass says which object it is and how its document maps to the
"! dictionary.
CLASS zcl_vsp_adt_dd_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  ABSTRACT
  CREATE PUBLIC.

  PUBLIC SECTION.
    CONSTANTS co_adtcore TYPE string VALUE 'http://www.sap.com/adt/core'.

    METHODS get REDEFINITION.
    METHODS post REDEFINITION.
    METHODS put REDEFINITION.
    METHODS delete REDEFINITION.

    "! The workbench object behind a URI under a collection of ours, for
    "! the features' ZIF_VSP_ADT_FEATURE~MAP_TO_WB_OBJECT.
    CLASS-METHODS to_wb_object
      IMPORTING uri           TYPE csequence
                collection    TYPE csequence
                objtype       TYPE trobjtype
                subtype       TYPE csequence
      RETURNING VALUE(result) TYPE REF TO cl_wb_object
      RAISING   cx_adt_uri_mapping.

  PROTECTED SECTION.
    DATA name TYPE ddobjname.

    "! DOMA, DTEL or TTYP.
    METHODS object_type ABSTRACT
      RETURNING VALUE(result) TYPE trobjtype.

    "! The ADT type: DOMA/DD.
    METHODS adt_type ABSTRACT
      RETURNING VALUE(result) TYPE string.

    "! The collection under the root: /ddic/domains.
    METHODS collection ABSTRACT
      RETURNING VALUE(result) TYPE string.

    METHODS content_type ABSTRACT
      RETURNING VALUE(result) TYPE string.

    "! Whether the dictionary has any version of the object.
    METHODS exists ABSTRACT
      RETURNING VALUE(result) TYPE abap_bool.

    "! The document: state A for the active version, M for the newest.
    METHODS render ABSTRACT
      IMPORTING state         TYPE ddobjstate
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_rest.

    "! Stores the document's content as the inactive version: what the
    "! root element holds, over the object as it is (nothing, if new).
    "! No COMMIT: the caller commits.
    METHODS write ABSTRACT
      IMPORTING root   TYPE REF TO if_ixml_element
                is_new TYPE abap_bool
      RAISING   cx_adt_rest.

    "! The adtcore attributes of the root element and the package
    "! reference after it, for a version with this header data.
    METHODS object_header
      IMPORTING description  TYPE csequence
                gotstate     TYPE csequence
                changed_by   TYPE csequence
                changed_on   TYPE d
                changed_at   TYPE t
      EXPORTING attributes   TYPE string
                package_ref  TYPE string.

    "! The master language: the dictionary's (DD01L-DOMMASTER,
    "! DD04L-DTELMASTER) when it has one — older objects have none — else
    "! the object directory's.
    METHODS master_language
      IMPORTING dictionary    TYPE sylangu
      RETURNING VALUE(result) TYPE sylangu.

    "! The description on the root element, and whether it is there.
    METHODS description_of
      IMPORTING root        TYPE REF TO if_ixml_element
      EXPORTING description TYPE string
                found       TYPE abap_bool.

    CLASS-METHODS el
      IMPORTING tag           TYPE csequence
                value         TYPE simple
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS bool
      IMPORTING flag          TYPE csequence
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS esc
      IMPORTING value         TYPE csequence
      RETURNING VALUE(result) TYPE string.

    "! The first element of that name below the parent, at any depth.
    CLASS-METHODS child
      IMPORTING parent        TYPE REF TO if_ixml_element
                namespace     TYPE string
                tag           TYPE string
      RETURNING VALUE(result) TYPE REF TO if_ixml_element.

    "! The text of CHILD( ); FOUND tells an empty element from none.
    CLASS-METHODS child_text
      IMPORTING parent    TYPE REF TO if_ixml_element
                namespace TYPE string
                tag       TYPE string
      EXPORTING value     TYPE string
                found     TYPE abap_bool.

    "! 'X' for true, space for anything else.
    CLASS-METHODS flag_of
      IMPORTING value         TYPE csequence
      RETURNING VALUE(result) TYPE flag.

    "! A number of the document as an integer; not a number → not acceptable.
    CLASS-METHODS int_of
      IMPORTING value         TYPE csequence
                tag           TYPE csequence
      RETURNING VALUE(result) TYPE i
      RAISING   cx_adt_rest.

    "! A component that not every release has, by name: empty if absent.
    CLASS-METHODS get_component
      IMPORTING structure     TYPE any
                component     TYPE csequence
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS set_component
      IMPORTING component TYPE csequence
                value     TYPE csequence
      CHANGING  structure TYPE any.

  PRIVATE SECTION.
    METHODS parse_uri
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    METHODS parse_document
      IMPORTING request       TYPE REF TO if_adt_rest_request
      RETURNING VALUE(result) TYPE REF TO if_ixml_element
      RAISING   cx_adt_rest.

    METHODS require_object
      RAISING cx_adt_rest.

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
      IMPORTING request  TYPE REF TO if_adt_rest_request
                devclass TYPE devclass OPTIONAL
                mode     TYPE csequence DEFAULT 'MODIFY'
      RAISING   cx_adt_rest.

    METHODS to_working_area.

    METHODS location
      RETURNING VALUE(result) TYPE string.
ENDCLASS.



CLASS zcl_vsp_adt_dd_resource IMPLEMENTATION.

  METHOD get.
    DATA version TYPE string.
    DATA state TYPE ddobjstate VALUE 'M'.

    parse_uri( request ).
    request->get_uri_query_parameter(
      EXPORTING name      = 'version'
                mandatory = abap_false
      IMPORTING value     = version ).
    IF to_lower( version ) = 'active'.
      state = 'A'.
    ENDIF.
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = content_type( )
                               body         = render( state ) ).
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
    CASE action.
      WHEN 'LOCK'.
        require_object( ).
        lock( request = request response = response ).
      WHEN 'UNLOCK'.
        " Also after a DELETE: the object is gone, the lock is not.
        unlock( ).
      WHEN OTHERS.
        RAISE EXCEPTION TYPE cx_adt_res_meth_not_supported.
    ENDCASE.
  ENDMETHOD.


  METHOD put.
    DATA root TYPE REF TO if_ixml_element.

    parse_uri( request ).
    require_object( ).
    check_handle( request ).
    root = parse_document( request ).
    corr_insert( request ).
    write( root = root is_new = abap_false ).
    to_working_area( ).
    COMMIT WORK.
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = content_type( )
                               body         = render( 'M' ) ).
  ENDMETHOD.


  METHOD delete.
    DATA corrnr TYPE trkorr.
    DATA objname TYPE rsedd0-ddobjname.
    DATA objtype TYPE c LENGTH 1.
    DATA sap_text TYPE string.

    parse_uri( request ).
    require_object( ).
    check_handle( request ).
    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).

    CASE object_type( ).
      WHEN 'DOMA'.
        objtype = 'D'.
      WHEN 'DTEL'.
        objtype = 'E'.
      WHEN 'TTYP'.
        objtype = 'A'.
    ENDCASE.
    " RS_DD_DELETE_OBJ takes the lock itself and would find ours in its way
    " ("already being edited"): the handle has been checked, hand it over.
    unlock( ).
    objname = name.
    CALL FUNCTION 'RS_DD_DELETE_OBJ'
      EXPORTING
        no_ask               = abap_true
        objname              = objname
        objtype              = objtype
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
      " The dictionary would ask first: the object is used. Nothing is
      " deleted without that question being answered in SE11.
      MESSAGE ID sy-msgid TYPE 'S' NUMBER sy-msgno
        WITH sy-msgv1 sy-msgv2 sy-msgv3 sy-msgv4 INTO sap_text.
      zcl_vsp_adt_util=>raise_failure(
        resource_type = object_type( ) resource_id = name
        text = |{ name } was not deleted: it is in use, and the dictionary | &&
               |asks before deleting it (SE11). { sap_text }| ).
    ELSEIF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = object_type( ) resource_id = name ).
    ENDIF.
    COMMIT WORK.
  ENDMETHOD.


  METHOD to_wb_object.
    DATA path TYPE string.
    DATA rest TYPE string.
    DATA key TYPE string.
    DATA object_type TYPE wbobjtype.
    DATA object_key TYPE seu_objkey.

    path = uri.
    SPLIT path AT '?' INTO path rest.
    SPLIT path AT '#' INTO path rest.
    rest = |^{ zif_vsp_adt_feature=>co_root }{ collection }/([^/]+)$|.
    FIND REGEX rest IN path IGNORING CASE SUBMATCHES key.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.
    key = to_upper( cl_http_utility=>unescape_url( key ) ).

    object_type-objtype_tr = objtype.
    object_type-subtype_wb = subtype.
    object_key = key.
    cl_wb_object=>create_from_global_type(
      EXPORTING
        p_object_type = object_type
        p_object_key  = object_key
      RECEIVING
        p_wb_object   = result
      EXCEPTIONS
        OTHERS        = 1 ).
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.
  ENDMETHOD.


  METHOD object_header.
    DATA devclass TYPE devclass.
    DATA masterlang TYPE masterlang.
    DATA author TYPE responsibl.
    DATA srcsystem TYPE srcsystem.
    DATA iso TYPE laiso.
    DATA master_iso TYPE laiso.
    DATA version TYPE string VALUE 'inactive'.
    DATA changed TYPE string.
    DATA objtype TYPE trobjtype.
    DATA zone TYPE timezone.
    DATA stamp TYPE timestamp.
    DATA utc_date TYPE d.
    DATA utc_time TYPE t.

    objtype = object_type( ).
    SELECT SINGLE devclass masterlang author srcsystem FROM tadir
      INTO (devclass, masterlang, author, srcsystem)
      WHERE pgmid = 'R3TR' AND object = objtype AND obj_name = name.
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
    IF gotstate = 'A'.
      version = 'active'.
    ENDIF.
    IF changed_on IS NOT INITIAL.
      " The dictionary keeps system time; the document says UTC.
      CALL FUNCTION 'GET_SYSTEM_TIMEZONE'
        IMPORTING
          timezone = zone
        EXCEPTIONS
          OTHERS   = 1.
      CONVERT DATE changed_on TIME changed_at INTO TIME STAMP stamp TIME ZONE zone.
      CONVERT TIME STAMP stamp TIME ZONE 'UTC' INTO DATE utc_date TIME utc_time.
      changed = |adtcore:changedAt="{ utc_date(4) }-{ utc_date+4(2) }-{ utc_date+6(2) }T| &&
                |{ utc_time(2) }:{ utc_time+2(2) }:{ utc_time+4(2) }Z" |.
    ENDIF.

    attributes =
      |adtcore:responsible="{ esc( author ) }" adtcore:masterLanguage="{ master_iso }" | &&
      |adtcore:masterSystem="{ esc( srcsystem ) }" adtcore:name="{ esc( name ) }" | &&
      |adtcore:type="{ adt_type( ) }" { changed }adtcore:version="{ version }" | &&
      |adtcore:changedBy="{ esc( changed_by ) }" adtcore:createdBy="{ esc( author ) }" | &&
      |adtcore:description="{ esc( description ) }" adtcore:language="{ iso }"|.
    package_ref =
      |<adtcore:packageRef adtcore:uri="/sap/bc/adt/packages/| &&
      |{ cl_http_utility=>escape_url( to_lower( devclass ) ) }" | &&
      |adtcore:type="DEVC/K" adtcore:name="{ esc( devclass ) }"/>|.
  ENDMETHOD.


  METHOD master_language.
    DATA objtype TYPE trobjtype.

    result = dictionary.
    IF result IS INITIAL.
      objtype = object_type( ).
      SELECT SINGLE masterlang FROM tadir INTO result
        WHERE pgmid = 'R3TR' AND object = objtype AND obj_name = name.
    ENDIF.
  ENDMETHOD.


  METHOD description_of.
    DATA attribute TYPE REF TO if_ixml_attribute.

    CLEAR: description, found.
    attribute = root->get_attribute_node_ns( name = 'description' uri = co_adtcore ).
    IF attribute IS BOUND.
      description = attribute->get_value( ).
      found = abap_true.
    ENDIF.
  ENDMETHOD.


  METHOD el.
    DATA text TYPE string.

    text = value.
    IF text IS INITIAL.
      result = |<{ tag }/>|.
    ELSE.
      result = |<{ tag }>{ escape( val = text format = cl_abap_format=>e_xml_text ) }</{ tag }>|.
    ENDIF.
  ENDMETHOD.


  METHOD bool.
    IF flag IS INITIAL.
      result = 'false'.
    ELSE.
      result = 'true'.
    ENDIF.
  ENDMETHOD.


  METHOD esc.
    result = escape( val = value format = cl_abap_format=>e_xml_attr ).
  ENDMETHOD.


  METHOD child.
    result = parent->find_from_name_ns( name = tag uri = namespace ).
  ENDMETHOD.


  METHOD child_text.
    DATA element TYPE REF TO if_ixml_element.

    CLEAR: value, found.
    element = child( parent = parent namespace = namespace tag = tag ).
    IF element IS BOUND.
      value = element->get_value( ).
      found = abap_true.
    ENDIF.
  ENDMETHOD.


  METHOD flag_of.
    IF to_lower( condense( value ) ) = 'true'.
      result = abap_true.
    ENDIF.
  ENDMETHOD.


  METHOD int_of.
    DATA text TYPE string.

    text = condense( value ).
    IF text IS INITIAL.
      RETURN.
    ENDIF.
    IF text CN '0123456789'.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( |{ tag } is not a number: "{ text }"| ).
    ENDIF.
    result = text.
  ENDMETHOD.


  METHOD get_component.
    FIELD-SYMBOLS <value> TYPE any.

    ASSIGN COMPONENT component OF STRUCTURE structure TO <value>.
    IF sy-subrc = 0.
      result = <value>.
    ENDIF.
  ENDMETHOD.


  METHOD set_component.
    FIELD-SYMBOLS <value> TYPE any.

    ASSIGN COMPONENT component OF STRUCTURE structure TO <value>.
    IF sy-subrc = 0.
      <value> = value.
    ENDIF.
  ENDMETHOD.


  METHOD parse_uri.
    DATA value TYPE string.

    request->get_uri_attribute(
      EXPORTING name      = 'name'
                mandatory = abap_true
      IMPORTING value     = value ).
    value = to_upper( cl_http_utility=>unescape_url( value ) ).
    IF strlen( value ) > 30.
      RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDIF.
    name = value.
  ENDMETHOD.


  METHOD parse_document.
    DATA ixml TYPE REF TO if_ixml.
    DATA factory TYPE REF TO if_ixml_stream_factory.
    DATA stream TYPE REF TO if_ixml_istream.
    DATA parser TYPE REF TO if_ixml_parser.
    DATA document TYPE REF TO if_ixml_document.
    DATA xml TYPE xstring.

    xml = request->get_inner_rest_request( )->get_entity( )->get_binary_data( ).
    ixml = cl_ixml=>create( ).
    factory = ixml->create_stream_factory( ).
    stream = factory->create_istream_xstring( xml ).
    document = ixml->create_document( ).
    parser = ixml->create_parser( stream_factory = factory
                                  istream        = stream
                                  document       = document ).
    IF parser->parse( ) <> 0.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( 'The body is not a well-formed XML document' ).
    ENDIF.
    result = document->get_root_element( ).
    IF result IS NOT BOUND.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( 'The body has no root element' ).
    ENDIF.
  ENDMETHOD.


  METHOD require_object.
    IF exists( ) = abap_false.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = CONV #( object_type( ) )
          resource_id   = CONV #( name ).
    ENDIF.
  ENDMETHOD.


  METHOD create.
    DATA root TYPE REF TO if_ixml_element.
    DATA package TYPE REF TO if_ixml_element.
    DATA value TYPE string.
    DATA devclass TYPE devclass.

    root = parse_document( request ).
    value = to_upper( root->get_attribute_ns( name = 'name' uri = co_adtcore ) ).
    IF value IS INITIAL OR strlen( value ) > 30.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( |The { object_type( ) } has no valid adtcore:name| ).
    ENDIF.
    name = value.
    package = child( parent = root namespace = co_adtcore tag = 'packageRef' ).
    IF package IS BOUND.
      devclass = to_upper( package->get_attribute_ns( name = 'name' uri = co_adtcore ) ).
    ENDIF.
    IF devclass IS INITIAL.
      zcl_vsp_adt_txt_format=>raise_not_acceptable(
        |The { object_type( ) } names no package (adtcore:packageRef)| ).
    ENDIF.

    IF exists( ) = abap_true.
      zcl_vsp_adt_txt_format=>set_message( |{ name } already exists in the dictionary| ).
      RAISE EXCEPTION TYPE cx_adt_res_already_exists
        EXPORTING
          textid = cx_adt_rest=>create_textid_from_msg_params( ).
    ENDIF.

    corr_insert( request = request devclass = devclass mode = 'INSERT' ).
    write( root = root is_new = abap_true ).
    to_working_area( ).
    COMMIT WORK.

    response->set_status( cl_rest_status_code=>gc_success_created ).
    response->set_location( location( ) ).
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = content_type( )
                               body         = render( 'M' ) ).
  ENDMETHOD.


  METHOD lock.
    DATA result TYPE sadt_object_lock_result.
    DATA transport TYPE sadt_object_lock_result.
    DATA object TYPE c LENGTH 34.

    object = |{ object_type( ) }{ name }|.
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
      zcl_vsp_adt_util=>raise_failure( resource_type = object_type( ) resource_id = name ).
    ENDIF.

    result-lock_handle = zcl_vsp_adt_util=>lock_handle( name ).
    IF result-lock_handle IS INITIAL.
      unlock( ).
      zcl_vsp_adt_util=>raise_failure( resource_type = object_type( ) resource_id = name
                                       text = |No lock entry for { name } after locking it| ).
    ENDIF.
    transport = cl_adt_cts_management=>get_transport_info(
      pgmid    = 'R3TR'
      obj_type = object_type( )
      obj_name = CONV #( name ) ).
    result-corrnr = transport-corrnr.
    result-corruser = transport-corruser.
    result-corrtext = transport-corrtext.
    result-is_link_up = transport-is_link_up.
    result-is_local = cl_adt_cts_management=>is_transport_disabled_for_obj(
      object   = object_type( )
      obj_name = CONV #( name ) ).
    zcl_vsp_adt_util=>respond_lock( response = response result = result ).
  ENDMETHOD.


  METHOD unlock.
    DATA object TYPE c LENGTH 34.

    object = |{ object_type( ) }{ name }|.
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
          resource_type = CONV #( object_type( ) )
          resource_id   = CONV #( name )
          lock_handle   = handle.
    ENDIF.
  ENDMETHOD.


  METHOD corr_insert.
    DATA corrnr TYPE trkorr.
    DATA package TYPE devclass.
    DATA object TYPE c LENGTH 34.
    DATA objtype TYPE trobjtype.

    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).
    package = devclass.
    IF package IS INITIAL.
      objtype = object_type( ).
      SELECT SINGLE devclass FROM tadir INTO package
        WHERE pgmid = 'R3TR' AND object = objtype AND obj_name = name.
    ENDIF.

    object = |{ object_type( ) }{ name }|.
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
      zcl_vsp_adt_util=>raise_failure( resource_type = object_type( ) resource_id = name ).
    ENDIF.
  ENDMETHOD.


  METHOD to_working_area.
    CALL FUNCTION 'RS_INSERT_INTO_WORKING_AREA'
      EXPORTING
        object   = object_type( )
        obj_name = CONV trobj_name( name )
      EXCEPTIONS
        OTHERS   = 1.
  ENDMETHOD.


  METHOD location.
    result = |{ zif_vsp_adt_feature=>co_root }{ collection( ) }/| &&
             |{ cl_http_utility=>escape_url( to_lower( name ) ) }|.
  ENDMETHOD.

ENDCLASS.
