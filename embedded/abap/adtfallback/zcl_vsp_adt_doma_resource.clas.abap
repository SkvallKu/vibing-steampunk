"! <p class="shorttext synchronized">vsp ADT fallback: domains</p>
"! /sap/bc/adt/zvsp/ddic/domains[/{name}] — the domain document of the
"! standard's /sap/bc/adt/ddic/domains (7.57 has it, 7.50 and 7.40 not): data type, output characteristics, value table and
"! fixed values, over DDIF_DOMA_GET/PUT. Fixed values contributed by a
"! fixed value append belong to the append and are neither shown nor
"! written here. See ZCL_VSP_ADT_DD_RESOURCE for the protocol.
CLASS zcl_vsp_adt_doma_resource DEFINITION
  PUBLIC
  INHERITING FROM zcl_vsp_adt_dd_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'application/vnd.sap.adt.domains.v2+xml'.
    CONSTANTS co_namespace TYPE string
      VALUE 'http://www.sap.com/dictionary/domain'.

  PROTECTED SECTION.
    METHODS object_type REDEFINITION.
    METHODS adt_type REDEFINITION.
    METHODS collection REDEFINITION.
    METHODS content_type REDEFINITION.
    METHODS exists REDEFINITION.
    METHODS render REDEFINITION.
    METHODS write REDEFINITION.

  PRIVATE SECTION.
    TYPES tt_values TYPE STANDARD TABLE OF dd07v WITH DEFAULT KEY.

    "! The domain in a state, texts in its master language.
    METHODS read
      IMPORTING state    TYPE ddobjstate
      EXPORTING header   TYPE dd01v
                values   TYPE tt_values
                gotstate TYPE ddgotstate.
ENDCLASS.



CLASS zcl_vsp_adt_doma_resource IMPLEMENTATION.

  METHOD object_type.
    result = 'DOMA'.
  ENDMETHOD.


  METHOD adt_type.
    result = 'DOMA/DD'.
  ENDMETHOD.


  METHOD collection.
    result = '/ddic/domains'.
  ENDMETHOD.


  METHOD content_type.
    result = co_content_type.
  ENDMETHOD.


  METHOD exists.
    DATA found TYPE domname.

    SELECT SINGLE domname FROM dd01l INTO found WHERE domname = name.
    result = boolc( sy-subrc = 0 ).
  ENDMETHOD.


  METHOD read.
    DATA master_langu TYPE sylangu.
    DATA master_values TYPE tt_values.
    DATA master TYPE dd01v.

    CLEAR: header, values, gotstate.
    CALL FUNCTION 'DDIF_DOMA_GET'
      EXPORTING
        name          = name
        state         = state
        langu         = sy-langu
      IMPORTING
        gotstate      = gotstate
        dd01v_wa      = header
      TABLES
        dd07v_tab     = values
      EXCEPTIONS
        illegal_input = 1
        OTHERS        = 2.
    IF sy-subrc <> 0 OR gotstate IS INITIAL.
      CLEAR gotstate.
      RETURN.
    ENDIF.

    " The texts are the master language's.
    master_langu = master_language( header-dommaster ).
    IF master_langu IS INITIAL OR master_langu = sy-langu.
      RETURN.
    ENDIF.
    CALL FUNCTION 'DDIF_DOMA_GET'
      EXPORTING
        name          = name
        state         = state
        langu         = master_langu
      IMPORTING
        dd01v_wa      = master
      TABLES
        dd07v_tab     = master_values
      EXCEPTIONS
        OTHERS        = 1.
    IF sy-subrc = 0.
      header-ddtext = master-ddtext.
      header-ddlanguage = master-ddlanguage.
      values = master_values.
    ENDIF.
  ENDMETHOD.


  METHOD render.
    DATA header TYPE dd01v.
    DATA values TYPE tt_values.
    DATA value TYPE dd07v.
    DATA gotstate TYPE ddgotstate.
    DATA attributes TYPE string.
    DATA package_ref TYPE string.
    DATA value_table TYPE string.
    DATA fixed TYPE string.

    read( EXPORTING state    = state
          IMPORTING header   = header
                    values   = values
                    gotstate = gotstate ).
    IF gotstate IS INITIAL.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'DOMA'
          resource_id   = CONV #( name ).
    ENDIF.
    object_header( EXPORTING description = header-ddtext
                             gotstate    = gotstate
                             changed_by  = header-as4user
                             changed_on  = header-as4date
                             changed_at  = header-as4time
                   IMPORTING attributes  = attributes
                             package_ref = package_ref ).

    IF header-entitytab IS INITIAL.
      value_table = '<doma:valueTableRef/>'.
    ELSE.
      value_table =
        |<doma:valueTableRef adtcore:uri="/sap/bc/adt/ddic/tables/| &&
        |{ cl_http_utility=>escape_url( to_lower( header-entitytab ) ) }" | &&
        |adtcore:type="TABL/DT" adtcore:name="{ esc( header-entitytab ) }"/>|.
    ENDIF.

    SORT values BY valpos.
    LOOP AT values INTO value WHERE appval IS INITIAL.
      fixed = fixed && '<doma:fixValue>' &&
        el( tag = 'doma:position' value = value-valpos ) &&
        el( tag = 'doma:low' value = value-domvalue_l ) &&
        el( tag = 'doma:high' value = value-domvalue_h ) &&
        el( tag = 'doma:text' value = value-ddtext ) &&
        '</doma:fixValue>'.
    ENDLOOP.
    IF fixed IS INITIAL.
      fixed = '<doma:fixValues/>'.
    ELSE.
      fixed = |<doma:fixValues>{ fixed }</doma:fixValues>|.
    ENDIF.

    result =
      |<?xml version="1.0" encoding="utf-8"?>| &&
      |<doma:domain { attributes } xmlns:doma="{ co_namespace }" xmlns:adtcore="{ co_adtcore }">| &&
      package_ref &&
      |<doma:content>| &&
      |<doma:typeInformation>| &&
      el( tag = 'doma:datatype' value = header-datatype ) &&
      |<doma:length>{ header-leng }</doma:length>| &&
      |<doma:decimals>{ header-decimals }</doma:decimals>| &&
      |</doma:typeInformation>| &&
      |<doma:outputInformation>| &&
      |<doma:length>{ header-outputlen }</doma:length>| &&
      el( tag = 'doma:style' value = get_component( structure = header component = 'OUTPUTSTYLE' ) ) &&
      el( tag = 'doma:conversionExit' value = header-convexit ) &&
      |<doma:signExists>{ bool( header-signflag ) }</doma:signExists>| &&
      |<doma:lowercase>{ bool( header-lowercase ) }</doma:lowercase>| &&
      |<doma:ampmFormat>{ bool( get_component( structure = header component = 'AMPMFORMAT' ) ) }</doma:ampmFormat>| &&
      |</doma:outputInformation>| &&
      |<doma:valueInformation>| &&
      value_table &&
      |<doma:appendExists>{ bool( header-appexist ) }</doma:appendExists>| &&
      fixed &&
      |</doma:valueInformation>| &&
      |</doma:content>| &&
      |</doma:domain>|.
  ENDMETHOD.


  METHOD write.
    DATA header TYPE dd01v.
    DATA values TYPE tt_values.
    DATA own TYPE tt_values.
    DATA value TYPE dd07v.
    DATA gotstate TYPE ddgotstate.
    DATA part TYPE REF TO if_ixml_element.
    DATA element TYPE REF TO if_ixml_element.
    DATA list TYPE REF TO if_ixml_node_collection.
    DATA iterator TYPE REF TO if_ixml_node_iterator.
    DATA node TYPE REF TO if_ixml_node.
    DATA text TYPE string.
    DATA found TYPE abap_bool.
    DATA position TYPE i.

    IF is_new = abap_false.
      read( EXPORTING state    = 'M'
            IMPORTING header   = header
                      values   = values
                      gotstate = gotstate ).
      LOOP AT values INTO value WHERE appval IS INITIAL.
        APPEND value TO own.
      ENDLOOP.
    ELSE.
      header-dommaster = sy-langu.
    ENDIF.
    header-domname = name.
    header-ddlanguage = master_language( header-dommaster ).
    IF header-ddlanguage IS INITIAL.
      header-ddlanguage = sy-langu.
    ENDIF.

    description_of( EXPORTING root = root IMPORTING description = text found = found ).
    IF found = abap_true.
      header-ddtext = text.
    ENDIF.

    part = child( parent = root namespace = co_namespace tag = 'typeInformation' ).
    IF part IS BOUND.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'datatype'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-datatype = to_upper( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'length'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-leng = int_of( value = text tag = 'typeInformation/length' ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'decimals'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-decimals = int_of( value = text tag = 'typeInformation/decimals' ).
      ENDIF.
    ENDIF.

    part = child( parent = root namespace = co_namespace tag = 'outputInformation' ).
    IF part IS BOUND.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'length'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-outputlen = int_of( value = text tag = 'outputInformation/length' ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'style'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        set_component( EXPORTING component = 'OUTPUTSTYLE' value = text
                       CHANGING  structure = header ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'conversionExit'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-convexit = to_upper( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'signExists'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-signflag = flag_of( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'lowercase'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-lowercase = flag_of( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'ampmFormat'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        set_component( EXPORTING component = 'AMPMFORMAT' value = flag_of( text )
                       CHANGING  structure = header ).
      ENDIF.
    ENDIF.

    part = child( parent = root namespace = co_namespace tag = 'valueInformation' ).
    IF part IS BOUND.
      element = child( parent = part namespace = co_namespace tag = 'valueTableRef' ).
      IF element IS BOUND.
        header-entitytab = to_upper( element->get_attribute_ns( name = 'name' uri = co_adtcore ) ).
      ENDIF.

      " The fixed values of the document replace the domain's own.
      element = child( parent = part namespace = co_namespace tag = 'fixValues' ).
      IF element IS BOUND.
        CLEAR own.
        list = element->get_elements_by_tag_name_ns( name = 'fixValue' uri = co_namespace ).
        iterator = list->create_iterator( ).
        node = iterator->get_next( ).
        WHILE node IS BOUND.
          element ?= node.
          CLEAR value.
          position = position + 1.
          value-domname = name.
          child_text( EXPORTING parent = element namespace = co_namespace tag = 'position'
                      IMPORTING value = text ).
          value-valpos = int_of( value = text tag = 'fixValue/position' ).
          IF value-valpos IS INITIAL.
            value-valpos = position.
          ENDIF.
          child_text( EXPORTING parent = element namespace = co_namespace tag = 'low'
                      IMPORTING value = text ).
          value-domvalue_l = text.
          child_text( EXPORTING parent = element namespace = co_namespace tag = 'high'
                      IMPORTING value = text ).
          value-domvalue_h = text.
          child_text( EXPORTING parent = element namespace = co_namespace tag = 'text'
                      IMPORTING value = text ).
          value-ddtext = text.
          APPEND value TO own.
          node = iterator->get_next( ).
        ENDWHILE.
      ENDIF.
    ENDIF.

    IF header-outputlen IS INITIAL.
      header-outputlen = header-leng.
    ENDIF.
    IF own IS INITIAL.
      CLEAR header-valexi.
    ELSE.
      header-valexi = abap_true.
    ENDIF.
    LOOP AT own INTO value.
      value-ddlanguage = header-ddlanguage.
      MODIFY own FROM value.
    ENDLOOP.

    CALL FUNCTION 'DDIF_DOMA_PUT'
      EXPORTING
        name              = name
        dd01v_wa          = header
      TABLES
        dd07v_tab         = own
      EXCEPTIONS
        doma_not_found    = 1
        name_inconsistent = 2
        doma_inconsistent = 3
        put_failure       = 4
        put_refused       = 5
        OTHERS            = 6.
    IF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = 'DOMA' resource_id = name ).
    ENDIF.
  ENDMETHOD.

ENDCLASS.
