"! <p class="shorttext synchronized">vsp ADT fallback: table types</p>
"! /sap/bc/adt/zvsp/ddic/tabletypes[/{name}] — the table type document of
"! the standard's /sap/bc/adt/ddic/tabletypes (7.57 has it, 7.50 and 7.40
"! not): row type, initial row count, access, primary key and secondary
"! keys, over DDIF_TTYP_GET/PUT. Mapped as the standard maps them: a
"! dictionary row type is stored as structured and the activation names
"! what it is; a range table type keeps its range structure in ROWTYPE,
"! its data element in RANGE_CTYP and its predefined type in CT*; the
"! range structure is not generated. Predefined types carry their ADT
"! names (STRING, not STRG). See ZCL_VSP_ADT_DD_RESOURCE for the protocol.
CLASS zcl_vsp_adt_ttyp_resource DEFINITION
  PUBLIC
  INHERITING FROM zcl_vsp_adt_dd_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'application/vnd.sap.adt.tabletype.v1+xml'.
    CONSTANTS co_namespace TYPE string
      VALUE 'http://www.sap.com/dictionary/tabletype'.

  PROTECTED SECTION.
    METHODS object_type REDEFINITION.
    METHODS adt_type REDEFINITION.
    METHODS collection REDEFINITION.
    METHODS content_type REDEFINITION.
    METHODS exists REDEFINITION.
    METHODS render REDEFINITION.
    METHODS write REDEFINITION.

  PRIVATE SECTION.
    TYPES tt_dd42v TYPE STANDARD TABLE OF dd42v WITH DEFAULT KEY.
    TYPES tt_dd43v TYPE STANDARD TABLE OF dd43v WITH DEFAULT KEY.
    TYPES:
      BEGIN OF ty_code,
        kind TYPE c LENGTH 1,
        code TYPE c LENGTH 1,
        name TYPE string,
      END OF ty_code.
    TYPES tt_codes TYPE STANDARD TABLE OF ty_code WITH DEFAULT KEY.

    "! The table type in a state, texts in its master language.
    METHODS read
      IMPORTING state    TYPE ddobjstate
      EXPORTING header   TYPE dd40v
                keys     TYPE tt_dd42v
                seckeys  TYPE tt_dd43v
                gotstate TYPE ddgotstate.

    METHODS row_type_kind
      IMPORTING header        TYPE dd40v
      RETURNING VALUE(result) TYPE string.

    "! Sets the row type fields of the header for a document's row type.
    METHODS set_row_type
      IMPORTING part   TYPE REF TO if_ixml_element
      CHANGING  header TYPE dd40v
      RAISING   cx_adt_rest.

    "! What the dictionary says a type is: DTEL, TTYP, CLAS, INTF...
    CLASS-METHODS type_kind_of
      IMPORTING type_name     TYPE csequence
      RETURNING VALUE(result) TYPE string.

    "! The dictionary's code for a document value of a kind (A access,
    "! D key definition, K key kind, F further secondary keys, S secondary
    "! key definition), or the value for a code.
    CLASS-METHODS codes
      RETURNING VALUE(result) TYPE tt_codes.

    CLASS-METHODS code_of
      IMPORTING category      TYPE csequence
                value         TYPE csequence
                tag           TYPE csequence
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_rest.

    CLASS-METHODS name_of
      IMPORTING category      TYPE csequence
                stored        TYPE csequence
      RETURNING VALUE(result) TYPE string.

    "! The ADT name of a predefined type (STRING for STRG), and back.
    CLASS-METHODS adt_data_type
      IMPORTING ddic          TYPE csequence
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS ddic_data_type
      IMPORTING adt           TYPE csequence
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS components_xml
      IMPORTING keys          TYPE tt_dd42v
                seckeyname    TYPE csequence
      RETURNING VALUE(result) TYPE string.

    "! The component names below an element, in order.
    CLASS-METHODS component_names
      IMPORTING parent        TYPE REF TO if_ixml_element
      RETURNING VALUE(result) TYPE string_table.

    CLASS-METHODS value_helps
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS help
      IMPORTING key           TYPE string
                value         TYPE string
      RETURNING VALUE(result) TYPE string.
ENDCLASS.



CLASS zcl_vsp_adt_ttyp_resource IMPLEMENTATION.

  METHOD object_type.
    result = 'TTYP'.
  ENDMETHOD.


  METHOD adt_type.
    result = 'TTYP/DA'.
  ENDMETHOD.


  METHOD collection.
    result = '/ddic/tabletypes'.
  ENDMETHOD.


  METHOD content_type.
    result = co_content_type.
  ENDMETHOD.


  METHOD exists.
    DATA found TYPE ttypename.

    SELECT SINGLE typename FROM dd40l INTO found WHERE typename = name.
    result = boolc( sy-subrc = 0 ).
  ENDMETHOD.


  METHOD read.
    DATA master_langu TYPE sylangu.
    DATA master TYPE dd40v.
    DATA master_keys TYPE tt_dd42v.
    DATA master_seckeys TYPE tt_dd43v.
    DATA seckey TYPE dd43v.
    DATA master_seckey TYPE dd43v.

    CLEAR: header, keys, seckeys, gotstate.
    CALL FUNCTION 'DDIF_TTYP_GET'
      EXPORTING
        name          = name
        state         = state
        langu         = sy-langu
      IMPORTING
        gotstate      = gotstate
        dd40v_wa      = header
      TABLES
        dd42v_tab     = keys
        dd43v_tab     = seckeys
      EXCEPTIONS
        illegal_input = 1
        OTHERS        = 2.
    IF sy-subrc <> 0 OR gotstate IS INITIAL.
      CLEAR gotstate.
      RETURN.
    ENDIF.

    " The texts are the master language's.
    master_langu = master_language( space ).
    IF master_langu IS INITIAL OR master_langu = sy-langu.
      RETURN.
    ENDIF.
    CALL FUNCTION 'DDIF_TTYP_GET'
      EXPORTING
        name          = name
        state         = state
        langu         = master_langu
      IMPORTING
        dd40v_wa      = master
      TABLES
        dd42v_tab     = master_keys
        dd43v_tab     = master_seckeys
      EXCEPTIONS
        OTHERS        = 1.
    IF sy-subrc <> 0.
      RETURN.
    ENDIF.
    header-ddtext = master-ddtext.
    header-ddlanguage = master-ddlanguage.
    LOOP AT seckeys INTO seckey.
      READ TABLE master_seckeys INTO master_seckey WITH KEY seckeyname = seckey-seckeyname.
      IF sy-subrc = 0.
        seckey-keydescription = master_seckey-keydescription.
        seckey-ddlanguage = master_seckey-ddlanguage.
        MODIFY seckeys FROM seckey.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.


  METHOD row_type_kind.
    IF header-ttypkind = 'R'.
      IF header-range_ctyp IS INITIAL.
        result = 'rangeTypeOnPredefinedType'.
      ELSE.
        result = 'rangeTypeOnDataelement'.
      ENDIF.
    ELSEIF header-rowkind = 'R'.
      CASE header-reftype.
        WHEN 'B'.
          result = 'refToPredefinedAbapType'.
        WHEN 'C' OR 'I' OR 'O'.
          result = 'refToClassOrInterfaceType'.
        WHEN OTHERS.
          result = 'refToDictionaryType'.
      ENDCASE.
    ELSEIF header-rowkind IS INITIAL.
      result = 'predefinedAbapType'.
    ELSE.
      result = 'dictionaryType'.
    ENDIF.
  ENDMETHOD.


  METHOD render.
    DATA header TYPE dd40v.
    DATA keys TYPE tt_dd42v.
    DATA seckeys TYPE tt_dd43v.
    DATA seckey TYPE dd43v.
    DATA gotstate TYPE ddgotstate.
    DATA attributes TYPE string.
    DATA package_ref TYPE string.
    DATA kind TYPE string.
    DATA type_name TYPE string.
    DATA data_type TYPE string.
    DATA leng TYPE ddleng.
    DATA decimals TYPE decimals.
    DATA range_type TYPE string.
    DATA editable TYPE string VALUE 'true'.
    DATA access TYPE string.
    DATA secondary TYPE string.

    read( EXPORTING state    = state
          IMPORTING header   = header
                    keys     = keys
                    seckeys  = seckeys
                    gotstate = gotstate ).
    IF gotstate IS INITIAL.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'TTYP'
          resource_id   = CONV #( name ).
    ENDIF.
    object_header( EXPORTING description = header-ddtext
                             gotstate    = gotstate
                             changed_by  = header-as4user
                             changed_on  = header-as4date
                             changed_at  = header-as4time
                   IMPORTING attributes  = attributes
                             package_ref = package_ref ).

    kind = row_type_kind( header ).
    leng = header-leng.
    decimals = header-decimals.
    CASE kind.
      WHEN 'rangeTypeOnDataelement' OR 'rangeTypeOnPredefinedType'.
        type_name = header-range_ctyp.
        data_type = adt_data_type( header-ctdatatype ).
        leng = header-ctleng.
        decimals = header-ctdecimals.
        range_type = header-rowtype.
        editable = 'false'.
      WHEN 'refToPredefinedAbapType'.
        data_type = header-rowtype.
      WHEN 'refToDictionaryType' OR 'refToClassOrInterfaceType'.
        type_name = header-rowtype.
      WHEN OTHERS.
        type_name = header-rowtype.
        data_type = adt_data_type( header-datatype ).
    ENDCASE.

    SORT keys BY seckeyname keyfdpos.
    LOOP AT seckeys INTO seckey.
      IF seckey-seckeyunique IS INITIAL.
        access = 'nonUniqueSorted'.
      ELSEIF seckey-accessmode = 'H'.
        access = 'uniqueHashed'.
      ELSE.
        access = 'uniqueSorted'.
      ENDIF.
      secondary = secondary &&
        |<ttyp:secondaryKey ttyp:identifier="{ esc( seckey-seckeyname ) }" | &&
        |ttyp:description="{ esc( seckey-keydescription ) }" ttyp:language="{ esc( seckey-ddlanguage ) }" | &&
        |ttyp:access="{ access }" ttyp:definition="{ name_of( category = 'S' stored = seckey-kind ) }">| &&
        components_xml( keys = keys seckeyname = seckey-seckeyname ) &&
        |</ttyp:secondaryKey>|.
    ENDLOOP.

    result =
      |<?xml version="1.0" encoding="utf-8"?>| &&
      |<ttyp:tableType { attributes } adtcore:descriptionTextLimit="60" | &&
      |xmlns:ttyp="{ co_namespace }" xmlns:adtcore="{ co_adtcore }">| &&
      package_ref &&
      |<ttyp:rowType>| &&
      el( tag = 'ttyp:typeKind' value = kind ) &&
      el( tag = 'ttyp:typeName' value = type_name ) &&
      |<ttyp:builtInType>| &&
      el( tag = 'ttyp:dataType' value = data_type ) &&
      |<ttyp:length>{ leng }</ttyp:length>| &&
      |<ttyp:decimals>{ decimals }</ttyp:decimals>| &&
      |</ttyp:builtInType>| &&
      el( tag = 'ttyp:rangeType' value = range_type ) &&
      |</ttyp:rowType>| &&
      |<ttyp:initialRowCount>{ header-occurs }</ttyp:initialRowCount>| &&
      el( tag = 'ttyp:accessType' value = name_of( category = 'A' stored = header-accessmode ) ) &&
      |<ttyp:primaryKey ttyp:isVisible="{ editable }" ttyp:isEditable="{ editable }">| &&
      el( tag = 'ttyp:definition' value = name_of( category = 'D' stored = header-keydef ) ) &&
      el( tag = 'ttyp:kind' value = name_of( category = 'K' stored = header-keykind ) ) &&
      components_xml( keys = keys seckeyname = space ) &&
      el( tag = 'ttyp:alias' value = header-alias ) &&
      |</ttyp:primaryKey>| &&
      |<ttyp:secondaryKeys ttyp:isVisible="{ editable }" ttyp:isEditable="{ editable }">| &&
      el( tag = 'ttyp:allowed' value = name_of( category = 'F' stored = header-furtherseckey ) ) &&
      secondary &&
      |</ttyp:secondaryKeys>| &&
      value_helps( ) &&
      |</ttyp:tableType>|.
  ENDMETHOD.


  METHOD write.
    DATA header TYPE dd40v.
    DATA keys TYPE tt_dd42v.
    DATA seckeys TYPE tt_dd43v.
    DATA key TYPE dd42v.
    DATA seckey TYPE dd43v.
    DATA gotstate TYPE ddgotstate.
    DATA part TYPE REF TO if_ixml_element.
    DATA element TYPE REF TO if_ixml_element.
    DATA list TYPE REF TO if_ixml_node_collection.
    DATA iterator TYPE REF TO if_ixml_node_iterator.
    DATA node TYPE REF TO if_ixml_node.
    DATA text TYPE string.
    DATA found TYPE abap_bool.
    DATA names TYPE string_table.
    DATA component TYPE string.
    DATA position TYPE i.
    DATA access TYPE string.

    IF is_new = abap_false.
      read( EXPORTING state    = 'M'
            IMPORTING header   = header
                      keys     = keys
                      seckeys  = seckeys
                      gotstate = gotstate ).
    ELSE.
      " The standard's defaults for a new table type: a standard table of
      " CHAR 1, standard key, non-unique.
      header-accessmode = 'T'.
      header-datatype = 'CHAR'.
      header-leng = 1.
      header-keydef = 'D'.
      header-keykind = 'N'.
    ENDIF.
    header-typename = name.
    header-ddlanguage = master_language( space ).
    IF header-ddlanguage IS INITIAL.
      header-ddlanguage = sy-langu.
    ENDIF.

    description_of( EXPORTING root = root IMPORTING description = text found = found ).
    IF found = abap_true.
      header-ddtext = text.
    ENDIF.

    part = child( parent = root namespace = co_namespace tag = 'rowType' ).
    IF part IS BOUND.
      set_row_type( EXPORTING part = part CHANGING header = header ).
    ENDIF.

    child_text( EXPORTING parent = root namespace = co_namespace tag = 'initialRowCount'
                IMPORTING value = text found = found ).
    IF found = abap_true.
      header-occurs = int_of( value = text tag = 'initialRowCount' ).
    ENDIF.
    child_text( EXPORTING parent = root namespace = co_namespace tag = 'accessType'
                IMPORTING value = text found = found ).
    IF found = abap_true.
      header-accessmode = code_of( category = 'A' value = text tag = 'accessType' ).
    ENDIF.

    part = child( parent = root namespace = co_namespace tag = 'primaryKey' ).
    IF part IS BOUND.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'definition'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-keydef = code_of( category = 'D' value = text tag = 'primaryKey/definition' ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'kind'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-keykind = code_of( category = 'K' value = text tag = 'primaryKey/kind' ).
      ENDIF.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'alias'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-alias = to_upper( text ).
      ENDIF.
      element = child( parent = part namespace = co_namespace tag = 'components' ).
      IF element IS BOUND.
        DELETE keys WHERE seckeyname IS INITIAL.
        names = component_names( part ).
        LOOP AT names INTO component.
          CLEAR key.
          key-typename = name.
          key-keyfdpos = sy-tabix.
          key-keyfield = component.
          APPEND key TO keys.
        ENDLOOP.
      ENDIF.
    ENDIF.

    part = child( parent = root namespace = co_namespace tag = 'secondaryKeys' ).
    IF part IS BOUND.
      child_text( EXPORTING parent = part namespace = co_namespace tag = 'allowed'
                  IMPORTING value = text found = found ).
      IF found = abap_true.
        header-furtherseckey = code_of( category = 'F' value = text tag = 'secondaryKeys/allowed' ).
      ENDIF.

      " The keys of the document replace the table type's own.
      DELETE keys WHERE seckeyname IS NOT INITIAL.
      CLEAR seckeys.
      list = part->get_elements_by_tag_name_ns( name = 'secondaryKey' uri = co_namespace ).
      iterator = list->create_iterator( ).
      node = iterator->get_next( ).
      WHILE node IS BOUND.
        element ?= node.
        CLEAR seckey.
        seckey-typename = name.
        seckey-seckeyname = to_upper( element->get_attribute_ns( name = 'identifier' uri = co_namespace ) ).
        IF seckey-seckeyname IS INITIAL.
          zcl_vsp_adt_txt_format=>raise_not_acceptable( 'A secondary key has no ttyp:identifier' ).
        ENDIF.
        READ TABLE seckeys TRANSPORTING NO FIELDS WITH KEY seckeyname = seckey-seckeyname.
        IF sy-subrc = 0.
          zcl_vsp_adt_txt_format=>raise_not_acceptable( |Secondary key { seckey-seckeyname } is there twice| ).
        ENDIF.
        seckey-keydescription = element->get_attribute_ns( name = 'description' uri = co_namespace ).
        seckey-ddlanguage = header-ddlanguage.
        access = element->get_attribute_ns( name = 'access' uri = co_namespace ).
        CASE access.
          WHEN 'uniqueHashed'.
            seckey-accessmode = 'H'.
            seckey-seckeyunique = abap_true.
          WHEN 'uniqueSorted'.
            seckey-accessmode = 'S'.
            seckey-seckeyunique = abap_true.
          WHEN 'nonUniqueSorted'.
            seckey-accessmode = 'S'.
          WHEN OTHERS.
            zcl_vsp_adt_txt_format=>raise_not_acceptable(
              |Secondary key { seckey-seckeyname }: access "{ access }" is not | &&
              |uniqueHashed, uniqueSorted or nonUniqueSorted| ).
        ENDCASE.
        text = element->get_attribute_ns( name = 'definition' uri = co_namespace ).
        IF text IS INITIAL.
          text = 'keyComponents'.
        ENDIF.
        seckey-kind = code_of( category = 'S' value = text tag = |secondaryKey { seckey-seckeyname }/definition| ).
        APPEND seckey TO seckeys.
        position = 0.
        names = component_names( element ).
        LOOP AT names INTO component.
          CLEAR key.
          position = position + 1.
          key-typename = name.
          key-seckeyname = seckey-seckeyname.
          key-keyfdpos = position.
          key-keyfield = component.
          APPEND key TO keys.
        ENDLOOP.
        node = iterator->get_next( ).
      ENDWHILE.
    ENDIF.

    CALL FUNCTION 'DDIF_TTYP_PUT'
      EXPORTING
        name              = name
        dd40v_wa          = header
      TABLES
        dd42v_tab         = keys
        dd43v_tab         = seckeys
      EXCEPTIONS
        ttyp_not_found    = 1
        name_inconsistent = 2
        ttyp_inconsistent = 3
        put_failure       = 4
        put_refused       = 5
        OTHERS            = 6.
    IF sy-subrc <> 0.
      zcl_vsp_adt_util=>raise_failure( resource_type = 'TTYP' resource_id = name ).
    ENDIF.
  ENDMETHOD.


  METHOD set_row_type.
    DATA kind TYPE string.
    DATA type_name TYPE string.
    DATA data_type TYPE string.
    DATA range_type TYPE string.
    DATA leng TYPE i.
    DATA decimals TYPE i.
    DATA text TYPE string.
    DATA found TYPE abap_bool.
    DATA builtin TYPE REF TO if_ixml_element.

    child_text( EXPORTING parent = part namespace = co_namespace tag = 'typeKind'
                IMPORTING value = kind found = found ).
    IF found = abap_false.
      RETURN.
    ENDIF.
    child_text( EXPORTING parent = part namespace = co_namespace tag = 'typeName'
                IMPORTING value = type_name ).
    type_name = to_upper( condense( type_name ) ).
    child_text( EXPORTING parent = part namespace = co_namespace tag = 'rangeType'
                IMPORTING value = range_type ).
    range_type = to_upper( condense( range_type ) ).
    builtin = child( parent = part namespace = co_namespace tag = 'builtInType' ).
    IF builtin IS BOUND.
      child_text( EXPORTING parent = builtin namespace = co_namespace tag = 'dataType'
                  IMPORTING value = data_type ).
      data_type = ddic_data_type( to_upper( condense( data_type ) ) ).
      child_text( EXPORTING parent = builtin namespace = co_namespace tag = 'length'
                  IMPORTING value = text ).
      leng = int_of( value = text tag = 'builtInType/length' ).
      child_text( EXPORTING parent = builtin namespace = co_namespace tag = 'decimals'
                  IMPORTING value = text ).
      decimals = int_of( value = text tag = 'builtInType/decimals' ).
    ENDIF.

    CLEAR: header-rowtype, header-rowkind, header-reftype, header-ttypkind, header-range_ctyp,
           header-datatype, header-leng, header-decimals,
           header-ctdatatype, header-ctleng, header-ctdecimals.
    CASE kind.
      WHEN 'dictionaryType'.
        " Stored as structured; the activation names what the type is.
        header-rowkind = 'S'.
        header-rowtype = type_name.
        header-datatype = data_type.
        header-leng = leng.
        header-decimals = decimals.
      WHEN 'predefinedAbapType'.
        header-datatype = data_type.
        header-leng = leng.
        header-decimals = decimals.
      WHEN 'refToPredefinedAbapType'.
        header-rowkind = 'R'.
        header-reftype = 'B'.
        header-rowtype = data_type.
      WHEN 'refToDictionaryType'.
        header-rowkind = 'R'.
        header-rowtype = type_name.
        CASE type_name.
          WHEN 'DATA'.
            header-reftype = 'D'.
          WHEN 'OBJECT'.
            header-reftype = 'O'.
          WHEN OTHERS.
            CASE type_kind_of( type_name ).
              WHEN 'DTEL'.
                header-reftype = 'E'.
              WHEN 'TTYP'.
                header-reftype = 'L'.
              WHEN OTHERS.
                header-reftype = 'S'.
            ENDCASE.
        ENDCASE.
      WHEN 'refToClassOrInterfaceType'.
        header-rowkind = 'R'.
        header-rowtype = type_name.
        CASE type_kind_of( type_name ).
          WHEN 'CLAS'.
            header-reftype = 'C'.
          WHEN 'INTF'.
            header-reftype = 'I'.
          WHEN OTHERS.
            header-reftype = 'O'.
        ENDCASE.
      WHEN 'rangeTypeOnDataelement' OR 'rangeTypeOnPredefinedType'.
        IF range_type IS INITIAL.
          zcl_vsp_adt_txt_format=>raise_not_acceptable( 'A range table type names its range structure in rangeType' ).
        ENDIF.
        header-rowkind = 'S'.
        header-ttypkind = 'R'.
        header-rowtype = range_type.
        header-ctdatatype = data_type.
        header-ctleng = leng.
        header-ctdecimals = decimals.
        IF kind = 'rangeTypeOnDataelement'.
          header-range_ctyp = type_name.
        ENDIF.
        " The key of a range table type is fixed.
        header-accessmode = 'T'.
        header-keydef = 'D'.
        header-keykind = 'N'.
      WHEN OTHERS.
        zcl_vsp_adt_txt_format=>raise_not_acceptable( |Row type kind "{ kind }" is not known| ).
    ENDCASE.
  ENDMETHOD.


  METHOD type_kind_of.
    DATA typename TYPE typename.
    DATA typekind TYPE ddtypekind.

    typename = type_name.
    CALL FUNCTION 'DDIF_TYPEINFO_GET'
      EXPORTING
        typename = typename
      IMPORTING
        typekind = typekind.
    result = typekind.
  ENDMETHOD.


  METHOD codes.
    APPEND VALUE #( kind = 'A' code = 'T' name = `standard` ) TO result.
    APPEND VALUE #( kind = 'A' code = 'S' name = `sorted` ) TO result.
    APPEND VALUE #( kind = 'A' code = 'H' name = `hashed` ) TO result.
    APPEND VALUE #( kind = 'A' code = 'I' name = `index` ) TO result.
    APPEND VALUE #( kind = 'A' code = 'A' name = `notSpecified` ) TO result.
    APPEND VALUE #( kind = 'D' code = 'D' name = `standard` ) TO result.
    APPEND VALUE #( kind = 'D' code = 'T' name = `rowType` ) TO result.
    APPEND VALUE #( kind = 'D' code = 'K' name = `keyComponents` ) TO result.
    APPEND VALUE #( kind = 'D' code = 'G' name = `notSpecified` ) TO result.
    APPEND VALUE #( kind = 'K' code = 'U' name = `unique` ) TO result.
    APPEND VALUE #( kind = 'K' code = 'N' name = `nonUnique` ) TO result.
    APPEND VALUE #( kind = 'K' code = 'G' name = `notSpecified` ) TO result.
    APPEND VALUE #( kind = 'F' code = 'W' name = `allowed` ) TO result.
    APPEND VALUE #( kind = 'F' code = 'O' name = `notAllowed` ) TO result.
    APPEND VALUE #( kind = 'F' code = ' ' name = `notSpecified` ) TO result.
    APPEND VALUE #( kind = 'S' code = 'T' name = `rowType` ) TO result.
    APPEND VALUE #( kind = 'S' code = 'K' name = `keyComponents` ) TO result.
  ENDMETHOD.


  METHOD code_of.
    DATA all TYPE tt_codes.
    DATA entry TYPE ty_code.
    DATA allowed TYPE string.

    all = codes( ).
    LOOP AT all INTO entry WHERE kind = category.
      IF entry-name = value.
        result = entry-code.
        RETURN.
      ENDIF.
      IF allowed IS NOT INITIAL.
        allowed = allowed && ', '.
      ENDIF.
      allowed = allowed && entry-name.
    ENDLOOP.
    zcl_vsp_adt_txt_format=>raise_not_acceptable( |{ tag } "{ value }" is none of { allowed }| ).
  ENDMETHOD.


  METHOD name_of.
    DATA all TYPE tt_codes.
    DATA entry TYPE ty_code.

    all = codes( ).
    LOOP AT all INTO entry WHERE kind = category AND code = stored.
      result = entry-name.
      RETURN.
    ENDLOOP.
    IF category = 'S'.
      result = 'keyComponents'.
    ELSE.
      result = 'notSpecified'.
    ENDIF.
  ENDMETHOD.


  METHOD adt_data_type.
    CASE ddic.
      WHEN 'STRG'.
        result = 'STRING'.
      WHEN 'RSTR'.
        result = 'RAWSTRING'.
      WHEN 'SSTR'.
        result = 'SSTRING'.
      WHEN 'D16D'.
        result = 'DF16_DEC'.
      WHEN 'D16R'.
        result = 'DF16_RAW'.
      WHEN 'D16S'.
        result = 'DF16_SCL'.
      WHEN 'D34D'.
        result = 'DF34_DEC'.
      WHEN 'D34R'.
        result = 'DF34_RAW'.
      WHEN 'D34S'.
        result = 'DF34_SCL'.
      WHEN OTHERS.
        result = ddic.
    ENDCASE.
  ENDMETHOD.


  METHOD ddic_data_type.
    CASE adt.
      WHEN 'STRING'.
        result = 'STRG'.
      WHEN 'RAWSTRING'.
        result = 'RSTR'.
      WHEN 'SSTRING'.
        result = 'SSTR'.
      WHEN 'DF16_DEC'.
        result = 'D16D'.
      WHEN 'DF16_RAW'.
        result = 'D16R'.
      WHEN 'DF16_SCL'.
        result = 'D16S'.
      WHEN 'DF34_DEC'.
        result = 'D34D'.
      WHEN 'DF34_RAW'.
        result = 'D34R'.
      WHEN 'DF34_SCL'.
        result = 'D34S'.
      WHEN OTHERS.
        result = adt.
    ENDCASE.
  ENDMETHOD.


  METHOD components_xml.
    DATA key TYPE dd42v.

    LOOP AT keys INTO key WHERE seckeyname = seckeyname AND keyfield IS NOT INITIAL.
      result = |{ result }<ttyp:component ttyp:name="{ esc( key-keyfield ) }"/>|.
    ENDLOOP.
    IF result IS INITIAL.
      result = '<ttyp:components ttyp:isVisible="false"/>'.
    ELSE.
      result = |<ttyp:components ttyp:isVisible="true">{ result }</ttyp:components>|.
    ENDIF.
  ENDMETHOD.


  METHOD component_names.
    DATA list TYPE REF TO if_ixml_node_collection.
    DATA iterator TYPE REF TO if_ixml_node_iterator.
    DATA node TYPE REF TO if_ixml_node.
    DATA element TYPE REF TO if_ixml_element.
    DATA components TYPE REF TO if_ixml_element.
    DATA text TYPE string.

    components = child( parent = parent namespace = co_namespace tag = 'components' ).
    IF components IS NOT BOUND.
      RETURN.
    ENDIF.
    list = components->get_elements_by_tag_name_ns( name = 'component' uri = co_namespace ).
    iterator = list->create_iterator( ).
    node = iterator->get_next( ).
    WHILE node IS BOUND.
      element ?= node.
      text = to_upper( condense( element->get_attribute_ns( name = 'name' uri = co_namespace ) ) ).
      IF text IS NOT INITIAL.
        APPEND text TO result.
      ENDIF.
      node = iterator->get_next( ).
    ENDWHILE.
  ENDMETHOD.


  METHOD value_helps.
    " The lists the standard serves with every table type.
    result = '<ttyp:valueHelps>'.
    result = result && '<ttyp:typeKindValues>' &&
      help( key = `dictionaryType` value = `Dictionary Type` ) &&
      help( key = `predefinedAbapType` value = `Predefined Type` ) &&
      help( key = `refToPredefinedAbapType` value = `Reference to Predefined Type` ) &&
      help( key = `refToDictionaryType` value = `Reference to Dictionary Type` ) &&
      help( key = `refToClassOrInterfaceType` value = `Reference to Class/Interface` ) &&
      help( key = `rangeTypeOnPredefinedType` value = `Range Table on Predefined Type` ) &&
      help( key = `rangeTypeOnDataelement` value = `Range Table on Data Element` ) &&
      '</ttyp:typeKindValues>'.
    result = result && '<ttyp:keyDefinitionValues>' &&
      help( key = `standard` value = `Standard Key` ) &&
      help( key = `rowType` value = `Row Type` ) &&
      help( key = `keyComponents` value = `Key Components` ) &&
      help( key = `notSpecified` value = `Not Specified` ) &&
      '</ttyp:keyDefinitionValues>'.
    result = result && '<ttyp:keyKindValues>' &&
      help( key = `unique` value = `Unique` ) &&
      help( key = `nonUnique` value = `Non-Unique` ) &&
      help( key = `notSpecified` value = `Not Specified` ) &&
      '</ttyp:keyKindValues>'.
    result = result && '<ttyp:accessTypeValues>' &&
      help( key = `standard` value = `Standard Table` ) &&
      help( key = `sorted` value = `Sorted Table` ) &&
      help( key = `hashed` value = `Hashed Table` ) &&
      help( key = `index` value = `Index Table` ) &&
      help( key = `notSpecified` value = `Not Specified` ) &&
      '</ttyp:accessTypeValues>'.
    result = result && '<ttyp:secKeyAccessValues>' &&
      help( key = `uniqueHashed` value = `Unique Hashed` ) &&
      help( key = `uniqueSorted` value = `Unique Sorted` ) &&
      help( key = `nonUniqueSorted` value = `Non-Unique Sorted` ) &&
      '</ttyp:secKeyAccessValues>'.
    result = result && '<ttyp:secKeyDefinitionValues>' &&
      help( key = `rowType` value = `Row Type` ) &&
      help( key = `keyComponents` value = `Key Components` ) &&
      '</ttyp:secKeyDefinitionValues>'.
    result = result && '<ttyp:secKeyAllowedValues>' &&
      help( key = `allowed` value = `Allowed` ) &&
      help( key = `notAllowed` value = `Not Allowed` ) &&
      help( key = `notSpecified` value = `Not Specified` ) &&
      '</ttyp:secKeyAllowedValues>'.
    result = result && '</ttyp:valueHelps>'.
  ENDMETHOD.


  METHOD help.
    result = |<ttyp:valueHelp ttyp:key="{ key }" ttyp:value="{ esc( value ) }" ttyp:description=""/>|.
  ENDMETHOD.

ENDCLASS.
