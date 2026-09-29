"! <p class="shorttext synchronized">vsp ADT fallback: data elements</p>
"! /sap/bc/adt/zvsp/ddic/dataelements[/{name}] — the data element
"! document of the standard's /sap/bc/adt/ddic/dataelements (7.50 has it,
"! 7.40 not): the type, the field labels and the further properties, over
"! DDIF_DTEL_GET/PUT. The type kinds are the standard's: domain,
"! predefinedAbapType, refToPredefinedAbapType, refToDictionaryType,
"! refToClifType. See ZCL_VSP_ADT_DD_RESOURCE for the protocol.
CLASS zcl_vsp_adt_dtel_resource DEFINITION
  PUBLIC
  INHERITING FROM zcl_vsp_adt_dd_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'application/vnd.sap.adt.dataelements.v1+xml'.
    "! What newer releases name the same document.
    CONSTANTS co_content_type_v2 TYPE string
      VALUE 'application/vnd.sap.adt.dataelements.v2+xml'.
    CONSTANTS co_wbobj TYPE string
      VALUE 'http://www.sap.com/wbobj/dictionary/dtel'.
    CONSTANTS co_namespace TYPE string
      VALUE 'http://www.sap.com/adt/dictionary/dataelements'.

  PROTECTED SECTION.
    METHODS object_type REDEFINITION.
    METHODS adt_type REDEFINITION.
    METHODS collection REDEFINITION.
    METHODS content_type REDEFINITION.
    METHODS exists REDEFINITION.
    METHODS render REDEFINITION.
    METHODS write REDEFINITION.

  PRIVATE SECTION.
    "! The data element in a state, texts in its master language.
    METHODS read
      IMPORTING state    TYPE ddobjstate
      EXPORTING header   TYPE dd04v
                gotstate TYPE ddgotstate.

    CLASS-METHODS type_kind
      IMPORTING header        TYPE dd04v
      RETURNING VALUE(result) TYPE string.

    "! Sets the type fields of the header for a kind and type name.
    CLASS-METHODS set_type
      IMPORTING kind      TYPE string
                type_name TYPE string
                data_type TYPE string
                length    TYPE i
                decimals  TYPE i
      CHANGING  header    TYPE dd04v
      RAISING   cx_adt_rest.

    "! A label and its length; the length is the label's maximum when
    "! the document gives none.
    CLASS-METHODS take_label
      IMPORTING parent  TYPE REF TO if_ixml_element
                prefix  TYPE string
                maximum TYPE i
      CHANGING  label   TYPE csequence
                length  TYPE n
      RAISING   cx_adt_rest.
ENDCLASS.



CLASS zcl_vsp_adt_dtel_resource IMPLEMENTATION.

  METHOD object_type.
    result = 'DTEL'.
  ENDMETHOD.


  METHOD adt_type.
    result = 'DTEL/DE'.
  ENDMETHOD.


  METHOD collection.
    result = '/ddic/dataelements'.
  ENDMETHOD.


  METHOD content_type.
    result = co_content_type.
  ENDMETHOD.


  METHOD exists.
    DATA found TYPE rollname.

    SELECT SINGLE rollname FROM dd04l INTO found WHERE rollname = name.
    result = boolc( sy-subrc = 0 ).
  ENDMETHOD.


  METHOD read.
    DATA master_langu TYPE sylangu.
    DATA master TYPE dd04v.

    CLEAR: header, gotstate.
    CALL FUNCTION 'DDIF_DTEL_GET'
      EXPORTING
        name          = name
        state         = state
        langu         = sy-langu
      IMPORTING
        gotstate      = gotstate
        dd04v_wa      = header
      EXCEPTIONS
        illegal_input = 1
        OTHERS        = 2.
    IF sy-subrc <> 0 OR gotstate IS INITIAL.
      CLEAR gotstate.
      RETURN.
    ENDIF.

    " The texts are the master language's.
    master_langu = master_language( header-dtelmaster ).
    IF master_langu IS INITIAL OR master_langu = sy-langu.
      RETURN.
    ENDIF.
    CALL FUNCTION 'DDIF_DTEL_GET'
      EXPORTING
        name          = name
        state         = state
        langu         = master_langu
      IMPORTING
        dd04v_wa      = master
      EXCEPTIONS
        OTHERS        = 1.
    IF sy-subrc <> 0.
      RETURN.
    ENDIF.
    header-ddlanguage = master-ddlanguage.
    header-ddtext = master-ddtext.
    header-reptext = master-reptext.
    header-scrtext_s = master-scrtext_s.
    header-scrtext_m = master-scrtext_m.
    header-scrtext_l = master-scrtext_l.
  ENDMETHOD.


  METHOD type_kind.
    IF header-refkind = 'R'.
      CASE header-reftype.
        WHEN 'B'.
          result = 'refToPredefinedAbapType'.
        WHEN 'C' OR 'I' OR 'O'.
          result = 'refToClifType'.
        WHEN OTHERS.
          result = 'refToDictionaryType'.
      ENDCASE.
    ELSEIF header-domname IS NOT INITIAL.
      result = 'domain'.
    ELSE.
      result = 'predefinedAbapType'.
    ENDIF.
  ENDMETHOD.


  METHOD render.
    DATA header TYPE dd04v.
    DATA gotstate TYPE ddgotstate.
    DATA attributes TYPE string.
    DATA package_ref TYPE string.
    DATA kind TYPE string.
    DATA type_name TYPE string.
    DATA data_type TYPE string.
    DATA length TYPE string.
    DATA decimals TYPE string.

    read( EXPORTING state    = state
          IMPORTING header   = header
                    gotstate = gotstate ).
    IF gotstate IS INITIAL.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'DTEL'
          resource_id   = CONV #( name ).
    ENDIF.
    object_header( EXPORTING description = header-ddtext
                             gotstate    = gotstate
                             changed_by  = header-as4user
                             changed_on  = header-as4date
                             changed_at  = header-as4time
                   IMPORTING attributes  = attributes
                             package_ref = package_ref ).

    kind = type_kind( header ).
    length = header-leng.
    decimals = header-decimals.
    CASE kind.
      WHEN 'domain'.
        type_name = header-domname.
        data_type = header-datatype.
      WHEN 'predefinedAbapType'.
        data_type = header-datatype.
      WHEN 'refToPredefinedAbapType'.
        " The referenced type is kept where the domain would be.
        data_type = header-domname.
      WHEN OTHERS.
        type_name = header-domname.
        length = '000000'.
        decimals = '000000'.
    ENDCASE.

    result =
      |<?xml version="1.0" encoding="utf-8"?>| &&
      |<blue:wbobj { attributes } xmlns:blue="{ co_wbobj }" xmlns:adtcore="{ co_adtcore }">| &&
      package_ref &&
      |<dtel:dataElement xmlns:dtel="{ co_namespace }">| &&
      el( tag = 'dtel:typeKind' value = kind ) &&
      el( tag = 'dtel:typeName' value = type_name ) &&
      el( tag = 'dtel:dataType' value = data_type ) &&
      |<dtel:dataTypeLength>{ length }</dtel:dataTypeLength>| &&
      |<dtel:dataTypeDecimals>{ decimals }</dtel:dataTypeDecimals>| &&
      el( tag = 'dtel:shortFieldLabel' value = header-scrtext_s ) &&
      |<dtel:shortFieldLength>{ header-scrlen1 }</dtel:shortFieldLength>| &&
      |<dtel:shortFieldMaxLength>10</dtel:shortFieldMaxLength>| &&
      el( tag = 'dtel:mediumFieldLabel' value = header-scrtext_m ) &&
      |<dtel:mediumFieldLength>{ header-scrlen2 }</dtel:mediumFieldLength>| &&
      |<dtel:mediumFieldMaxLength>20</dtel:mediumFieldMaxLength>| &&
      el( tag = 'dtel:longFieldLabel' value = header-scrtext_l ) &&
      |<dtel:longFieldLength>{ header-scrlen3 }</dtel:longFieldLength>| &&
      |<dtel:longFieldMaxLength>40</dtel:longFieldMaxLength>| &&
      el( tag = 'dtel:headingFieldLabel' value = header-reptext ) &&
      |<dtel:headingFieldLength>{ header-headlen }</dtel:headingFieldLength>| &&
      |<dtel:headingFieldMaxLength>55</dtel:headingFieldMaxLength>| &&
      el( tag = 'dtel:searchHelp' value = header-shlpname ) &&
      el( tag = 'dtel:searchHelpParameter' value = header-shlpfield ) &&
      el( tag = 'dtel:setGetParameter' value = header-memoryid ) &&
      el( tag = 'dtel:defaultComponentName' value = header-deffdname ) &&
      |<dtel:deactivateInputHistory>| &&
      |{ bool( get_component( structure = header component = 'NOHISTORY' ) ) }| &&
      |</dtel:deactivateInputHistory>| &&
      |<dtel:changeDocument>{ bool( header-logflag ) }</dtel:changeDocument>| &&
      |<dtel:leftToRightDirection>| &&
      |{ bool( get_component( structure = header component = 'LTRFLDDIS' ) ) }| &&
      |</dtel:leftToRightDirection>| &&
      |<dtel:deactivateBIDIFiltering>| &&
      |{ bool( get_component( structure = header component = 'BIDICTRLC' ) ) }| &&
      |</dtel:deactivateBIDIFiltering>| &&
      |</dtel:dataElement>| &&
      |</blue:wbobj>|.
  ENDMETHOD.


  METHOD write.
    DATA header TYPE dd04v.
    DATA gotstate TYPE ddgotstate.
    DATA part TYPE REF TO if_ixml_element.
    DATA text TYPE string.
    DATA found TYPE abap_bool.
    DATA kind TYPE string.
    DATA type_name TYPE string.
    DATA data_type TYPE string.
    DATA length TYPE i.
    DATA decimals TYPE i.
    DATA type_given TYPE abap_bool.

    IF is_new = abap_false.
      read( EXPORTING state    = 'M'
            IMPORTING header   = header
                      gotstate = gotstate ).
    ELSE.
      header-dtelmaster = sy-langu.
    ENDIF.
    header-rollname = name.
    header-ddlanguage = master_language( header-dtelmaster ).
    IF header-ddlanguage IS INITIAL.
      header-ddlanguage = sy-langu.
    ENDIF.

    description_of( EXPORTING root = root IMPORTING description = text found = found ).
    IF found = abap_true.
      header-ddtext = text.
    ENDIF.

    part = child( parent = root namespace = co_namespace tag = 'dataElement' ).
    IF part IS NOT BOUND.
      IF is_new = abap_false.
        zcl_vsp_adt_txt_format=>raise_not_acceptable( 'The document has no dtel:dataElement' ).
      ENDIF.
    ELSE.
      " The type: the kind decides which of the other elements count.
      kind = type_kind( header ).
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'typeKind'
                  IMPORTING value = text found = found ).
      IF found = abap_true AND text IS NOT INITIAL.
        kind = text.
        type_given = abap_true.
      ENDIF.
      type_name = header-domname.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'typeName'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        type_name = to_upper( text ).
        type_given = abap_true.
      ENDIF.
      data_type = header-datatype.
      IF header-refkind = 'R' AND header-reftype = 'B'.
        data_type = header-domname.
      ENDIF.
      length = header-leng.
      decimals = header-decimals.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'dataType'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        data_type = to_upper( text ).
        type_given = abap_true.
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'dataTypeLength'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        length = int_of( value = text tag = 'dataTypeLength' ).
        type_given = abap_true.
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'dataTypeDecimals'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        decimals = int_of( value = text tag = 'dataTypeDecimals' ).
        type_given = abap_true.
      ENDIF.
      IF type_given = abap_true.
        set_type( EXPORTING kind      = kind
                            type_name = type_name
                            data_type = data_type
                            length    = length
                            decimals  = decimals
                  CHANGING  header    = header ).
      ENDIF.

      take_label( EXPORTING parent = part prefix = 'short' maximum = 10
                  CHANGING  label = header-scrtext_s length = header-scrlen1 ).
      take_label( EXPORTING parent = part prefix = 'medium' maximum = 20
                  CHANGING  label = header-scrtext_m length = header-scrlen2 ).
      take_label( EXPORTING parent = part prefix = 'long' maximum = 40
                  CHANGING  label = header-scrtext_l length = header-scrlen3 ).
      take_label( EXPORTING parent = part prefix = 'heading' maximum = 55
                  CHANGING  label = header-reptext length = header-headlen ).

      child_text( EXPORTING parent = part namespace = co_namespace tag = 'searchHelp'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-shlpname = to_upper( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'searchHelpParameter'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-shlpfield = to_upper( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'setGetParameter'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-memoryid = to_upper( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'defaultComponentName'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-deffdname = to_upper( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'changeDocument'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-logflag = flag_of( text ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'deactivateInputHistory'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        set_component( EXPORTING component = 'NOHISTORY' value = flag_of( text )
                       CHANGING  structure = header ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'leftToRightDirection'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        set_component( EXPORTING component = 'LTRFLDDIS' value = flag_of( text )
                       CHANGING  structure = header ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'deactivateBIDIFiltering'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        set_component( EXPORTING component = 'BIDICTRLC' value = flag_of( text )
                       CHANGING  structure = header ).
      ENDIF.
    ENDIF.

    CALL FUNCTION 'DDIF_DTEL_PUT'
      EXPORTING
        name              = name
        dd04v_wa          = header
      EXCEPTIONS
        dtel_not_found    = 1
        name_inconsistent = 2
        dtel_inconsistent = 3
        put_failure       = 4
        put_refused       = 5
        OTHERS            = 6.
    IF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = 'DTEL' resource_id = name ).
    ENDIF.
  ENDMETHOD.


  METHOD set_type.
    DATA clstype TYPE seoclstype.
    DATA found TYPE tabname.

    CLEAR: header-domname, header-refkind, header-reftype, header-datatype,
           header-leng, header-decimals.
    CASE kind.
      WHEN 'domain'.
        " The data type, length and decimals come from the domain on
        " activation.
        header-domname = type_name.
        header-refkind = 'D'.
      WHEN 'predefinedAbapType'.
        header-datatype = data_type.
        header-leng = length.
        header-decimals = decimals.
      WHEN 'refToPredefinedAbapType'.
        header-refkind = 'R'.
        header-reftype = 'B'.
        header-domname = data_type.
        header-datatype = 'REF'.
        header-leng = length.
        header-decimals = decimals.
      WHEN 'refToDictionaryType'.
        header-refkind = 'R'.
        header-domname = type_name.
        header-datatype = 'REF'.
        IF type_name = 'DATA'.
          header-reftype = 'D'.
        ELSE.
          SELECT SINGLE tabname FROM dd02l INTO found WHERE tabname = type_name.
          IF sy-subrc = 0.
            header-reftype = 'S'.
          ELSE.
            SELECT SINGLE typename FROM dd40l INTO found WHERE typename = type_name.
            IF sy-subrc = 0.
              header-reftype = 'L'.
            ELSE.
              header-reftype = 'E'.
            ENDIF.
          ENDIF.
        ENDIF.
      WHEN 'refToClifType'.
        header-refkind = 'R'.
        header-domname = type_name.
        header-datatype = 'REF'.
        IF type_name = 'OBJECT'.
          header-reftype = 'O'.
        ELSE.
          SELECT SINGLE clstype FROM seoclass INTO clstype WHERE clsname = type_name.
          IF sy-subrc = 0 AND clstype = '1'.
            header-reftype = 'I'.
          ELSE.
            header-reftype = 'C'.
          ENDIF.
        ENDIF.
      WHEN OTHERS.
        zcl_vsp_adt_txt_format=>raise_not_acceptable(
          |Unknown typeKind "{ kind }": want domain, predefinedAbapType, | &&
          |refToPredefinedAbapType, refToDictionaryType or refToClifType| ).
    ENDCASE.
  ENDMETHOD.


  METHOD take_label.
    DATA text TYPE string.
    DATA found TYPE abap_bool.
    DATA label_found TYPE abap_bool.
    DATA tag TYPE string.

    child_text( EXPORTING parent = parent namespace = co_namespace tag = |{ prefix }FieldLabel|
                IMPORTING value = text found = label_found ).
    IF label_found = abap_true.
      label = text.
    ENDIF.
    tag = |{ prefix }FieldLength|.
    child_text( EXPORTING parent = parent namespace = co_namespace tag = tag
                IMPORTING value = text found = found ).
    IF found = abap_true AND text IS NOT INITIAL.
      length = int_of( value = text tag = tag ).
    ELSEIF label_found = abap_true AND length IS INITIAL.
      length = maximum.
    ENDIF.
  ENDMETHOD.

ENDCLASS.
