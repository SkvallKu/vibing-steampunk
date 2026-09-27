"! <p class="shorttext synchronized">vsp ADT fallback: structure source, ours</p>
"! The structure source for a release without the standard's (7.40):
"! the "define type" language of 7.50, read and written in plain ABAP.
"! It does what 7.50's parser and generator do with a source, to the
"! same dictionary tables and back to the same text, so that a structure
"! reads and writes alike on both releases (checked against 7.50's with
"! ?parser=own). What the dictionary knows of a type (data element,
"! domain) it adds itself (DD_INT_DD03P_ACCUMULATE); whether a type or a
"! check table exists is the activation's question, not the parser's.
"!
"! Grammar, as 7.50 has it:
"! <ul>
"! <li>{@annotation} define type|structure NAME '{' {component} '}'
"!     or: extend type NAME with NAME '{' {component | extension} '}'</li>
"! <li>component: {@annotation} [key] FIELD : TYPE [not null]
"!     [with foreign key [CARD] TABLE where ...] [with value help ...] ;</li>
"! <li>include: [key] [GROUP :] include TYPE [with suffix S] [not null]
"!     {{@annotation} extend FIELD : additions or removals} ;</li>
"! <li>TYPE: NAME [boxed] | abap.T[(n[,m])] | reference to
"!     data|object|NAME|abap.T</li>
"! </ul>
CLASS zcl_vsp_ddic_src_own DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_ddic_source.

    CLASS-METHODS class_constructor.

  PROTECTED SECTION.
  PRIVATE SECTION.
    TYPES:
      BEGIN OF ty_token,
        kind  TYPE c LENGTH 1, " I identifier, N number, S string, E enum, O operator, Z end
        text  TYPE string,
        upper TYPE string,
        line  TYPE i,
      END OF ty_token,
      ty_tokens TYPE STANDARD TABLE OF ty_token WITH DEFAULT KEY,
      BEGIN OF ty_annotation,
        path  TYPE string, " canonical spelling
        value TYPE string, " as written, quotes and # taken off
        kind  TYPE c LENGTH 1,
        line  TYPE i,
      END OF ty_annotation,
      ty_annotations TYPE STANDARD TABLE OF ty_annotation WITH DEFAULT KEY,
      BEGIN OF ty_type,
        ddic      TYPE datatype_d,
        pattern   TYPE string,
        kind      TYPE c LENGTH 1, " F fixed, L length, D length and decimals
        len_min   TYPE i,
        len_max   TYPE i,
        dec_min   TYPE i,
        dec_max   TYPE i,
        fixed_len TYPE i,
      END OF ty_type,
      ty_types TYPE SORTED TABLE OF ty_type WITH UNIQUE KEY ddic,
      BEGIN OF ty_type_info,
        comptype TYPE comptype,
        datatype TYPE datatype_d,
        reftype  TYPE ddreftype,
        rollname TYPE rollname,
        leng     TYPE ddleng,
        decimals TYPE decimals,
      END OF ty_type_info,
      BEGIN OF ty_owner,
        fieldname TYPE fieldname,
        include   TYPE i, " index of the top-level include in the component list
      END OF ty_owner,
      ty_owners TYPE SORTED TABLE OF ty_owner WITH UNIQUE KEY fieldname.

    CONSTANTS:
      co_label        TYPE string VALUE 'EndUserText.label',
      co_enh_category TYPE string VALUE 'AbapCatalog.enhancementCategory',
      co_enh_cat_new  TYPE string VALUE 'AbapCatalog.enhancement.category',
      co_activation   TYPE string VALUE 'AbapCatalog.activationType',
      co_text_lang    TYPE string VALUE 'AbapCatalog.textLanguage',
      co_fk_label     TYPE string VALUE 'AbapCatalog.foreignKey.label',
      co_fk_screen    TYPE string VALUE 'AbapCatalog.foreignKey.screenCheck',
      co_fk_msgclass  TYPE string VALUE 'AbapCatalog.foreignKey.messageClass',
      co_fk_msgnr     TYPE string VALUE 'AbapCatalog.foreignKey.messageNumber',
      co_fk_keytype   TYPE string VALUE 'AbapCatalog.foreignKey.keyType',
      co_amount       TYPE string VALUE 'Semantics.amount.currencyCode',
      co_quantity     TYPE string VALUE 'Semantics.quantity.unitOfMeasure'.

    CLASS-DATA types TYPE ty_types.

    DATA tokens TYPE ty_tokens.
    DATA pos TYPE i.
    DATA object_name TYPE ddobjname.
    DATA struc_name TYPE string.
    DATA struc_line TYPE i.
    DATA header TYPE dd02v.
    DATA fields TYPE dd03ptab.
    DATA foreign_keys TYPE dd08vttyp.
    DATA fk_fields TYPE dd05mttyp.
    DATA value_helps TYPE dd35vttyp.
    DATA vh_fields TYPE dd36mttyp.

    CLASS-METHODS add_type
      IMPORTING ddic    TYPE datatype_d
                pattern TYPE string
                kind    TYPE c
                len_min TYPE i
                len_max TYPE i
                dec_min TYPE i DEFAULT 0
                dec_max TYPE i DEFAULT 0.

    " --- reading ---------------------------------------------------------
    METHODS fail
      IMPORTING text TYPE csequence
                line TYPE i OPTIONAL
      RAISING   cx_adt_res_not_acceptable.
    METHODS tokenize
      IMPORTING text TYPE string
      RAISING   cx_adt_res_not_acceptable.
    METHODS peek
      IMPORTING ahead         TYPE i DEFAULT 0
      RETURNING VALUE(result) TYPE ty_token.
    METHODS next
      RETURNING VALUE(result) TYPE ty_token.
    METHODS is_kw
      IMPORTING word          TYPE csequence
                ahead         TYPE i DEFAULT 0
      RETURNING VALUE(result) TYPE abap_bool.
    METHODS is_op
      IMPORTING op            TYPE csequence
                ahead         TYPE i DEFAULT 0
      RETURNING VALUE(result) TYPE abap_bool.
    METHODS expect_kw
      IMPORTING word TYPE csequence
      RAISING   cx_adt_res_not_acceptable.
    METHODS expect_op
      IMPORTING op TYPE csequence
      RAISING   cx_adt_res_not_acceptable.
    METHODS unexpected
      RAISING cx_adt_res_not_acceptable.
    METHODS read_name
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_annotation
      RETURNING VALUE(result) TYPE ty_annotation
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_annotations
      RETURNING VALUE(result) TYPE ty_annotations
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_structure
      RAISING cx_adt_res_not_acceptable.
    METHODS read_component
      IMPORTING in_append TYPE abap_bool
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_field
      IMPORTING annotations TYPE ty_annotations
                key         TYPE abap_bool
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_include
      IMPORTING key TYPE abap_bool
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_typing
      RETURNING VALUE(result) TYPE ty_type_info
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_predefined
      IMPORTING reference     TYPE abap_bool
      RETURNING VALUE(result) TYPE ty_type_info
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_additions
      IMPORTING fieldname   TYPE fieldname
                annotations TYPE ty_annotations
                comptype    TYPE comptype OPTIONAL
                noinherit   TYPE c OPTIONAL
                removals    TYPE abap_bool DEFAULT abap_false
                check_type  TYPE abap_bool DEFAULT abap_false
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_foreign_key
      IMPORTING fieldname   TYPE fieldname
                annotations TYPE ty_annotations
                noinherit   TYPE c
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_value_help
      IMPORTING fieldname TYPE fieldname
                shlpinher TYPE c
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_cardinality
      CHANGING  foreign_key TYPE dd08v
      RAISING   cx_adt_res_not_acceptable.
    METHODS read_reference
      EXPORTING table    TYPE string
                field    TYPE string
                const    TYPE string
                is_const TYPE abap_bool
      RAISING   cx_adt_res_not_acceptable.
    METHODS assimilate
      CHANGING  include TYPE dd03p
      RAISING   cx_adt_res_not_acceptable.
    METHODS reftype_of
      IMPORTING name          TYPE rollname
      RETURNING VALUE(result) TYPE ddreftype.
    METHODS check_length
      IMPORTING info TYPE ty_type_info
                line TYPE i
      RAISING   cx_adt_res_not_acceptable.
    METHODS adjusted_fk_fields
      RETURNING VALUE(result) TYPE dd05mttyp.
    CLASS-METHODS annotation_value
      IMPORTING annotations   TYPE ty_annotations
                path          TYPE string
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS has_annotation
      IMPORTING annotations   TYPE ty_annotations
                path          TYPE string
      RETURNING VALUE(result) TYPE abap_bool.

    " --- writing ---------------------------------------------------------
    CLASS-METHODS enh_category_of
      IMPORTING exclass       TYPE dd02v-exclass
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS pattern_of
      IMPORTING ddic          TYPE csequence
                leng          TYPE i
                decimals      TYPE i
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS escape
      IMPORTING text          TYPE csequence
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS spaces
      IMPORTING count         TYPE i
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS is_include_row
      IMPORTING field         TYPE dd03p
      RETURNING VALUE(result) TYPE abap_bool.
    CLASS-METHODS fk_annotations
      IMPORTING foreign_key   TYPE dd08v
                indent        TYPE string
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS where_clause
      IMPORTING fk_fields     TYPE dd05mttyp OPTIONAL
                vh_fields     TYPE dd36mttyp OPTIONAL
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS additions
      IMPORTING kind          TYPE c " C component, I include, A append
                foreign_keys  TYPE dd08vttyp
                fk_fields     TYPE dd05mttyp
                value_helps   TYPE dd35vttyp
                vh_fields     TYPE dd36mttyp
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS format_additions
      IMPORTING indent        TYPE i
                text          TYPE string
      RETURNING VALUE(result) TYPE string.
    CLASS-METHODS field_type_text
      IMPORTING field         TYPE dd03p
      RETURNING VALUE(result) TYPE string.
ENDCLASS.



CLASS zcl_vsp_ddic_src_own IMPLEMENTATION.

  METHOD class_constructor.
    add_type( ddic = 'ACCP' pattern = 'accp' kind = 'F' len_min = 6 len_max = 6 ).
    add_type( ddic = 'CHAR' pattern = 'char(len)' kind = 'L' len_min = 1 len_max = 30000 ).
    add_type( ddic = 'CLNT' pattern = 'clnt' kind = 'F' len_min = 3 len_max = 3 ).
    add_type( ddic = 'CUKY' pattern = 'cuky' kind = 'F' len_min = 5 len_max = 5 ).
    add_type( ddic = 'CURR' pattern = 'curr(len,decimals)' kind = 'D' len_min = 1 len_max = 31
              dec_min = 0 dec_max = 14 ).
    add_type( ddic = 'D16D' pattern = 'df16_dec(len,decimals)' kind = 'D' len_min = 1 len_max = 15
              dec_min = 0 dec_max = 14 ).
    add_type( ddic = 'D16R' pattern = 'df16_raw' kind = 'F' len_min = 16 len_max = 16 ).
    add_type( ddic = 'D16S' pattern = 'df16_scl' kind = 'F' len_min = 16 len_max = 16 ).
    add_type( ddic = 'D34D' pattern = 'df34_dec(len,decimals)' kind = 'D' len_min = 1 len_max = 31
              dec_min = 0 dec_max = 14 ).
    add_type( ddic = 'D34R' pattern = 'df34_raw' kind = 'F' len_min = 34 len_max = 34 ).
    add_type( ddic = 'D34S' pattern = 'df34_scl' kind = 'F' len_min = 34 len_max = 34 ).
    add_type( ddic = 'DATS' pattern = 'dats' kind = 'F' len_min = 8 len_max = 8 ).
    add_type( ddic = 'DEC' pattern = 'dec(len,decimals)' kind = 'D' len_min = 1 len_max = 31
              dec_min = 0 dec_max = 14 ).
    add_type( ddic = 'FLTP' pattern = 'fltp' kind = 'F' len_min = 16 len_max = 16 ).
    add_type( ddic = 'INT1' pattern = 'int1' kind = 'F' len_min = 3 len_max = 3 ).
    add_type( ddic = 'INT2' pattern = 'int2' kind = 'F' len_min = 5 len_max = 5 ).
    add_type( ddic = 'INT4' pattern = 'int4' kind = 'F' len_min = 10 len_max = 10 ).
    add_type( ddic = 'INT8' pattern = 'int8' kind = 'F' len_min = 19 len_max = 19 ).
    add_type( ddic = 'LANG' pattern = 'lang' kind = 'F' len_min = 1 len_max = 1 ).
    add_type( ddic = 'LCHR' pattern = 'lchr(len)' kind = 'L' len_min = 256 len_max = 32000 ).
    add_type( ddic = 'LRAW' pattern = 'lraw(len)' kind = 'L' len_min = 256 len_max = 32000 ).
    add_type( ddic = 'NUMC' pattern = 'numc(len)' kind = 'L' len_min = 1 len_max = 255 ).
    add_type( ddic = 'PREC' pattern = 'prec' kind = 'F' len_min = 2 len_max = 2 ).
    add_type( ddic = 'QUAN' pattern = 'quan(len,decimals)' kind = 'D' len_min = 1 len_max = 31
              dec_min = 0 dec_max = 14 ).
    add_type( ddic = 'RAW' pattern = 'raw(len)' kind = 'L' len_min = 1 len_max = 32000 ).
    add_type( ddic = 'RSTR' pattern = 'rawstring(len)' kind = 'L' len_min = 0 len_max = 2147483647 ).
    add_type( ddic = 'SSTR' pattern = 'sstring(len)' kind = 'L' len_min = 1 len_max = 1333 ).
    add_type( ddic = 'STRG' pattern = 'string(len)' kind = 'L' len_min = 0 len_max = 2147483647 ).
    add_type( ddic = 'TIMS' pattern = 'tims' kind = 'F' len_min = 6 len_max = 6 ).
    add_type( ddic = 'UNIT' pattern = 'unit(len)' kind = 'L' len_min = 2 len_max = 3 ).
  ENDMETHOD.


  METHOD add_type.
    DATA entry TYPE ty_type.
    entry-ddic = ddic.
    entry-pattern = pattern.
    entry-kind = kind.
    entry-len_min = len_min.
    entry-len_max = len_max.
    entry-dec_min = dec_min.
    entry-dec_max = dec_max.
    INSERT entry INTO TABLE types.
  ENDMETHOD.


  METHOD zif_vsp_ddic_source~parse.
    DATA text TYPE string.

    CLEAR: tokens, pos, struc_name, struc_line, header, fields, foreign_keys,
           fk_fields, value_helps, vh_fields.
    object_name = name.
    CONCATENATE LINES OF source INTO text SEPARATED BY cl_abap_char_utilities=>newline.
    tokenize( text ).
    read_structure( ).

    result-header = header.
    result-fields = fields.
    result-foreign_keys = foreign_keys.
    result-foreign_key_fields = adjusted_fk_fields( ).
    result-value_helps = value_helps.
    result-value_help_fields = vh_fields.
  ENDMETHOD.


  METHOD fail.
    IF line > 0.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( |Line { line }: { text }| ).
    ENDIF.
    zcl_vsp_adt_txt_format=>raise_not_acceptable( text ).
  ENDMETHOD.


  METHOD tokenize.
    DATA len TYPE i.
    DATA i TYPE i.
    DATA j TYPE i.
    DATA c TYPE c LENGTH 1.
    DATA d TYPE c LENGTH 1.
    DATA line TYPE i VALUE 1.
    DATA token TYPE ty_token.
    DATA word_chars TYPE string VALUE 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_'.
    DATA newline TYPE c LENGTH 1.
    DATA cr TYPE c LENGTH 1.
    DATA tab TYPE c LENGTH 1.
    DATA done TYPE abap_bool.

    newline = cl_abap_char_utilities=>newline.
    cr = cl_abap_char_utilities=>cr_lf.
    tab = cl_abap_char_utilities=>horizontal_tab.
    len = strlen( text ).
    WHILE i < len.
      c = text+i(1).
      IF i + 1 < len.
        j = i + 1.
        d = text+j(1).
      ELSE.
        CLEAR d.
      ENDIF.
      CLEAR token.
      token-line = line.

      IF c = newline.
        line = line + 1.
        i = i + 1.
        CONTINUE.
      ENDIF.
      " c against c: a blank compared with a string would be ''.
      IF c = space OR c = tab OR c = cr.
        i = i + 1.
        CONTINUE.
      ENDIF.
      " Comments: // and -- to the end of the line, /* ... */.
      IF ( c = '/' AND d = '/' ) OR ( c = '-' AND d = '-' ).
        WHILE i < len AND text+i(1) <> newline.
          i = i + 1.
        ENDWHILE.
        CONTINUE.
      ENDIF.
      IF c = '/' AND d = '*'.
        i = i + 2.
        done = abap_false.
        WHILE i < len AND done = abap_false.
          IF text+i(1) = newline.
            line = line + 1.
          ENDIF.
          j = i + 1.
          IF text+i(1) = '*' AND j < len AND text+j(1) = '/'.
            i = i + 2.
            done = abap_true.
          ELSE.
            i = i + 1.
          ENDIF.
        ENDWHILE.
        CONTINUE.
      ENDIF.

      IF c = `'`.
        " A string: '' is a quote inside it.
        token-kind = 'S'.
        i = i + 1.
        done = abap_false.
        WHILE done = abap_false.
          IF i >= len OR text+i(1) = newline.
            fail( text = 'Unterminated string' line = token-line ).
          ENDIF.
          j = i + 1.
          IF text+i(1) = `'`.
            IF j < len AND text+j(1) = `'`.
              token-text = token-text && `'`.
              i = i + 2.
            ELSE.
              i = i + 1.
              done = abap_true.
            ENDIF.
          ELSE.
            " substring( ) is a string: a blank in the text stays one.
            token-text = token-text && substring( val = text off = i len = 1 ).
            i = i + 1.
          ENDIF.
        ENDWHILE.
      ELSEIF c = '#'.
        token-kind = 'E'.
        i = i + 1.
        WHILE i < len.
          c = text+i(1).
          IF c = space OR word_chars NA c.
            EXIT.
          ENDIF.
          token-text = token-text && c.
          i = i + 1.
        ENDWHILE.
        IF token-text IS INITIAL.
          fail( text = 'Unexpected character "#"' line = token-line ).
        ENDIF.
      ELSEIF word_chars CA c.
        WHILE i < len.
          c = text+i(1).
          IF c = space OR word_chars NA c.
            EXIT.
          ENDIF.
          token-text = token-text && c.
          i = i + 1.
        ENDWHILE.
        IF token-text CO '0123456789'.
          token-kind = 'N'.
        ELSE.
          token-kind = 'I'.
        ENDIF.
      ELSEIF c = '.' AND d = '.'.
        token-kind = 'O'.
        token-text = '..'.
        i = i + 2.
      ELSEIF '.,:;@/(){}[]=*' CA c.
        token-kind = 'O'.
        token-text = c.
        i = i + 1.
      ELSE.
        fail( text = |Unexpected character "{ c }"| line = token-line ).
      ENDIF.
      token-upper = to_upper( token-text ).
      APPEND token TO tokens.
    ENDWHILE.

    CLEAR token.
    token-kind = 'Z'.
    token-line = line.
    APPEND token TO tokens.
    pos = 1.
  ENDMETHOD.


  METHOD peek.
    DATA index TYPE i.
    index = pos + ahead.
    READ TABLE tokens INDEX index INTO result.
    IF sy-subrc <> 0.
      READ TABLE tokens INDEX lines( tokens ) INTO result.
    ENDIF.
  ENDMETHOD.


  METHOD next.
    result = peek( ).
    IF result-kind <> 'Z'.
      pos = pos + 1.
    ENDIF.
  ENDMETHOD.


  METHOD is_kw.
    DATA token TYPE ty_token.
    token = peek( ahead ).
    result = boolc( token-kind = 'I' AND token-upper = word ).
  ENDMETHOD.


  METHOD is_op.
    DATA token TYPE ty_token.
    token = peek( ahead ).
    result = boolc( token-kind = 'O' AND token-text = op ).
  ENDMETHOD.


  METHOD expect_kw.
    IF is_kw( word ) = abap_false.
      unexpected( ).
    ENDIF.
    next( ).
  ENDMETHOD.


  METHOD expect_op.
    IF is_op( op ) = abap_false.
      unexpected( ).
    ENDIF.
    next( ).
  ENDMETHOD.


  METHOD unexpected.
    DATA token TYPE ty_token.
    token = peek( ).
    IF token-kind = 'Z'.
      fail( text = 'Unexpected end of source' line = token-line ).
    ENDIF.
    IF token-kind = 'S'.
      fail( text = |Unexpected token "'{ token-text }'"| line = token-line ).
    ENDIF.
    fail( text = |Unexpected token "{ token-text }"| line = token-line ).
  ENDMETHOD.


  METHOD read_name.
    " NAME or /NAMESPACE/NAME, upper case.
    DATA token TYPE ty_token.
    IF is_op( '/' ) = abap_true.
      next( ).
      token = peek( ).
      IF token-kind <> 'I' AND token-kind <> 'N'.
        unexpected( ).
      ENDIF.
      next( ).
      result = |/{ token-upper }/|.
      expect_op( '/' ).
      token = peek( ).
      IF token-kind <> 'I' AND token-kind <> 'N'.
        unexpected( ).
      ENDIF.
      next( ).
      result = result && token-upper.
      RETURN.
    ENDIF.
    token = peek( ).
    IF token-kind <> 'I' AND token-kind <> 'N'.
      unexpected( ).
    ENDIF.
    next( ).
    result = token-upper.
  ENDMETHOD.


  METHOD read_annotation.
    DATA token TYPE ty_token.
    DATA path TYPE string.
    DATA lower TYPE string.

    token = peek( ).
    result-line = token-line.
    expect_op( '@' ).
    token = peek( ).
    IF token-kind <> 'I'.
      unexpected( ).
    ENDIF.
    next( ).
    path = token-text.
    WHILE is_op( '.' ) = abap_true.
      next( ).
      token = peek( ).
      IF token-kind <> 'I'.
        unexpected( ).
      ENDIF.
      next( ).
      path = |{ path }.{ token-text }|.
    ENDWHILE.
    IF is_op( ':' ) = abap_true.
      next( ).
      token = peek( ).
      CASE token-kind.
        WHEN 'E' OR 'S' OR 'I' OR 'N'.
          result-kind = token-kind.
          result-value = token-text.
          next( ).
          " An identifier value may be a path: 'a.b'.
          WHILE token-kind = 'I' AND is_op( '.' ) = abap_true.
            next( ).
            token = peek( ).
            IF token-kind <> 'I'.
              unexpected( ).
            ENDIF.
            next( ).
            result-value = |{ result-value }.{ token-text }|.
          ENDWHILE.
        WHEN OTHERS.
          unexpected( ).
      ENDCASE.
    ENDIF.

    " The canonical spelling; 7.52 on writes enhancement.category.
    lower = to_lower( path ).
    IF lower = 'abapcatalog.enhancement.category'.
      lower = 'abapcatalog.enhancementcategory'.
    ENDIF.
    CASE lower.
      WHEN 'endusertext.label'.                    result-path = co_label.
      WHEN 'abapcatalog.enhancementcategory'.      result-path = co_enh_category.
      WHEN 'abapcatalog.activationtype'.           result-path = co_activation.
      WHEN 'abapcatalog.textlanguage'.             result-path = co_text_lang.
      WHEN 'abapcatalog.foreignkey.label'.         result-path = co_fk_label.
      WHEN 'abapcatalog.foreignkey.screencheck'.   result-path = co_fk_screen.
      WHEN 'abapcatalog.foreignkey.messageclass'.  result-path = co_fk_msgclass.
      WHEN 'abapcatalog.foreignkey.messagenumber'. result-path = co_fk_msgnr.
      WHEN 'abapcatalog.foreignkey.keytype'.       result-path = co_fk_keytype.
      WHEN 'semantics.amount.currencycode'.        result-path = co_amount.
      WHEN 'semantics.quantity.unitofmeasure'.     result-path = co_quantity.
      WHEN OTHERS.
        " Unknown to the dictionary: 7.50 warns and ignores it.
        result-path = path.
    ENDCASE.
  ENDMETHOD.


  METHOD read_annotations.
    WHILE is_op( '@' ) = abap_true.
      APPEND read_annotation( ) TO result.
    ENDWHILE.
  ENDMETHOD.


  METHOD annotation_value.
    DATA annotation TYPE ty_annotation.
    READ TABLE annotations INTO annotation WITH KEY path = path.
    IF sy-subrc = 0.
      result = annotation-value.
    ENDIF.
  ENDMETHOD.


  METHOD has_annotation.
    READ TABLE annotations TRANSPORTING NO FIELDS WITH KEY path = path.
    result = boolc( sy-subrc = 0 ).
  ENDMETHOD.


  METHOD read_structure.
    DATA annotations TYPE ty_annotations.
    DATA annotation TYPE ty_annotation.
    DATA extended TYPE string.
    DATA is_append TYPE abap_bool.
    DATA token TYPE ty_token.
    DATA value TYPE string.

    annotations = read_annotations( ).

    IF is_kw( 'EXTEND' ) = abap_true.
      next( ).
      expect_kw( 'TYPE' ).
      extended = read_name( ).
      expect_kw( 'WITH' ).
      is_append = abap_true.
    ELSE.
      IF is_kw( 'DEFINE' ) = abap_true.
        next( ).
      ENDIF.
      IF is_kw( 'TYPE' ) = abap_true OR is_kw( 'STRUCTURE' ) = abap_true.
        next( ).
      ELSE.
        unexpected( ).
      ENDIF.
    ENDIF.
    token = peek( ).
    struc_line = token-line.
    struc_name = read_name( ).
    expect_op( '{' ).

    IF is_append = abap_false AND is_op( '}' ) = abap_true.
      unexpected( ).
    ENDIF.
    WHILE is_op( '}' ) = abap_false.
      token = peek( ).
      IF token-kind = 'Z'.
        unexpected( ).
      ENDIF.
      read_component( is_append ).
    ENDWHILE.
    expect_op( '}' ).
    IF is_op( ';' ) = abap_true.
      next( ).
    ENDIF.
    token = peek( ).
    IF token-kind <> 'Z'.
      unexpected( ).
    ENDIF.

    IF strlen( struc_name ) > 30.
      fail( text = |The name { struc_name } is longer than 30 characters| line = struc_line ).
    ENDIF.
    IF struc_name <> object_name.
      fail( text = |The source defines { struc_name }, not { object_name }| line = struc_line ).
    ENDIF.
    IF has_annotation( annotations = annotations path = co_enh_category ) = abap_false.
      fail( |Mandatory annotation "{ co_enh_category }" for structure { struc_name } is missing| ).
    ENDIF.
    IF has_annotation( annotations = annotations path = co_label ) = abap_false.
      fail( |Mandatory annotation "{ co_label }" for structure { struc_name } is missing| ).
    ENDIF.

    header-tabname = struc_name.
    IF is_append = abap_true.
      header-tabclass = 'APPEND'.
      header-sqltab = extended.
    ELSE.
      header-tabclass = 'INTTAB'.
    ENDIF.
    READ TABLE annotations INTO annotation WITH KEY path = co_enh_category.
    value = to_upper( annotation-value ).
    CASE value.
      WHEN 'NOT_CLASSIFIED'.               header-exclass = '0'.
      WHEN 'NOT_EXTENSIBLE'.               header-exclass = '1'.
      WHEN 'EXTENSIBLE_CHARACTER'.         header-exclass = '2'.
      WHEN 'EXTENSIBLE_CHARACTER_NUMERIC'. header-exclass = '3'.
      WHEN 'EXTENSIBLE_ANY'.               header-exclass = '4'.
      WHEN OTHERS.
        fail( text = |Invalid value #{ annotation-value } for { co_enh_category }| line = annotation-line ).
    ENDCASE.
    header-ddtext = annotation_value( annotations = annotations path = co_label ).
  ENDMETHOD.


  METHOD read_component.
    DATA annotations TYPE ty_annotations.
    DATA key TYPE abap_bool.
    DATA fieldname TYPE fieldname.
    DATA saved TYPE i.
    DATA token TYPE ty_token.

    annotations = read_annotations( ).

    IF is_kw( 'EXTEND' ) = abap_true.
      " An append's extension of a field of the structure it extends.
      IF in_append = abap_false.
        unexpected( ).
      ENDIF.
      next( ).
      fieldname = read_name( ).
      expect_op( ':' ).
      read_additions( fieldname   = fieldname
                      annotations = annotations
                      noinherit   = 'E' ).
      expect_op( ';' ).
      RETURN.
    ENDIF.

    IF is_kw( 'KEY' ) = abap_true.
      next( ).
      key = abap_true.
    ENDIF.

    " An include, with or without a group: [GROUP :] include ...
    IF is_kw( 'INCLUDE' ) = abap_true AND is_op( op = ':' ahead = 1 ) = abap_false.
      IF annotations IS NOT INITIAL.
        token = peek( ).
        fail( text = 'An include takes no annotations' line = token-line ).
      ENDIF.
      read_include( key ).
      RETURN.
    ENDIF.
    saved = pos.
    read_name( ).
    IF is_op( ':' ) = abap_true AND is_kw( word = 'INCLUDE' ahead = 1 ) = abap_true.
      pos = saved.
      IF annotations IS NOT INITIAL.
        token = peek( ).
        fail( text = 'An include takes no annotations' line = token-line ).
      ENDIF.
      read_include( key ).
      RETURN.
    ENDIF.
    pos = saved.
    read_field( annotations = annotations key = key ).
  ENDMETHOD.


  METHOD read_field.
    DATA field TYPE dd03p.
    DATA info TYPE ty_type_info.
    DATA token TYPE ty_token.
    DATA accumulate TYPE dd03ptab.
    DATA reference TYPE string.
    DATA line TYPE i.

    token = peek( ).
    line = token-line.
    field-tabname = struc_name.
    field-adminfield = '0'.
    field-position = lines( fields ) + 1.
    field-ddlanguage = sy-langu.
    field-keyflag = key.
    field-fieldname = read_name( ).
    IF strlen( field-fieldname ) > 30.
      fail( text = |The field name { field-fieldname } is longer than 30 characters| line = line ).
    ENDIF.
    expect_op( ':' ).
    token = peek( ).
    info = read_typing( ).
    check_length( info = info line = token-line ).
    field-comptype = info-comptype.
    field-datatype = info-datatype.
    field-reftype = info-reftype.
    field-rollname = info-rollname.
    field-leng = info-leng.
    field-decimals = info-decimals.
    IF is_kw( 'NOT' ) = abap_true AND is_kw( word = 'NULL' ahead = 1 ) = abap_true.
      next( ).
      next( ).
      field-notnull = abap_true.
    ENDIF.

    " What the dictionary knows of the type, as the editor adds it.
    APPEND field TO accumulate.
    CALL FUNCTION 'DD_INT_DD03P_ACCUMULATE'
      EXPORTING
        all_fields    = abap_true
        get_checkshlp = abap_true
        read_doma     = abap_true
      TABLES
        dd03p_tab     = accumulate
      EXCEPTIONS
        OTHERS        = 4.
    IF sy-subrc = 0 AND accumulate IS NOT INITIAL.
      READ TABLE accumulate INDEX 1 INTO field.
    ENDIF.

    " A short text only where no data element gives one.
    IF ( field-rollname IS INITIAL AND field-precfield IS INITIAL )
        OR ( field-comptype = 'R' AND field-reftype = 'B' ).
      field-ddtext = annotation_value( annotations = annotations path = co_label ).
      field-ddlanguage = sy-langu.
    ENDIF.
    IF field-datatype = 'CURR'.
      reference = to_upper( annotation_value( annotations = annotations path = co_amount ) ).
      SPLIT reference AT '.' INTO field-reftable field-reffield.
    ENDIF.
    IF field-datatype = 'QUAN'.
      reference = to_upper( annotation_value( annotations = annotations path = co_quantity ) ).
      SPLIT reference AT '.' INTO field-reftable field-reffield.
    ENDIF.
    IF field-datatype = 'LANG' AND has_annotation( annotations = annotations path = co_text_lang ) = abap_true.
      field-languflag = abap_true.
    ENDIF.
    APPEND field TO fields.

    read_additions( fieldname   = field-fieldname
                    annotations = annotations
                    comptype    = field-comptype
                    check_type  = abap_true ).
    expect_op( ';' ).
  ENDMETHOD.


  METHOD read_include.
    DATA field TYPE dd03p.
    DATA annotations TYPE ty_annotations.
    DATA fieldname TYPE fieldname.
    DATA token TYPE ty_token.
    DATA saved TYPE i.

    field-tabname = struc_name.
    field-adminfield = '0'.
    field-fieldname = '.INCLUDE'.
    field-position = lines( fields ) + 1.
    field-comptype = 'S'.
    field-keyflag = key.

    IF is_kw( 'INCLUDE' ) = abap_false.
      field-groupname = read_name( ).
      expect_op( ':' ).
    ENDIF.
    expect_kw( 'INCLUDE' ).
    field-precfield = read_name( ).
    IF is_kw( 'WITH' ) = abap_true AND is_kw( word = 'SUFFIX' ahead = 1 ) = abap_true.
      next( ).
      next( ).
      token = peek( ).
      IF token-kind <> 'I' AND token-kind <> 'N'.
        unexpected( ).
      ENDIF.
      next( ).
      field-fieldname = |.INCLU-{ token-upper }|.
    ENDIF.
    IF is_kw( 'NOT' ) = abap_true AND is_kw( word = 'NULL' ahead = 1 ) = abap_true.
      next( ).
      next( ).
      field-notnull = abap_true.
    ENDIF.

    " Foreign keys and value helps set or removed for included fields.
    DO.
      saved = pos.
      annotations = read_annotations( ).
      IF is_kw( 'EXTEND' ) = abap_false.
        IF annotations IS NOT INITIAL.
          unexpected( ).
        ENDIF.
        pos = saved.
        EXIT.
      ENDIF.
      " 7.50 takes "not null" only right before the ';': with extensions
      " after it, the include is read as nullable. So here too.
      field-notnull = abap_false.
      next( ).
      fieldname = read_name( ).
      expect_op( ':' ).
      read_additions( fieldname   = fieldname
                      annotations = annotations
                      noinherit   = 'Y'
                      removals    = abap_true ).
    ENDDO.
    expect_op( ';' ).

    assimilate( CHANGING include = field ).
  ENDMETHOD.


  METHOD assimilate.
    " The included structure's fields follow its include row, as the
    " editor lays them out; its foreign keys and value helps come along
    " unless the including structure sets its own for the field.
    DATA include_header TYPE dd02v.
    DATA inc_fields TYPE dd03ptab.
    DATA inc_fk_fields TYPE dd05mttyp.
    DATA inc_fks TYPE dd08vttyp.
    DATA inc_vhs TYPE dd35vttyp.
    DATA inc_vh_fields TYPE dd36mttyp.
    DATA suffix TYPE string.
    DATA position TYPE i.
    DATA top TYPE dd03p.
    FIELD-SYMBOLS <field> TYPE dd03p.
    FIELD-SYMBOLS <fk> TYPE dd08v.
    FIELD-SYMBOLS <fk_field> TYPE dd05m.
    FIELD-SYMBOLS <vh> TYPE dd35v.
    FIELD-SYMBOLS <vh_field> TYPE dd36m.

    IF include-fieldname(7) = '.INCLU-'.
      suffix = include-fieldname+7.
    ENDIF.
    APPEND include TO fields.
    position = lines( fields ) + 1.

    CALL FUNCTION 'DDIF_TABL_GET'
      EXPORTING
        name          = include-precfield
        langu         = sy-langu
      IMPORTING
        dd02v_wa      = include_header
      TABLES
        dd03p_tab     = inc_fields
        dd05m_tab     = inc_fk_fields
        dd08v_tab     = inc_fks
        dd35v_tab     = inc_vhs
        dd36m_tab     = inc_vh_fields
      EXCEPTIONS
        OTHERS        = 1.
    IF sy-subrc <> 0 OR include_header IS INITIAL.
      " As 7.50: the activation says the include is not there.
      RETURN.
    ENDIF.
    READ TABLE fields ASSIGNING <field> INDEX lines( fields ).
    <field>-ddlanguage = include_header-ddlanguage.
    <field>-ddtext = include_header-ddtext.
    include = <field>.

    " Foreign keys: a field directly in the include takes the suffix.
    LOOP AT inc_fks ASSIGNING <fk>.
      READ TABLE inc_fields INTO top WITH KEY tabname = <fk>-tabname fieldname = <fk>-fieldname.
      IF suffix IS NOT INITIAL AND sy-subrc = 0 AND top-adminfield = 0.
        <fk>-fieldname = <fk>-fieldname && suffix.
      ENDIF.
      <fk>-tabname = struc_name.
      <fk>-noinherit = 'N'.
    ENDLOOP.
    LOOP AT inc_fk_fields ASSIGNING <fk_field>.
      READ TABLE inc_fields INTO top WITH KEY tabname = <fk_field>-tabname fieldname = <fk_field>-fieldname.
      IF suffix IS NOT INITIAL AND sy-subrc = 0 AND top-adminfield = 0.
        <fk_field>-fieldname = <fk_field>-fieldname && suffix.
      ENDIF.
      IF <fk_field>-tabname = <fk_field>-fortable.
        READ TABLE inc_fields INTO top WITH KEY tabname = <fk_field>-fortable fieldname = <fk_field>-forkey.
        IF suffix IS NOT INITIAL AND sy-subrc = 0 AND top-adminfield = 0.
          <fk_field>-forkey = <fk_field>-forkey && suffix.
        ENDIF.
        <fk_field>-fortable = struc_name.
      ENDIF.
      <fk_field>-tabname = struc_name.
    ENDLOOP.

    " Value helps, the same way.
    LOOP AT inc_vhs ASSIGNING <vh>.
      READ TABLE inc_fields INTO top WITH KEY tabname = <vh>-tabname fieldname = <vh>-fieldname.
      IF suffix IS NOT INITIAL AND sy-subrc = 0 AND top-adminfield = 0.
        <vh>-fieldname = <vh>-fieldname && suffix.
      ENDIF.
      <vh>-tabname = struc_name.
      <vh>-shlpinher = abap_true.
    ENDLOOP.
    LOOP AT inc_vh_fields ASSIGNING <vh_field>.
      READ TABLE inc_fields INTO top WITH KEY tabname = <vh_field>-tabname fieldname = <vh_field>-fieldname.
      IF suffix IS NOT INITIAL AND sy-subrc = 0 AND top-adminfield = 0.
        <vh_field>-fieldname = <vh_field>-fieldname && suffix.
      ENDIF.
      IF <vh_field>-shtable = <vh_field>-tabname.
        READ TABLE inc_fields INTO top WITH KEY tabname = <vh_field>-shtable fieldname = <vh_field>-shfield.
        IF suffix IS NOT INITIAL AND sy-subrc = 0 AND top-adminfield = 0.
          <vh_field>-shfield = <vh_field>-shfield && suffix.
        ENDIF.
        <vh_field>-shtable = struc_name.
      ENDIF.
      <vh_field>-tabname = struc_name.
    ENDLOOP.

    " The fields: numbered on, key if the include is, never not null.
    LOOP AT inc_fields ASSIGNING <field>.
      <field>-tabname = struc_name.
      <field>-position = position.
      <field>-keyflag = include-keyflag.
      <field>-notnull = abap_false.
      <field>-adminfield = <field>-adminfield + 1.
      IF <field>-adminfield = 1.
        <field>-fieldname = <field>-fieldname && suffix.
      ENDIF.
      IF <field>-reftable = include-precfield.
        <field>-reftable = struc_name.
      ENDIF.
      position = position + 1.
    ENDLOOP.
    APPEND LINES OF inc_fields TO fields.

    " What the including structure sets itself wins.
    LOOP AT inc_fks ASSIGNING <fk>.
      READ TABLE foreign_keys TRANSPORTING NO FIELDS
        WITH KEY tabname = <fk>-tabname fieldname = <fk>-fieldname.
      IF sy-subrc = 0.
        DELETE inc_fk_fields WHERE tabname = <fk>-tabname AND fieldname = <fk>-fieldname.
        DELETE inc_fks.
      ENDIF.
    ENDLOOP.
    LOOP AT inc_vhs ASSIGNING <vh>.
      READ TABLE value_helps TRANSPORTING NO FIELDS
        WITH KEY tabname = <vh>-tabname fieldname = <vh>-fieldname.
      IF sy-subrc = 0.
        DELETE inc_vh_fields WHERE tabname = <vh>-tabname AND fieldname = <vh>-fieldname.
        DELETE inc_vhs.
      ENDIF.
    ENDLOOP.
    APPEND LINES OF inc_vhs TO value_helps.
    APPEND LINES OF inc_vh_fields TO vh_fields.
    APPEND LINES OF inc_fks TO foreign_keys.
    APPEND LINES OF inc_fk_fields TO fk_fields.
  ENDMETHOD.


  METHOD read_typing.
    DATA name TYPE string.

    IF is_kw( 'REFERENCE' ) = abap_true AND is_kw( word = 'TO' ahead = 1 ) = abap_true.
      next( ).
      next( ).
      IF is_kw( 'ABAP' ) = abap_true AND is_op( op = '.' ahead = 1 ) = abap_true.
        result = read_predefined( abap_true ).
        RETURN.
      ENDIF.
      IF is_kw( 'DATA' ) = abap_true AND is_op( op = ';' ahead = 1 ) = abap_true
          OR is_kw( 'DATA' ) = abap_true AND is_kw( word = 'NOT' ahead = 1 ) = abap_true
          OR is_kw( 'DATA' ) = abap_true AND is_kw( word = 'WITH' ahead = 1 ) = abap_true.
        next( ).
        result-rollname = 'DATA'.
      ELSEIF is_kw( 'OBJECT' ) = abap_true AND is_op( op = ';' ahead = 1 ) = abap_true
          OR is_kw( 'OBJECT' ) = abap_true AND is_kw( word = 'NOT' ahead = 1 ) = abap_true
          OR is_kw( 'OBJECT' ) = abap_true AND is_kw( word = 'WITH' ahead = 1 ) = abap_true.
        next( ).
        result-rollname = 'OBJECT'.
      ELSE.
        result-rollname = read_name( ).
      ENDIF.
      result-datatype = 'REF'.
      result-comptype = 'R'.
      result-reftype = reftype_of( result-rollname ).
      RETURN.
    ENDIF.

    IF is_kw( 'ABAP' ) = abap_true AND is_op( op = '.' ahead = 1 ) = abap_true.
      result = read_predefined( abap_false ).
      RETURN.
    ENDIF.

    name = read_name( ).
    result-rollname = name.
    IF is_kw( 'BOXED' ) = abap_true.
      next( ).
      result-comptype = 'R'.
      result-datatype = 'REF'.
      result-reftype = 'J'.
    ENDIF.
  ENDMETHOD.


  METHOD read_predefined.
    DATA token TYPE ty_token.
    DATA ddic TYPE string.
    DATA type TYPE ty_type.

    next( ). " abap
    next( ). " .
    token = peek( ).
    IF token-kind <> 'I'.
      unexpected( ).
    ENDIF.
    next( ).
    CASE token-upper.
      WHEN 'STRING'.                  ddic = 'STRG'.
      WHEN 'SSTRING'.                 ddic = 'SSTR'.
      WHEN 'RAWSTRING'.               ddic = 'RSTR'.
      WHEN 'DF16_DEC' OR 'DECFLOAT16'. ddic = 'D16D'.
      WHEN 'DF16_RAW'.                ddic = 'D16R'.
      WHEN 'DF16_SCL'.                ddic = 'D16S'.
      WHEN 'DF34_DEC' OR 'DECFLOAT34'. ddic = 'D34D'.
      WHEN 'DF34_RAW'.                ddic = 'D34R'.
      WHEN 'DF34_SCL'.                ddic = 'D34S'.
      WHEN OTHERS.                    ddic = token-upper.
    ENDCASE.
    READ TABLE types TRANSPORTING NO FIELDS WITH TABLE KEY ddic = ddic.
    IF sy-subrc <> 0.
      fail( text = |Unknown predefined type abap.{ token-text }| line = token-line ).
    ENDIF.
    IF reference = abap_true.
      result-rollname = ddic.
      result-reftype = 'B'.
      result-datatype = 'REF'.
      result-comptype = 'R'.
    ELSE.
      result-datatype = ddic.
    ENDIF.
    IF is_op( '(' ) = abap_true.
      next( ).
      token = peek( ).
      IF token-kind <> 'N'.
        unexpected( ).
      ENDIF.
      next( ).
      result-leng = token-text.
      IF is_op( ',' ) = abap_true.
        next( ).
        token = peek( ).
        IF token-kind <> 'N'.
          unexpected( ).
        ENDIF.
        next( ).
        result-decimals = token-text.
      ENDIF.
      expect_op( ')' ).
    ENDIF.
    " A fixed type has its length (and FLTP its decimals) as SE11 sets it.
    IF reference = abap_false.
      READ TABLE types INTO type WITH TABLE KEY ddic = ddic.
      IF type-kind = 'F' AND result-leng IS INITIAL.
        result-leng = type-len_min.
        IF ddic = 'FLTP'.
          result-decimals = 16.
        ENDIF.
      ENDIF.
    ENDIF.
  ENDMETHOD.


  METHOD check_length.
    " What SE11 refuses for a predefined type: a length or decimals out
    " of range.
    DATA type TYPE ty_type.
    DATA leng TYPE i.
    DATA decimals TYPE i.

    IF info-datatype IS INITIAL OR info-datatype = 'REF'.
      RETURN.
    ENDIF.
    READ TABLE types INTO type WITH TABLE KEY ddic = info-datatype.
    IF sy-subrc <> 0 OR type-kind = 'F'.
      RETURN.
    ENDIF.
    leng = info-leng.
    decimals = info-decimals.
    IF leng < type-len_min OR leng > type-len_max.
      fail( text = |Length { leng } of abap.{ to_lower( info-datatype ) } is outside { type-len_min } to { type-len_max }|
            line = line ).
    ENDIF.
    IF type-kind = 'D' AND ( decimals < type-dec_min OR decimals > type-dec_max OR decimals > leng ).
      fail( text = |Decimals { decimals } of abap.{ to_lower( info-datatype ) } are not allowed with length { leng }|
            line = line ).
    ENDIF.
  ENDMETHOD.


  METHOD reftype_of.
    DATA typekind TYPE ddtypekind.

    CASE name.
      WHEN 'DATA'.
        result = 'D'.
      WHEN 'OBJECT'.
        result = 'O'.
      WHEN OTHERS.
        CALL FUNCTION 'DDIF_TYPEINFO_GET'
          EXPORTING
            typename = name
          IMPORTING
            typekind = typekind.
        CASE typekind.
          WHEN 'CLAS'. result = 'C'.
          WHEN 'INTF'. result = 'I'.
          WHEN 'DTEL'. result = 'E'.
          WHEN 'TTYP'. result = 'L'.
          WHEN 'TABL'. result = 'S'.
          WHEN OTHERS. CLEAR result.
        ENDCASE.
    ENDCASE.
  ENDMETHOD.


  METHOD read_additions.
    DATA had_fk TYPE abap_bool.
    DATA had_vh TYPE abap_bool.
    DATA token TYPE ty_token.
    DATA shlpinher TYPE c LENGTH 1.
    DATA foreign_key TYPE dd08v.
    DATA value_help TYPE dd35v.

    IF noinherit = 'E'.
      shlpinher = 'E'.
    ENDIF.
    DO.
      token = peek( ).
      IF is_kw( 'WITH' ) = abap_true AND is_kw( word = 'FOREIGN' ahead = 1 ) = abap_true AND had_fk = abap_false.
        IF check_type = abap_true AND comptype <> 'E'.
          fail( text = 'A foreign key needs a field typed with a data element' line = token-line ).
        ENDIF.
        read_foreign_key( fieldname = fieldname annotations = annotations noinherit = noinherit ).
        had_fk = abap_true.
      ELSEIF is_kw( 'WITH' ) = abap_true AND is_kw( word = 'VALUE' ahead = 1 ) = abap_true AND had_vh = abap_false.
        IF check_type = abap_true AND comptype <> 'E'.
          fail( text = 'A value help needs a field typed with a data element' line = token-line ).
        ENDIF.
        read_value_help( fieldname = fieldname shlpinher = shlpinher ).
        had_vh = abap_true.
      ELSEIF removals = abap_true AND is_kw( 'REMOVE' ) = abap_true
          AND is_kw( word = 'FOREIGN' ahead = 1 ) = abap_true AND had_fk = abap_false.
        next( ).
        next( ).
        expect_kw( 'KEY' ).
        CLEAR foreign_key.
        foreign_key-tabname = struc_name.
        foreign_key-fieldname = fieldname.
        foreign_key-noinherit = noinherit.
        foreign_key-checktable = '*'.
        APPEND foreign_key TO foreign_keys.
        had_fk = abap_true.
      ELSEIF removals = abap_true AND is_kw( 'REMOVE' ) = abap_true
          AND is_kw( word = 'VALUE' ahead = 1 ) = abap_true AND had_vh = abap_false.
        next( ).
        next( ).
        expect_kw( 'HELP' ).
        CLEAR value_help.
        value_help-tabname = struc_name.
        value_help-fieldname = fieldname.
        value_help-shlpname = '*'.
        APPEND value_help TO value_helps.
        had_vh = abap_true.
      ELSE.
        EXIT.
      ENDIF.
    ENDDO.
  ENDMETHOD.


  METHOD read_foreign_key.
    DATA foreign_key TYPE dd08v.
    DATA fk_field TYPE dd05m.
    DATA keys TYPE dd05mttyp.
    DATA table TYPE string.
    DATA field TYPE string.
    DATA const TYPE string.
    DATA is_const TYPE abap_bool.
    DATA token TYPE ty_token.
    DATA screen TYPE string.
    DATA value TYPE string.
    DATA annotation TYPE ty_annotation.

    foreign_key-tabname = struc_name.
    foreign_key-fieldname = fieldname.
    foreign_key-noinherit = noinherit.

    expect_kw( 'WITH' ).
    expect_kw( 'FOREIGN' ).
    expect_kw( 'KEY' ).
    IF is_op( '[' ) = abap_true.
      read_cardinality( CHANGING foreign_key = foreign_key ).
    ENDIF.
    foreign_key-checktable = read_name( ).
    expect_kw( 'WHERE' ).
    DO.
      CLEAR fk_field.
      fk_field-tabname = struc_name.
      fk_field-fieldname = fieldname.
      fk_field-checktable = foreign_key-checktable.
      token = peek( ).
      fk_field-checkfield = read_name( ).
      expect_op( '=' ).
      read_reference( IMPORTING table = table field = field const = const is_const = is_const ).
      IF is_const = abap_true.
        fk_field-fortable = |'{ to_upper( const ) }'|.
      ELSE.
        IF table IS INITIAL.
          fk_field-fortable = struc_name.
        ELSE.
          fk_field-fortable = table.
        ENDIF.
        fk_field-forkey = field.
      ENDIF.
      READ TABLE keys TRANSPORTING NO FIELDS
        WITH KEY checktable = fk_field-checktable checkfield = fk_field-checkfield.
      IF sy-subrc <> 0.
        fk_field-primpos = lines( keys ) + 1.
        APPEND fk_field TO keys.
      ENDIF.
      IF is_kw( 'AND' ) = abap_false.
        EXIT.
      ENDIF.
      next( ).
    ENDDO.

    " The key type, text, screen check and message.
    READ TABLE annotations INTO annotation WITH KEY path = co_fk_keytype.
    IF sy-subrc = 0.
      value = to_upper( annotation-value ).
      CASE value.
        WHEN 'KEY'.      foreign_key-frkart = 'KEY'.
        WHEN 'NON_KEY'.  foreign_key-frkart = 'REF'.
        WHEN 'TEXT_KEY'. foreign_key-frkart = 'TEXT'.
        WHEN OTHERS.
          fail( text = |Invalid value #{ annotation-value } for { co_fk_keytype }| line = annotation-line ).
      ENDCASE.
    ENDIF.
    foreign_key-ddtext = annotation_value( annotations = annotations path = co_fk_label ).
    foreign_key-ddlanguage = sy-langu.
    screen = to_lower( annotation_value( annotations = annotations path = co_fk_screen ) ).
    CASE screen.
      WHEN 'true'.
        foreign_key-checkflag = abap_false.
      WHEN OTHERS.
        foreign_key-checkflag = abap_true.
    ENDCASE.
    foreign_key-arbgb = to_upper( annotation_value( annotations = annotations path = co_fk_msgclass ) ).
    foreign_key-msgnr = annotation_value( annotations = annotations path = co_fk_msgnr ).
    IF ( foreign_key-arbgb IS INITIAL AND foreign_key-msgnr IS NOT INITIAL )
        OR ( foreign_key-arbgb IS NOT INITIAL AND foreign_key-msgnr IS INITIAL ).
      fail( text = |The foreign key of { fieldname } needs both a message class and a message number|
            line = token-line ).
    ENDIF.

    APPEND LINES OF keys TO fk_fields.
    APPEND foreign_key TO foreign_keys.
  ENDMETHOD.


  METHOD read_cardinality.
    DATA token TYPE ty_token.
    DATA first TYPE string.
    DATA last TYPE string.

    expect_op( '[' ).
    token = peek( ).
    IF token-kind <> 'N'.
      unexpected( ).
    ENDIF.
    next( ).
    first = token-text.
    IF is_op( '..' ) = abap_true.
      next( ).
      token = peek( ).
      IF token-kind = 'N' OR is_op( '*' ) = abap_true.
        next( ).
      ELSE.
        unexpected( ).
      ENDIF.
      last = token-text.
      CASE |{ first }..{ last }|.
        WHEN '0..1'. foreign_key-card = 'C'.
        WHEN '0..*'. foreign_key-card = 'CN'.
        WHEN '1..*'. foreign_key-card = 'N'.
        WHEN OTHERS.
          fail( text = |Invalid cardinality { first }..{ last }| line = token-line ).
      ENDCASE.
    ELSEIF first = '1'.
      foreign_key-card = '1'.
    ELSE.
      fail( text = |Invalid cardinality { first }| line = token-line ).
    ENDIF.
    expect_op( ',' ).
    token = peek( ).
    IF token-kind <> 'N'.
      unexpected( ).
    ENDIF.
    next( ).
    first = token-text.
    IF is_op( '..' ) = abap_true.
      next( ).
      token = peek( ).
      IF token-kind <> 'N'.
        unexpected( ).
      ENDIF.
      next( ).
      IF first = '0' AND token-text = '1'.
        foreign_key-cardleft = 'C'.
      ELSE.
        fail( text = |Invalid cardinality { first }..{ token-text }| line = token-line ).
      ENDIF.
    ELSEIF first = '1'.
      foreign_key-cardleft = '1'.
    ELSE.
      fail( text = |Invalid cardinality { first }| line = token-line ).
    ENDIF.
    expect_op( ']' ).
  ENDMETHOD.


  METHOD read_reference.
    " 'constant' | [TABLE .] FIELD
    DATA token TYPE ty_token.
    DATA name TYPE string.

    CLEAR: table, field, const, is_const.
    token = peek( ).
    IF token-kind = 'S'.
      next( ).
      const = token-text.
      is_const = abap_true.
      RETURN.
    ENDIF.
    name = read_name( ).
    IF is_op( '.' ) = abap_true.
      next( ).
      table = name.
      field = read_name( ).
    ELSE.
      field = name.
    ENDIF.
  ENDMETHOD.


  METHOD read_value_help.
    DATA value_help TYPE dd35v.
    DATA vh_field TYPE dd36m.
    DATA table TYPE string.
    DATA field TYPE string.
    DATA const TYPE string.
    DATA is_const TYPE abap_bool.

    value_help-tabname = struc_name.
    value_help-fieldname = fieldname.
    value_help-shlpinher = shlpinher.
    expect_kw( 'WITH' ).
    expect_kw( 'VALUE' ).
    expect_kw( 'HELP' ).
    value_help-shlpname = read_name( ).
    expect_kw( 'WHERE' ).
    DO.
      CLEAR vh_field.
      vh_field-tabname = struc_name.
      vh_field-fieldname = fieldname.
      vh_field-shlpname = value_help-shlpname.
      vh_field-shlpfield = read_name( ).
      expect_op( '=' ).
      read_reference( IMPORTING table = table field = field const = const is_const = is_const ).
      IF is_const = abap_true.
        vh_field-shtype = 'C'.
        vh_field-shtable = |'{ to_upper( const ) }'|.
      ELSEIF table IS NOT INITIAL.
        vh_field-shtable = table.
        vh_field-shfield = field.
      ELSE.
        vh_field-shtable = struc_name.
        vh_field-shfield = field.
      ENDIF.
      READ TABLE vh_fields TRANSPORTING NO FIELDS
        WITH KEY fieldname = vh_field-fieldname shlpname = vh_field-shlpname shlpfield = vh_field-shlpfield.
      IF sy-subrc <> 0.
        APPEND vh_field TO vh_fields.
      ENDIF.
      IF is_kw( 'AND' ) = abap_false.
        EXIT.
      ENDIF.
      next( ).
    ENDDO.
    APPEND value_help TO value_helps.
  ENDMETHOD.


  METHOD adjusted_fk_fields.
    " As the editor saves: every key field of the check table, at its
    " position there, the unassigned ones generic.
    DATA foreign_key TYPE dd08v.
    DATA fk_field TYPE dd05m.
    DATA check_fields TYPE STANDARD TABLE OF dd03k.
    DATA check_field TYPE dd03k.
    DATA new TYPE dd05m.

    LOOP AT foreign_keys INTO foreign_key WHERE noinherit <> 'N'.
      SELECT * FROM dd03k INTO TABLE check_fields BYPASSING BUFFER
        WHERE tabname = foreign_key-checktable.
      LOOP AT fk_fields INTO fk_field WHERE fieldname = foreign_key-fieldname.
        READ TABLE check_fields INTO check_field
          WITH KEY tabname = fk_field-checktable fieldname = fk_field-checkfield.
        IF sy-subrc = 0.
          fk_field-primpos = check_field-position.
        ENDIF.
        APPEND fk_field TO result.
      ENDLOOP.
      LOOP AT check_fields INTO check_field.
        READ TABLE result TRANSPORTING NO FIELDS
          WITH KEY tabname = foreign_key-tabname checktable = check_field-tabname
                   checkfield = check_field-fieldname.
        IF sy-subrc <> 0.
          CLEAR new.
          new-tabname = foreign_key-tabname.
          new-fieldname = foreign_key-fieldname.
          new-fortable = '*'.
          new-checktable = check_field-tabname.
          new-checkfield = check_field-fieldname.
          new-primpos = check_field-position.
          APPEND new TO result.
        ENDIF.
      ENDLOOP.
    ENDLOOP.
  ENDMETHOD.


  METHOD zif_vsp_ddic_source~render.
    DATA state TYPE ddobjstate.
    DATA gotstate TYPE ddgotstate.
    DATA masterlang TYPE masterlang.
    DATA tadir_name TYPE sobj_name.
    DATA dd02v TYPE dd02v.
    DATA header_logon TYPE dd02v.
    DATA dd03p_tab TYPE dd03ptab.
    DATA dd05m_tab TYPE dd05mttyp.
    DATA dd08v_tab TYPE dd08vttyp.
    DATA dd35v_tab TYPE dd35vttyp.
    DATA dd36m_tab TYPE dd36mttyp.
    DATA field TYPE dd03p.
    DATA owners TYPE ty_owners.
    DATA owner TYPE ty_owner.
    DATA inc_fields TYPE dd03ptab.
    DATA inc_field TYPE dd03p.
    DATA suffix TYPE string.
    DATA tops TYPE dd03ptab.
    DATA top_index TYPE i.
    DATA max_colon TYPE i.
    DATA offset TYPE i.
    DATA text TYPE string.
    DATA line TYPE string.
    DATA left TYPE string.
    DATA right TYPE string.
    DATA annotations TYPE string.
    DATA adds TYPE string.
    DATA source TYPE string.
    DATA newline TYPE string.
    DATA fks TYPE dd08vttyp.
    DATA fkf TYPE dd05mttyp.
    DATA vhs TYPE dd35vttyp.
    DATA vhf TYPE dd36mttyp.
    DATA fk TYPE dd08v.
    DATA vh TYPE dd35v.
    DATA app_fks TYPE dd08vttyp.
    DATA app_vhs TYPE dd35vttyp.
    DATA lines TYPE i.
    DATA statements TYPE string_table.
    DATA is_include TYPE abap_bool.
    DATA owner_field TYPE dd03p.

    newline = cl_abap_char_utilities=>newline.
    IF version = 'A'.
      state = 'A'.
    ELSE.
      state = 'M'.
    ENDIF.

    " Texts as 7.50 writes them: the object's in its original language,
    " the structure's own label in the logon language where it has one.
    SELECT SINGLE masterlang FROM tadir INTO masterlang
      WHERE pgmid = 'R3TR' AND object = 'TABL' AND obj_name = name.
    IF masterlang IS INITIAL.
      masterlang = sy-langu.
    ENDIF.
    CALL FUNCTION 'DDIF_TABL_GET'
      EXPORTING
        name          = name
        state         = state
        langu         = masterlang
      IMPORTING
        gotstate      = gotstate
        dd02v_wa      = dd02v
      TABLES
        dd03p_tab     = dd03p_tab
        dd05m_tab     = dd05m_tab
        dd08v_tab     = dd08v_tab
        dd35v_tab     = dd35v_tab
        dd36m_tab     = dd36m_tab
      EXCEPTIONS
        OTHERS        = 1.
    IF sy-subrc <> 0 OR dd02v IS INITIAL.
      RETURN.
    ENDIF.
    IF gotstate <> state AND NOT ( state = 'M' AND gotstate = 'N' ).
      RETURN.
    ENDIF.
    IF masterlang <> sy-langu.
      CALL FUNCTION 'DDIF_TABL_GET'
        EXPORTING
          name     = name
          state    = state
          langu    = sy-langu
        IMPORTING
          dd02v_wa = header_logon
        EXCEPTIONS
          OTHERS   = 1.
      " Only the logon language's: 7.50 writes '' when there is none.
      dd02v-ddtext = header_logon-ddtext.
    ENDIF.

    " Top-level components; an included field is found through its include.
    LOOP AT dd03p_tab INTO field.
      IF field-depth IS NOT INITIAL AND field-depth <> '00'.
        CONTINUE.
      ENDIF.
      IF field-adminfield <> '0'.
        CONTINUE.
      ENDIF.
      IF is_include_row( field ) = abap_true AND field-fieldname = '.INCLU--AP'.
        CONTINUE. " an append: not written in the structure it extends
      ENDIF.
      APPEND field TO tops.
      top_index = lines( tops ).
      IF is_include_row( field ) = abap_true.
        CLEAR: inc_fields, suffix.
        IF field-fieldname(7) = '.INCLU-'.
          suffix = field-fieldname+7.
        ENDIF.
        CALL FUNCTION 'DDIF_TABL_GET'
          EXPORTING
            name      = field-precfield
          TABLES
            dd03p_tab = inc_fields
          EXCEPTIONS
            OTHERS    = 1.
        LOOP AT inc_fields INTO inc_field WHERE fieldname NP '.INCLU*'.
          CLEAR owner.
          IF inc_field-adminfield = '0'.
            owner-fieldname = inc_field-fieldname && suffix.
          ELSE.
            owner-fieldname = inc_field-fieldname.
          ENDIF.
          owner-include = top_index.
          INSERT owner INTO TABLE owners.
        ENDLOOP.
      ELSE.
        CLEAR owner.
        owner-fieldname = field-fieldname.
        owner-include = top_index.
        INSERT owner INTO TABLE owners.
      ENDIF.
    ENDLOOP.

    " Alignment as 7.50 does it: the first ':' of every plain component
    " in the column of the rightmost one.
    LOOP AT tops INTO field.
      IF is_include_row( field ) = abap_true.
        CONTINUE.
      ENDIF.
      text = |{ to_lower( field-fieldname ) } : { field_type_text( field ) }|.
      IF field-keyflag = abap_true.
        text = |key { text }|.
      ENDIF.
      offset = find( val = text sub = ':' ).
      IF offset > max_colon.
        max_colon = offset.
      ENDIF.
    ENDLOOP.

    " Header.
    source = |@{ co_label } : '{ escape( dd02v-ddtext ) }'{ newline }| &&
             |@{ co_enh_category } : { enh_category_of( dd02v-exclass ) }{ newline }|.
    IF dd02v-tabclass = 'APPEND'.
      source = source && |extend type { to_lower( dd02v-sqltab ) } with { to_lower( dd02v-tabname ) } \{{ newline }|.
    ELSE.
      source = source && |define type { to_lower( dd02v-tabname ) } \{{ newline }|.
    ENDIF.

    LOOP AT tops INTO field.
      top_index = sy-tabix.
      is_include = is_include_row( field ).
      " The foreign keys and value helps written with this component.
      CLEAR: fks, fkf, vhs, vhf.
      " As 7.50: a key of the component (space) or set in an include
      " (Y); inherited ones (N), an append's (E, A) are not written here.
      LOOP AT dd08v_tab INTO fk WHERE noinherit = space OR noinherit = 'Y'.
        READ TABLE owners INTO owner WITH TABLE KEY fieldname = fk-fieldname.
        IF sy-subrc = 0 AND owner-include = top_index.
          APPEND fk TO fks.
        ENDIF.
      ENDLOOP.
      LOOP AT dd35v_tab INTO vh WHERE shlpinher = space.
        READ TABLE owners INTO owner WITH TABLE KEY fieldname = vh-fieldname.
        IF sy-subrc = 0 AND owner-include = top_index.
          APPEND vh TO vhs.
        ENDIF.
      ENDLOOP.
      LOOP AT dd05m_tab INTO DATA(fk_field).
        READ TABLE fks TRANSPORTING NO FIELDS WITH KEY fieldname = fk_field-fieldname.
        IF sy-subrc = 0.
          APPEND fk_field TO fkf.
        ENDIF.
      ENDLOOP.
      LOOP AT dd36m_tab INTO DATA(vh_field).
        READ TABLE vhs TRANSPORTING NO FIELDS WITH KEY fieldname = vh_field-fieldname.
        IF sy-subrc = 0.
          APPEND vh_field TO vhf.
        ENDIF.
      ENDLOOP.

      IF is_include = abap_true.
        line = |include { to_lower( field-precfield ) }|.
        IF field-fieldname(7) = '.INCLU-'.
          line = |{ line } with suffix { to_lower( field-fieldname+7 ) }|.
        ENDIF.
        IF field-groupname IS NOT INITIAL.
          line = |{ to_lower( field-groupname ) } : { line }|.
        ENDIF.
        IF field-keyflag = abap_true.
          line = |key { line }|.
        ENDIF.
        IF field-notnull = abap_true.
          line = |{ line } not null|.
        ENDIF.
        left = substring_before( val = line sub = ':' ).
        IF left IS NOT INITIAL AND max_colon - strlen( left ) > 0.
          right = substring_after( val = line sub = ':' ).
          line = left && spaces( max_colon - strlen( left ) ) && ':' && right.
        ENDIF.
        adds = format_additions( indent = 4
                                 text   = additions( kind         = 'I'
                                                     foreign_keys = fks
                                                     fk_fields    = fkf
                                                     value_helps  = vhs
                                                     vh_fields    = vhf ) ).
        IF adds IS NOT INITIAL.
          adds = newline && adds.
        ENDIF.
        source = |{ source }  { line }{ adds };{ newline }|.
        CONTINUE.
      ENDIF.

      " A plain component: annotations, name : type, additions.
      CLEAR annotations.
      IF ( ( field-rollname IS INITIAL AND field-precfield IS INITIAL )
           OR ( field-comptype = 'R' AND field-reftype = 'B' ) )
          AND field-ddtext IS NOT INITIAL.
        annotations = |  @{ co_label } : '{ escape( field-ddtext ) }'{ newline }|.
      ENDIF.
      IF field-datatype = 'CURR' AND ( field-reftable IS NOT INITIAL OR field-reffield IS NOT INITIAL ).
        annotations = |{ annotations }  @{ co_amount } : '{ to_lower( field-reftable ) }.{ to_lower( field-reffield ) }'{ newline }|.
      ENDIF.
      IF field-datatype = 'QUAN' AND ( field-reftable IS NOT INITIAL OR field-reffield IS NOT INITIAL ).
        annotations = |{ annotations }  @{ co_quantity } : '{ to_lower( field-reftable ) }.{ to_lower( field-reffield ) }'{ newline }|.
      ENDIF.
      IF field-languflag = abap_true.
        annotations = |{ annotations }  @{ co_text_lang }{ newline }|.
      ENDIF.
      READ TABLE fks INTO fk INDEX 1.
      IF sy-subrc = 0.
        annotations = annotations && fk_annotations( foreign_key = fk indent = `  ` ).
      ENDIF.

      line = |{ to_lower( field-fieldname ) } : { field_type_text( field ) }|.
      IF field-keyflag = abap_true.
        line = |key { line }|.
      ENDIF.
      IF field-notnull = abap_true.
        line = |{ line } not null|.
      ENDIF.
      left = substring_before( val = line sub = ':' ).
      IF left IS NOT INITIAL AND max_colon - strlen( left ) > 0.
        right = substring_after( val = line sub = ':' ).
        line = left && spaces( max_colon - strlen( left ) ) && ':' && right.
      ENDIF.
      adds = format_additions( indent = 4
                               text   = additions( kind         = 'C'
                                                   foreign_keys = fks
                                                   fk_fields    = fkf
                                                   value_helps  = vhs
                                                   vh_fields    = vhf ) ).
      IF adds IS NOT INITIAL.
        adds = newline && adds.
      ENDIF.
      source = |{ source }{ annotations }  { line }{ adds };{ newline }|.
    ENDLOOP.

    " An append's own foreign keys and value helps for fields it extends.
    LOOP AT dd08v_tab INTO fk WHERE noinherit = 'E' AND checktable <> '*'.
      APPEND fk TO app_fks.
    ENDLOOP.
    LOOP AT dd35v_tab INTO vh WHERE shlpinher = 'E' AND shlpname <> '*'.
      APPEND vh TO app_vhs.
    ENDLOOP.
    IF app_fks IS NOT INITIAL OR app_vhs IS NOT INITIAL.
      CLEAR: fkf, vhf.
      LOOP AT dd05m_tab INTO fk_field.
        READ TABLE app_fks TRANSPORTING NO FIELDS WITH KEY fieldname = fk_field-fieldname.
        IF sy-subrc = 0.
          APPEND fk_field TO fkf.
        ENDIF.
      ENDLOOP.
      LOOP AT dd36m_tab INTO vh_field.
        READ TABLE app_vhs TRANSPORTING NO FIELDS WITH KEY fieldname = vh_field-fieldname.
        IF sy-subrc = 0.
          APPEND vh_field TO vhf.
        ENDIF.
      ENDLOOP.
      source = source && format_additions( indent = 2
                                           text   = additions( kind         = 'A'
                                                               foreign_keys = app_fks
                                                               fk_fields    = fkf
                                                               value_helps  = app_vhs
                                                               vh_fields    = vhf ) ) && newline.
    ENDIF.
    source = |{ source }{ newline }\}|.

    SPLIT source AT newline INTO TABLE result.
  ENDMETHOD.


  METHOD is_include_row.
    IF field-fieldname CP '.INCLU*'.
      IF field-comptype = 'S'.
        result = abap_true.
      ELSEIF field-comptype IS INITIAL.
        FIND REGEX '^(ci_|si)' IN field-precfield IGNORING CASE.
        result = boolc( sy-subrc = 0 ).
      ENDIF.
    ENDIF.
  ENDMETHOD.


  METHOD enh_category_of.
    CASE exclass.
      WHEN '0'. result = '#NOT_CLASSIFIED'.
      WHEN '1'. result = '#NOT_EXTENSIBLE'.
      WHEN '2'. result = '#EXTENSIBLE_CHARACTER'.
      WHEN '3'. result = '#EXTENSIBLE_CHARACTER_NUMERIC'.
      WHEN '4'. result = '#EXTENSIBLE_ANY'.
    ENDCASE.
  ENDMETHOD.


  METHOD pattern_of.
    DATA type TYPE ty_type.
    READ TABLE types INTO type WITH TABLE KEY ddic = ddic.
    IF sy-subrc = 0.
      result = type-pattern.
      REPLACE 'len' IN result WITH |{ leng }|.
      REPLACE 'decimals' IN result WITH |{ decimals }|.
    ENDIF.
  ENDMETHOD.


  METHOD field_type_text.
    DATA leng TYPE i.
    DATA decimals TYPE i.

    leng = field-leng.
    decimals = field-decimals.
    IF field-comptype IS INITIAL.
      result = |abap.{ pattern_of( ddic = field-datatype leng = leng decimals = decimals ) }|.
    ELSEIF field-comptype = 'R' AND field-reftype = 'J'.
      result = |{ to_lower( field-rollname ) } boxed|.
    ELSEIF field-comptype = 'R' AND field-reftype = 'B'.
      result = |reference to abap.{ pattern_of( ddic = field-rollname leng = leng decimals = decimals ) }|.
    ELSEIF field-comptype = 'R'.
      result = |reference to { to_lower( field-rollname ) }|.
    ELSE.
      result = to_lower( field-rollname ).
    ENDIF.
  ENDMETHOD.


  METHOD escape.
    result = text.
    REPLACE ALL OCCURRENCES OF `'` IN result WITH `''`.
  ENDMETHOD.


  METHOD spaces.
    DATA i TYPE i.
    WHILE i < count.
      result = result && ` `.
      i = i + 1.
    ENDWHILE.
  ENDMETHOD.


  METHOD fk_annotations.
    DATA newline TYPE string.
    newline = cl_abap_char_utilities=>newline.
    IF foreign_key-ddtext IS NOT INITIAL.
      result = |{ result }{ indent }@{ co_fk_label } : '{ foreign_key-ddtext }'{ newline }|.
    ENDIF.
    CASE foreign_key-frkart.
      WHEN 'KEY'.  result = |{ result }{ indent }@{ co_fk_keytype } : #KEY{ newline }|.
      WHEN 'REF'.  result = |{ result }{ indent }@{ co_fk_keytype } : #NON_KEY{ newline }|.
      WHEN 'TEXT'. result = |{ result }{ indent }@{ co_fk_keytype } : #TEXT_KEY{ newline }|.
    ENDCASE.
    IF foreign_key-checkflag <> abap_true.
      result = |{ result }{ indent }@{ co_fk_screen } : true{ newline }|.
    ELSE.
      result = |{ result }{ indent }@{ co_fk_screen } : false{ newline }|.
    ENDIF.
    IF foreign_key-arbgb IS NOT INITIAL.
      result = |{ result }{ indent }@{ co_fk_msgclass } : '{ foreign_key-arbgb }'{ newline }|.
    ENDIF.
    IF foreign_key-msgnr IS NOT INITIAL.
      result = |{ result }{ indent }@{ co_fk_msgnr } : '{ foreign_key-msgnr }'{ newline }|.
    ENDIF.
  ENDMETHOD.


  METHOD where_clause.
    DATA fk_field TYPE dd05m.
    DATA vh_field TYPE dd36m.
    DATA count TYPE i.
    DATA word TYPE string.
    DATA value TYPE string.
    DATA newline TYPE string.

    newline = cl_abap_char_utilities=>newline.
    LOOP AT fk_fields INTO fk_field WHERE fortable <> '*'.
      count = count + 1.
      IF count = 1.
        word = 'where'.
      ELSE.
        word = 'and'.
      ENDIF.
      IF fk_field-forkey IS INITIAL.
        result = |{ result }{ newline }{ word } { to_lower( fk_field-checkfield ) } = { fk_field-fortable }|.
      ELSE.
        result = |{ result }{ newline }{ word } { to_lower( fk_field-checkfield ) } = | &&
                 |{ to_lower( fk_field-fortable ) }.{ to_lower( fk_field-forkey ) }|.
      ENDIF.
    ENDLOOP.
    CLEAR count.
    LOOP AT vh_fields INTO vh_field WHERE shtype <> 'G'.
      count = count + 1.
      IF count = 1.
        word = 'where'.
      ELSE.
        word = 'and'.
      ENDIF.
      value = vh_field-shtable.
      IF vh_field-shfield IS NOT INITIAL.
        value = |{ to_lower( value ) }.{ to_lower( vh_field-shfield ) }|.
      ENDIF.
      result = |{ result }{ newline }{ word } { to_lower( vh_field-shlpfield ) } = { value }|.
    ENDLOOP.
  ENDMETHOD.


  METHOD additions.
    " The "with foreign key / with value help" lines of one component,
    " of an include's extensions (kind I) or of an append's (kind A).
    DATA fk TYPE dd08v.
    DATA vh TYPE dd35v.
    DATA fkf TYPE dd05mttyp.
    DATA vhf TYPE dd36mttyp.
    DATA fk_field TYPE dd05m.
    DATA vh_field TYPE dd36m.
    DATA part TYPE string.
    DATA extension TYPE string.
    DATA card TYPE string.
    DATA source_card TYPE string.
    DATA target_card TYPE string.
    DATA newline TYPE string.

    newline = cl_abap_char_utilities=>newline.

    LOOP AT foreign_keys INTO fk.
      CLEAR: part, extension, fkf.
      IF kind <> 'C'.
        extension = |extend { to_lower( fk-fieldname ) } :{ newline }|.
        IF fk-checktable = '*'.
          part = |{ extension }remove foreign key|.
        ENDIF.
      ENDIF.
      IF part IS INITIAL.
        CLEAR: card, source_card, target_card.
        IF fk-cardleft IS NOT INITIAL AND fk-card IS NOT INITIAL.
          CASE fk-cardleft.
            WHEN '1'. target_card = '1'.
            WHEN 'C'. target_card = '0..1'.
          ENDCASE.
          CASE fk-card.
            WHEN '1'.  source_card = '1'.
            WHEN 'C'.  source_card = '0..1'.
            WHEN 'N'.  source_card = '1..*'.
            WHEN 'CN'. source_card = '0..*'.
          ENDCASE.
          card = | [{ source_card },{ target_card }]|.
        ENDIF.
        LOOP AT fk_fields INTO fk_field WHERE fieldname = fk-fieldname.
          APPEND fk_field TO fkf.
        ENDLOOP.
        part = |with foreign key{ card } { to_lower( fk-checktable ) }{ where_clause( fk_fields = fkf ) }|.
        IF kind <> 'C'.
          " 7.50 means to indent these by 4 and passes '    ' as a
          " string, which is empty: they start at the line's indent.
          part = fk_annotations( foreign_key = fk indent = `` ) && extension && part.
        ENDIF.
      ENDIF.
      " A value help of the same field goes with it.
      READ TABLE value_helps INTO vh WITH KEY fieldname = fk-fieldname.
      IF sy-subrc = 0.
        IF kind <> 'C' AND vh-shlpname = '*'.
          part = |{ part }{ newline }remove value help|.
        ELSEIF NOT ( kind = 'C' AND vh-shlpname = '*' ).
          CLEAR vhf.
          LOOP AT vh_fields INTO vh_field WHERE fieldname = vh-fieldname.
            APPEND vh_field TO vhf.
          ENDLOOP.
          part = |{ part }{ newline }with value help { to_lower( vh-shlpname ) }{ where_clause( vh_fields = vhf ) }|.
        ENDIF.
      ENDIF.
      IF kind = 'A'.
        part = part && ';'.
      ENDIF.
      IF result IS INITIAL.
        result = part.
      ELSE.
        result = result && newline && part.
      ENDIF.
    ENDLOOP.

    LOOP AT value_helps INTO vh.
      READ TABLE foreign_keys TRANSPORTING NO FIELDS WITH KEY fieldname = vh-fieldname.
      IF sy-subrc = 0.
        CONTINUE.
      ENDIF.
      CLEAR: part, extension.
      IF kind <> 'C'.
        extension = |extend { to_lower( vh-fieldname ) } :{ newline }|.
        IF vh-shlpname = '*'.
          part = |{ extension }remove value help|.
        ENDIF.
      ELSEIF vh-shlpname = '*'.
        CONTINUE.
      ENDIF.
      IF part IS INITIAL.
        CLEAR vhf.
        LOOP AT vh_fields INTO vh_field WHERE fieldname = vh-fieldname.
          APPEND vh_field TO vhf.
        ENDLOOP.
        part = |{ extension }with value help { to_lower( vh-shlpname ) }{ where_clause( vh_fields = vhf ) }|.
      ENDIF.
      IF kind = 'A'.
        part = part && ';'.
      ENDIF.
      IF result IS INITIAL.
        result = part.
      ELSE.
        result = result && newline && part.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.


  METHOD format_additions.
    " Indentation as 7.50 writes it: annotations and "extend" at the
    " start, "with" and "remove" 2 further when an extension leads,
    " "where" 2 further, "and" 2 more.
    DATA parts TYPE string_table.
    DATA part TYPE string.
    DATA first TYPE string.
    DATA indent_with TYPE i.
    DATA indent_where TYPE i.
    DATA indent_and TYPE i.
    DATA newline TYPE string.

    IF text IS INITIAL.
      RETURN.
    ENDIF.
    newline = cl_abap_char_utilities=>newline.
    SPLIT text AT newline INTO TABLE parts.
    READ TABLE parts INTO first INDEX 1.
    IF first CP '@*' OR first CP 'extend*'.
      indent_with = indent + 2.
    ELSE.
      indent_with = indent.
    ENDIF.
    indent_where = indent_with + 2.
    indent_and = indent_where + 2.

    LOOP AT parts INTO part.
      IF sy-tabix = 1.
        result = spaces( indent ) && part.
        CONTINUE.
      ENDIF.
      IF part CP '@*' OR part CP 'extend*'.
        result = result && newline && spaces( indent ) && part.
      ELSEIF part CP 'with*' OR part CP 'remove*'.
        result = result && newline && spaces( indent_with ) && part.
      ELSEIF part CP 'where*'.
        result = result && newline && spaces( indent_where ) && part.
      ELSEIF part CP 'and*'.
        result = result && newline && spaces( indent_and ) && part.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.
ENDCLASS.
