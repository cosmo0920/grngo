package grngo

/*
#cgo pkg-config: groonga
#include "grngo.h"
*/
import "C"

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

// -- Data types --

// Relationships among Groonga and Golang built-in data types are as follows:
//
// - ID: uint32
// - Bool: bool
// - (U)Int8/16/32/64: int64
// - Float: float64
// - Time: TODO
// - WGS84/TokyoGeoPoint: GeoPoint
// - (Short/Long)Text: []byte

type GeoPoint struct{ Latitude, Longitude int32 }

const NilID = uint32(C.GRN_ID_NIL)

type DataType int

const (
	Void          = DataType(C.GRN_DB_VOID)
	Bool          = DataType(C.GRN_DB_BOOL)
	Int8          = DataType(C.GRN_DB_INT8)
	Int16         = DataType(C.GRN_DB_INT16)
	Int32         = DataType(C.GRN_DB_INT32)
	Int64         = DataType(C.GRN_DB_INT64)
	UInt8         = DataType(C.GRN_DB_UINT8)
	UInt16        = DataType(C.GRN_DB_UINT16)
	UInt32        = DataType(C.GRN_DB_UINT32)
	UInt64        = DataType(C.GRN_DB_UINT64)
	Float         = DataType(C.GRN_DB_FLOAT)
	Time          = DataType(C.GRN_DB_TIME)
	ShortText     = DataType(C.GRN_DB_SHORT_TEXT)
	Text          = DataType(C.GRN_DB_TEXT)
	LongText      = DataType(C.GRN_DB_LONG_TEXT)
	TokyoGeoPoint = DataType(C.GRN_DB_TOKYO_GEO_POINT)
	WGS84GeoPoint = DataType(C.GRN_DB_WGS84_GEO_POINT)
)

func (dataType DataType) String() string {
	switch dataType {
	case Void:
		return "Void"
	case Bool:
		return "Bool"
	case Int8:
		return "Int8"
	case Int16:
		return "Int16"
	case Int32:
		return "Int32"
	case Int64:
		return "Int64"
	case UInt8:
		return "UInt8"
	case UInt16:
		return "UInt16"
	case UInt32:
		return "UInt32"
	case UInt64:
		return "UInt64"
	case Float:
		return "Float"
	case Time:
		return "Time"
	case ShortText:
		return "ShortText"
	case Text:
		return "Text"
	case LongText:
		return "LongText"
	case TokyoGeoPoint:
		return "TokyoGeoPoint"
	case WGS84GeoPoint:
		return "WGS84GeoPoint"
	default:
		return fmt.Sprintf("DataType(%d)", dataType)
	}
}

// -- TableOptions --

// Constants for TableOptions.
type TableType int

const (
	ArrayTable = TableType(iota)
	HashTable
	PatTable
	DatTable
)

// http://groonga.org/docs/reference/commands/table_create.html
type TableOptions struct {
	TableType
	WithSIS          bool     // KEY_WITH_SIS
	KeyType          string   // http://groonga.org/docs/reference/types.html
	ValueType        string   // http://groonga.org/docs/reference/types.html
	DefaultTokenizer string   // http://groonga.org/docs/reference/tokenizers.html
	Normalizer       string   // http://groonga.org/docs/reference/normalizers.html
	TokenFilters     []string // http://groonga.org/docs/reference/token_filters.html
}

// NewTableOptions() creates a new TableOptions object with the default
// settings.
func NewTableOptions() *TableOptions {
	var options TableOptions
	return &options
}

// -- ColumnOptions --

// Constants for ColumnOptions.
type ColumnType int

const (
	ScalarColumn = ColumnType(iota)
	VectorColumn
	IndexColumn
)

// Constants for ColumnOptions.
type CompressionType int

const (
	NoCompression = CompressionType(iota)
	ZlibCompression
	LzoCompression
)

// http://groonga.org/ja/docs/reference/commands/column_create.html
type ColumnOptions struct {
	ColumnType
	CompressionType
	WithSection  bool // WITH_SECTION
	WithWeight   bool // WITH_WEIGHT
	WithPosition bool // WITH_POSITION
	Source       string
}

// NewColumnOptions() creates a new ColumnOptions object with the default
// settings.
func NewColumnOptions() *ColumnOptions {
	var options ColumnOptions
	return &options
}

// -- Groonga --

// initCount is a counter for automatically initializing and finalizing
// Groonga.
var initCount = 0

// DisableInitCount() disables initCount.
// This is useful if you want to manyally initialize and finalize Groonga.
func DisableInitCount() {
	initCount = -1
}

// Init() initializes Groonga if needed.
// initCount is incremented and when it changes from 0 to 1, Groonga is
// initialized.
func Init() error {
	switch initCount {
	case -1: // Disabled.
		return nil
	case 0:
		if rc := C.grn_init(); rc != C.GRN_SUCCESS {
			return fmt.Errorf("grn_init() failed: rc = %d", rc)
		}
	}
	initCount++
	return nil
}

// Fin() finalizes Groonga if needed.
// initCount is decremented and when it changes from 1 to 0, Groonga is
// finalized.
func Fin() error {
	switch initCount {
	case -1: // Disabled.
		return nil
	case 0:
		return fmt.Errorf("Groonga is not initialized yet")
	case 1:
		if rc := C.grn_fin(); rc != C.GRN_SUCCESS {
			return fmt.Errorf("grn_fin() failed: rc = %d", rc)
		}
	}
	initCount--
	return nil
}

// openCtx() allocates memory for grn_ctx and initializes it.
func openCtx() (*C.grn_ctx, error) {
	if err := Init(); err != nil {
		return nil, err
	}
	ctx := C.grn_ctx_open(0)
	if ctx == nil {
		Fin()
		return nil, fmt.Errorf("grn_ctx_open() failed")
	}
	return ctx, nil
}

// closeCtx() finalizes grn_ctx and frees allocated memory.
func closeCtx(ctx *C.grn_ctx) error {
	rc := C.grn_ctx_close(ctx)
	Fin()
	if rc != C.GRN_SUCCESS {
		return fmt.Errorf("grn_ctx_close() failed: rc = %d", rc)
	}
	return nil
}

// -- DB --

type DB struct {
	ctx    *C.grn_ctx
	obj    *C.grn_obj
	tables map[string]*Table
}

// newDB() creates a new DB object.
func newDB(ctx *C.grn_ctx, obj *C.grn_obj) *DB {
	return &DB{ctx, obj, make(map[string]*Table)}
}

// CreateDB() creates a Groonga database and returns a handle to it.
// A temporary database is created if path is empty.
func CreateDB(path string) (*DB, error) {
	ctx, err := openCtx()
	if err != nil {
		return nil, err
	}
	var cPath *C.char
	if path != "" {
		cPath = C.CString(path)
		defer C.free(unsafe.Pointer(cPath))
	}
	obj := C.grn_db_create(ctx, cPath, nil)
	if obj == nil {
		closeCtx(ctx)
		errMsg := C.GoString(&ctx.errbuf[0])
		return nil, fmt.Errorf("grn_db_create() failed: err = %s", errMsg)
	}
	return newDB(ctx, obj), nil
}

// OpenDB() opens an existing Groonga database and returns a handle.
func OpenDB(path string) (*DB, error) {
	ctx, err := openCtx()
	if err != nil {
		return nil, err
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	obj := C.grn_db_open(ctx, cPath)
	if obj == nil {
		closeCtx(ctx)
		errMsg := C.GoString(&ctx.errbuf[0])
		return nil, fmt.Errorf("grn_db_open() failed: err = %s", errMsg)
	}
	return newDB(ctx, obj), nil
}

// Close() closes a handle.
func (db *DB) Close() error {
	rc := C.grn_obj_close(db.ctx, db.obj)
	if rc != C.GRN_SUCCESS {
		closeCtx(db.ctx)
		return fmt.Errorf("grn_obj_close() failed: rc = %d", rc)
	}
	return closeCtx(db.ctx)
}

// Send() sends a raw command.
// The given command must be well-formed.
func (db *DB) Send(command string) error {
	commandBytes := []byte(command)
	var cCommand *C.char
	if len(commandBytes) != 0 {
		cCommand = (*C.char)(unsafe.Pointer(&commandBytes[0]))
	}
	rc := C.grn_ctx_send(db.ctx, cCommand, C.uint(len(commandBytes)), 0)
	switch {
	case rc != C.GRN_SUCCESS:
		errMsg := C.GoString(&db.ctx.errbuf[0])
		return fmt.Errorf("grn_ctx_send() failed: rc = %d, err = %s", rc, errMsg)
	case db.ctx.rc != C.GRN_SUCCESS:
		errMsg := C.GoString(&db.ctx.errbuf[0])
		return fmt.Errorf("grn_ctx_send() failed: ctx.rc = %d, err = %s",
			db.ctx.rc, errMsg)
	}
	return nil
}

// SendEx() sends a command with separated options.
func (db *DB) SendEx(name string, options map[string]string) error {
	if name == "" {
		return fmt.Errorf("invalid command: name = <%s>", name)
	}
	for _, r := range name {
		if (r != '_') && (r < 'a') && (r > 'z') {
			return fmt.Errorf("invalid command: name = <%s>", name)
		}
	}
	commandParts := []string{name}
	for key, value := range options {
		if key == "" {
			return fmt.Errorf("invalid option: key = <%s>", key)
		}
		for _, r := range key {
			if (r != '_') && (r < 'a') && (r > 'z') {
				return fmt.Errorf("invalid option: key = <%s>", key)
			}
		}
		value = strings.Replace(value, "\\", "\\\\", -1)
		value = strings.Replace(value, "'", "\\'", -1)
		commandParts = append(commandParts, fmt.Sprintf("--%s '%s'", key, value))
	}
	return db.Send(strings.Join(commandParts, " "))
}

// Recv() receives the result of commands sent by Send().
func (db *DB) Recv() ([]byte, error) {
	var resultBuffer *C.char
	var resultLength C.uint
	var flags C.int
	rc := C.grn_ctx_recv(db.ctx, &resultBuffer, &resultLength, &flags)
	switch {
	case rc != C.GRN_SUCCESS:
		errMsg := C.GoString(&db.ctx.errbuf[0])
		return nil, fmt.Errorf(
			"grn_ctx_recv() failed: rc = %d, err = %s", rc, errMsg)
	case db.ctx.rc != C.GRN_SUCCESS:
		errMsg := C.GoString(&db.ctx.errbuf[0])
		return nil, fmt.Errorf(
			"grn_ctx_recv() failed: ctx.rc = %d, err = %s", db.ctx.rc, errMsg)
	}
	result := C.GoBytes(unsafe.Pointer(resultBuffer), C.int(resultLength))
	return result, nil
}

// Query() sends a raw command and receive the result.
func (db *DB) Query(command string) ([]byte, error) {
	if err := db.Send(command); err != nil {
		result, _ := db.Recv()
		return result, err
	}
	return db.Recv()
}

// QueryEx() sends a command with separated options and receives the result.
func (db *DB) QueryEx(name string, options map[string]string) (
	[]byte, error) {
	if err := db.SendEx(name, options); err != nil {
		result, _ := db.Recv()
		return result, err
	}
	return db.Recv()
}

// CreateTable() creates a table.
func (db *DB) CreateTable(name string, options *TableOptions) (*Table, error) {
	if options == nil {
		options = NewTableOptions()
	}
	optionsMap := make(map[string]string)
	optionsMap["name"] = name
	switch options.TableType {
	case ArrayTable:
		optionsMap["flags"] = "TABLE_NO_KEY"
	case HashTable:
		optionsMap["flags"] = "TABLE_HASH_KEY"
	case PatTable:
		optionsMap["flags"] = "TABLE_PAT_KEY"
	case DatTable:
		optionsMap["flags"] = "TABLE_DAT_KEY"
	default:
		return nil, fmt.Errorf("undefined table type: options = %+v", options)
	}
	if options.WithSIS {
		optionsMap["flags"] += "|KEY_WITH_SIS"
	}
	if options.KeyType != "" {
		switch options.KeyType {
		case "Bool", "Int8", "Int16", "Int32", "Int64", "UInt8", "UInt16",
			"UInt32", "UInt64", "Float", "Time", "ShortText", "TokyoGeoPoint",
			"WGS84GeoPoint":
			optionsMap["key_type"] = options.KeyType
		default:
			if _, err := db.FindTable(options.KeyType); err != nil {
				return nil, fmt.Errorf("unsupported key type: options = %+v", options)
			}
			optionsMap["key_type"] = options.KeyType
		}
	}
	if options.ValueType != "" {
		switch options.ValueType {
		case "Bool", "Int8", "Int16", "Int32", "Int64", "UInt8", "UInt16",
			"UInt32", "UInt64", "Float", "Time", "TokyoGeoPoint", "WGS84GeoPoint":
			optionsMap["value_type"] = options.ValueType
		default:
			if _, err := db.FindTable(options.ValueType); err != nil {
				return nil, fmt.Errorf("unsupported value type: options = %+v",
					options)
			}
			optionsMap["value_type"] = options.ValueType
		}
	}
	if options.DefaultTokenizer != "" {
		optionsMap["default_tokenizer"] = options.DefaultTokenizer
	}
	if options.Normalizer != "" {
		optionsMap["normalizer"] = options.Normalizer
	}
	if len(options.TokenFilters) != 0 {
		optionsMap["token_filters"] = strings.Join(options.TokenFilters, ",")
	}
	bytes, err := db.QueryEx("table_create", optionsMap)
	if err != nil {
		return nil, err
	}
	if string(bytes) != "true" {
		return nil, fmt.Errorf("table_create failed: name = <%s>", name)
	}
	return db.FindTable(name)
}

// FindTable() finds a table.
func (db *DB) FindTable(name string) (*Table, error) {
	if table, ok := db.tables[name]; ok {
		return table, nil
	}
	nameBytes := []byte(name)
	var cName *C.char
	if len(nameBytes) != 0 {
		cName = (*C.char)(unsafe.Pointer(&nameBytes[0]))
	}
	obj := C.grngo_find_table(db.ctx, cName, C.int(len(nameBytes)))
	if obj == nil {
		return nil, fmt.Errorf("table not found: name = <%s>", name)
	}
	var keyInfo C.grngo_type_info
	if ok := C.grngo_table_get_key_info(db.ctx, obj, &keyInfo); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_table_get_key_info() failed: name = <%s>",
			name)
	}
	// Check the key type.
	keyType := DataType(keyInfo.data_type)
	// Find the destination table if the key is table reference.
	var keyTable *Table
	if keyInfo.ref_table != nil {
		if keyType == Void {
			return nil, fmt.Errorf("reference to void: name = <%s>", name)
		}
		cKeyTableName := C.grngo_table_get_name(db.ctx, keyInfo.ref_table)
		if cKeyTableName == nil {
			return nil, fmt.Errorf("grngo_table_get_name() failed")
		}
		defer C.free(unsafe.Pointer(cKeyTableName))
		var err error
		keyTable, err = db.FindTable(C.GoString(cKeyTableName))
		if err != nil {
			return nil, err
		}
	}
	var valueInfo C.grngo_type_info
	if ok := C.grngo_table_get_value_info(db.ctx, obj, &valueInfo); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_table_get_value_info() failed: name = <%s>",
			name)
	}
	// Check the value type.
	valueType := DataType(valueInfo.data_type)
	// Find the destination table if the value is table reference.
	var valueTable *Table
	if valueInfo.ref_table != nil {
		if valueType == Void {
			return nil, fmt.Errorf("reference to void: name = <%s>", name)
		}
		cValueTableName := C.grngo_table_get_name(db.ctx, valueInfo.ref_table)
		if cValueTableName == nil {
			return nil, fmt.Errorf("grngo_table_get_name() failed")
		}
		defer C.free(unsafe.Pointer(cValueTableName))
		var err error
		valueTable, err = db.FindTable(C.GoString(cValueTableName))
		if err != nil {
			return nil, err
		}
	}
	table := newTable(db, obj, name, keyType, keyTable, valueType, valueTable)
	db.tables[name] = table
	return table, nil
}

// InsertRow() inserts a row.
func (db *DB) InsertRow(tableName string, key interface{}) (bool, uint32, error) {
	table, err := db.FindTable(tableName)
	if err != nil {
		return false, NilID, err
	}
	return table.InsertRow(key)
}

// CreateColumn() creates a column.
func (db *DB) CreateColumn(tableName, columnName string, valueType string,
	options *ColumnOptions) (*Column, error) {
	table, err := db.FindTable(tableName)
	if err != nil {
		return nil, err
	}
	return table.CreateColumn(columnName, valueType, options)
}

// FindColumn() finds a column.
func (db *DB) FindColumn(tableName, columnName string) (*Column, error) {
	table, err := db.FindTable(tableName)
	if err != nil {
		return nil, err
	}
	return table.FindColumn(columnName)
}

// -- Table --

type Table struct {
	db         *DB
	obj        *C.grn_obj
	name       string
	keyType    DataType
	keyTable   *Table
	valueType  DataType
	valueTable *Table
	columns    map[string]*Column
}

// newTable() creates a new Table object.
func newTable(db *DB, obj *C.grn_obj, name string, keyType DataType,
	keyTable *Table, valueType DataType, valueTable *Table) *Table {
	var table Table
	table.db = db
	table.obj = obj
	table.name = name
	table.keyType = keyType
	table.keyTable = keyTable
	table.valueType = valueType
	table.valueTable = valueTable
	table.columns = make(map[string]*Column)
	return &table
}

// insertVoid() inserts an empty row.
func (table *Table) insertVoid() (bool, uint32, error) {
	if table.keyType != Void {
		return false, NilID, fmt.Errorf("key type conflict")
	}
	rowInfo := C.grngo_table_insert_void(table.db.ctx, table.obj)
	if rowInfo.id == C.GRN_ID_NIL {
		return false, NilID, fmt.Errorf("grngo_table_insert_void() failed")
	}
	return rowInfo.inserted == C.GRN_TRUE, uint32(rowInfo.id), nil
}

// insertBool() inserts a row with Bool key.
func (table *Table) insertBool(key bool) (bool, uint32, error) {
	if table.keyType != Bool {
		return false, NilID, fmt.Errorf("key type conflict")
	}
	grnKey := C.grn_bool(C.GRN_FALSE)
	if key {
		grnKey = C.grn_bool(C.GRN_TRUE)
	}
	rowInfo := C.grngo_table_insert_bool(table.db.ctx, table.obj, grnKey)
	if rowInfo.id == C.GRN_ID_NIL {
		return false, NilID, fmt.Errorf("grngo_table_insert_bool() failed")
	}
	return rowInfo.inserted == C.GRN_TRUE, uint32(rowInfo.id), nil
}

// insertInt() inserts a row with Int key.
func (table *Table) insertInt(key int64) (bool, uint32, error) {
	var rowInfo C.grngo_row_info
	switch table.keyType {
	case Int8:
		grnKey := C.int8_t(key)
		rowInfo = C.grngo_table_insert_int8(table.db.ctx, table.obj, grnKey)
	case Int16:
		grnKey := C.int16_t(key)
		rowInfo = C.grngo_table_insert_int16(table.db.ctx, table.obj, grnKey)
	case Int32:
		grnKey := C.int32_t(key)
		rowInfo = C.grngo_table_insert_int32(table.db.ctx, table.obj, grnKey)
	case Int64:
		grnKey := C.int64_t(key)
		rowInfo = C.grngo_table_insert_int64(table.db.ctx, table.obj, grnKey)
	case UInt8:
		grnKey := C.uint8_t(key)
		rowInfo = C.grngo_table_insert_uint8(table.db.ctx, table.obj, grnKey)
	case UInt16:
		grnKey := C.uint16_t(key)
		rowInfo = C.grngo_table_insert_uint16(table.db.ctx, table.obj, grnKey)
	case UInt32:
		grnKey := C.uint32_t(key)
		rowInfo = C.grngo_table_insert_uint32(table.db.ctx, table.obj, grnKey)
	case UInt64:
		grnKey := C.uint64_t(key)
		rowInfo = C.grngo_table_insert_uint64(table.db.ctx, table.obj, grnKey)
	default:
		return false, NilID, fmt.Errorf("key type conflict")
	}
	if rowInfo.id == C.GRN_ID_NIL {
		return false, NilID, fmt.Errorf("grngo_table_insert_int*() failed")
	}
	return rowInfo.inserted == C.GRN_TRUE, uint32(rowInfo.id), nil
}

// insertFloat() inserts a row with Float key.
func (table *Table) insertFloat(key float64) (bool, uint32, error) {
	if table.keyType != Float {
		return false, NilID, fmt.Errorf("key type conflict")
	}
	grnKey := C.double(key)
	rowInfo := C.grngo_table_insert_float(table.db.ctx, table.obj, grnKey)
	if rowInfo.id == C.GRN_ID_NIL {
		return false, NilID, fmt.Errorf("grngo_table_insert_float() failed")
	}
	return rowInfo.inserted == C.GRN_TRUE, uint32(rowInfo.id), nil
}

// insertGeoPoint() inserts a row with GeoPoint key.
func (table *Table) insertGeoPoint(key GeoPoint) (bool, uint32, error) {
	switch table.keyType {
	case TokyoGeoPoint, WGS84GeoPoint:
	default:
		return false, NilID, fmt.Errorf("key type conflict")
	}
	grnKey := C.grn_geo_point{C.int(key.Latitude), C.int(key.Longitude)}
	rowInfo := C.grngo_table_insert_geo_point(table.db.ctx, table.obj, grnKey)
	if rowInfo.id == C.GRN_ID_NIL {
		return false, NilID, fmt.Errorf("grngo_table_insert_geo_point() failed")
	}
	return rowInfo.inserted == C.GRN_TRUE, uint32(rowInfo.id), nil
}

// insertText() inserts a row with Text key.
func (table *Table) insertText(key []byte) (bool, uint32, error) {
	if table.keyType != ShortText {
		return false, NilID, fmt.Errorf("key type conflict")
	}
	var grnKey C.grngo_text
	if len(key) != 0 {
		grnKey.ptr = (*C.char)(unsafe.Pointer(&key[0]))
		grnKey.size = C.size_t(len(key))
	}
	rowInfo := C.grngo_table_insert_text(table.db.ctx, table.obj, &grnKey)
	if rowInfo.id == C.GRN_ID_NIL {
		return false, NilID, fmt.Errorf("grngo_table_insert_text() failed")
	}
	return rowInfo.inserted == C.GRN_TRUE, uint32(rowInfo.id), nil
}

// InsertRow() inserts a row.
// The first return value specifies whether a row is inserted or not.
// The second return value is the ID of the inserted or found row.
func (table *Table) InsertRow(key interface{}) (bool, uint32, error) {
	switch value := key.(type) {
	case nil:
		return table.insertVoid()
	case bool:
		return table.insertBool(value)
	case int64:
		return table.insertInt(value)
	case float64:
		return table.insertFloat(value)
	case GeoPoint:
		return table.insertGeoPoint(value)
	case []byte:
		return table.insertText(value)
	default:
		return false, NilID, fmt.Errorf(
			"unsupported key type: typeName = <%s>", reflect.TypeOf(key).Name())
	}
}

// CreateColumn() creates a column.
func (table *Table) CreateColumn(name string, valueType string,
	options *ColumnOptions) (*Column, error) {
	if options == nil {
		options = NewColumnOptions()
	}
	optionsMap := make(map[string]string)
	optionsMap["table"] = table.name
	optionsMap["name"] = name
	switch valueType {
	case "Bool", "Int8", "Int16", "Int32", "Int64", "UInt8", "UInt16",
		"UInt32", "UInt64", "Float", "Time", "ShortText", "Text", "LongText",
		"TokyoGeoPoint", "WGS84GeoPoint":
		optionsMap["type"] = valueType
	default:
		if _, err := table.db.FindTable(valueType); err != nil {
			return nil, fmt.Errorf("unsupported value type: valueType = %s", valueType)
		}
		optionsMap["type"] = valueType
	}
	switch options.ColumnType {
	case ScalarColumn:
		optionsMap["flags"] = "COLUMN_SCALAR"
	case VectorColumn:
		optionsMap["flags"] = "COLUMN_VECTOR"
	case IndexColumn:
		optionsMap["flags"] = "COLUMN_INDEX"
	default:
		return nil, fmt.Errorf("undefined column type: options = %+v", options)
	}
	switch options.CompressionType {
	case NoCompression:
	case ZlibCompression:
		optionsMap["flags"] = "|COMPRESS_ZLIB"
	case LzoCompression:
		optionsMap["flags"] = "|COMRESS_LZO"
	default:
		return nil, fmt.Errorf("undefined compression type: options = %+v", options)
	}
	if options.WithSection {
		optionsMap["flags"] += "|WITH_SECTION"
	}
	if options.WithWeight {
		optionsMap["flags"] += "|WITH_WEIGHT"
	}
	if options.WithPosition {
		optionsMap["flags"] += "|WITH_POSITION"
	}
	if options.Source != "" {
		optionsMap["source"] = options.Source
	}
	bytes, err := table.db.QueryEx("column_create", optionsMap)
	if err != nil {
		return nil, err
	}
	if string(bytes) != "true" {
		return nil, fmt.Errorf("column_create failed: name = <%s>", name)
	}
	return table.FindColumn(name)
}

// findColumn() finds a column.
func (table *Table) findColumn(name string) (*Column, error) {
	if column, ok := table.columns[name]; ok {
		return column, nil
	}
	nameBytes := []byte(name)
	var cName *C.char
	if len(nameBytes) != 0 {
		cName = (*C.char)(unsafe.Pointer(&nameBytes[0]))
	}
	obj := C.grn_obj_column(table.db.ctx, table.obj, cName, C.uint(len(name)))
	if obj == nil {
		return nil, fmt.Errorf("grn_obj_column() failed: table = %+v, name = <%s>", table, name)
	}
	var valueType DataType
	var valueTable *Table
	var isVector bool
	switch name {
	case "_id":
		valueType = UInt32
	case "_key":
		valueType = table.keyType
		valueTable = table.keyTable
	case "_value":
		valueType = table.valueType
		valueTable = table.valueTable
	default:
		var valueInfo C.grngo_type_info
		if ok := C.grngo_column_get_value_info(table.db.ctx, obj, &valueInfo); ok != C.GRN_TRUE {
			return nil, fmt.Errorf("grngo_column_get_value_info() failed: name = <%s>",
				name)
		}
		// Check the value type.
		valueType = DataType(valueInfo.data_type)
		isVector = valueInfo.dimension > 0
		// Find the destination table if the value is table reference.
		if valueInfo.ref_table != nil {
			if valueType == Void {
				return nil, fmt.Errorf("reference to void: name = <%s>", name)
			}
			cValueTableName := C.grngo_table_get_name(table.db.ctx, valueInfo.ref_table)
			if cValueTableName == nil {
				return nil, fmt.Errorf("grngo_table_get_name() failed")
			}
			defer C.free(unsafe.Pointer(cValueTableName))
			var err error
			valueTable, err = table.db.FindTable(C.GoString(cValueTableName))
			if err != nil {
				return nil, err
			}
		}
	}
	column := newColumn(table, obj, name, valueType, isVector, valueTable)
	table.columns[name] = column
	return column, nil
}

// FindColumn() finds a column.
func (table *Table) FindColumn(name string) (*Column, error) {
	if column, ok := table.columns[name]; ok {
		return column, nil
	}
	delimPos := strings.IndexByte(name, '.')
	if delimPos == -1 {
		return table.findColumn(name)
	}
	columnNames := strings.Split(name, ".")
	column, err := table.findColumn(columnNames[0])
	if err != nil {
		return nil, err
	}
	isVector := column.isVector
	valueTable := column.valueTable
	for _, columnName := range columnNames[1:] {
		if column.valueTable == nil {
			return nil, fmt.Errorf("not table reference: column.name = <%s>", column.name)
		}
		column, err = column.valueTable.findColumn(columnName)
		if err != nil {
			return nil, err
		}
		if column.isVector {
			if isVector {
				return nil, fmt.Errorf("vector of vector is not supported")
			}
			isVector = true
		}
	}
	nameBytes := []byte(name)
	var cName *C.char
	if len(nameBytes) != 0 {
		cName = (*C.char)(unsafe.Pointer(&nameBytes[0]))
	}
	obj := C.grn_obj_column(table.db.ctx, table.obj, cName, C.uint(len(name)))
	if obj == nil {
		return nil, fmt.Errorf("grn_obj_column() failed: name = <%s>", name)
	}
	column = newColumn(table, obj, name, column.valueType, isVector, valueTable)
	table.columns[name] = column
	return column, nil
}

// -- Column --

type Column struct {
	table      *Table
	obj        *C.grn_obj
	name       string
	valueType  DataType
	isVector   bool
	valueTable *Table
}

// newColumn() creates a new Column object.
func newColumn(table *Table, obj *C.grn_obj, name string,
	valueType DataType, isVector bool, valueTable *Table) *Column {
	var column Column
	column.table = table
	column.obj = obj
	column.name = name
	column.valueType = valueType
	column.isVector = isVector
	column.valueTable = valueTable
	return &column
}

// setBool() assigns a Bool value.
func (column *Column) setBool(id uint32, value bool) error {
	if (column.valueType != Bool) || column.isVector {
		return fmt.Errorf("value type conflict")
	}
	var grnValue C.grn_bool = C.GRN_FALSE
	if value {
		grnValue = C.GRN_TRUE
	}
	if ok := C.grngo_column_set_bool(column.table.db.ctx, column.obj,
		C.grn_id(id), grnValue); ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_bool() failed")
	}
	return nil
}

// setInt() assigns an Int value.
func (column *Column) setInt(id uint32, value int64) error {
	if column.isVector {
		return fmt.Errorf("value type conflict")
	}
	ctx := column.table.db.ctx
	var ok C.grn_bool
	switch column.valueType {
	case Int8:
		grnValue := C.int8_t(value)
		ok = C.grngo_column_set_int8(ctx, column.obj, C.grn_id(id), grnValue)
	case Int16:
		grnValue := C.int16_t(value)
		ok = C.grngo_column_set_int16(ctx, column.obj, C.grn_id(id), grnValue)
	case Int32:
		grnValue := C.int32_t(value)
		ok = C.grngo_column_set_int32(ctx, column.obj, C.grn_id(id), grnValue)
	case Int64:
		grnValue := C.int64_t(value)
		ok = C.grngo_column_set_int64(ctx, column.obj, C.grn_id(id), grnValue)
	case UInt8:
		grnValue := C.uint8_t(value)
		ok = C.grngo_column_set_uint8(ctx, column.obj, C.grn_id(id), grnValue)
	case UInt16:
		grnValue := C.uint16_t(value)
		ok = C.grngo_column_set_uint16(ctx, column.obj, C.grn_id(id), grnValue)
	case UInt32:
		grnValue := C.uint32_t(value)
		ok = C.grngo_column_set_uint32(ctx, column.obj, C.grn_id(id), grnValue)
	case UInt64:
		grnValue := C.uint64_t(value)
		ok = C.grngo_column_set_uint64(ctx, column.obj, C.grn_id(id), grnValue)
	default:
		return fmt.Errorf("value type conflict")
	}
	if ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_int*() failed")
	}
	return nil
}

// setFloat() assigns a Float value.
func (column *Column) setFloat(id uint32, value float64) error {
	if (column.valueType != Float) || column.isVector {
		return fmt.Errorf("value type conflict")
	}
	grnValue := C.double(value)
	if ok := C.grngo_column_set_float(column.table.db.ctx, column.obj,
		C.grn_id(id), grnValue); ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_float() failed")
	}
	return nil
}

// setGeoPoint() assigns a GeoPoint value.
func (column *Column) setGeoPoint(id uint32, value GeoPoint) error {
	switch column.valueType {
	case TokyoGeoPoint, WGS84GeoPoint:
	default:
		return fmt.Errorf("value type conflict")
	}
	if column.isVector {
		return fmt.Errorf("value type conflict")
	}
	grnValue := C.grn_geo_point{C.int(value.Latitude), C.int(value.Longitude)}
	if ok := C.grngo_column_set_geo_point(column.table.db.ctx, column.obj,
		C.grn_builtin_type(column.valueType),
		C.grn_id(id), grnValue); ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_geo_point() failed")
	}
	return nil
}

// setText() assigns a Text value.
func (column *Column) setText(id uint32, value []byte) error {
	switch column.valueType {
	case ShortText, Text, LongText:
	default:
		return fmt.Errorf("value type conflict")
	}
	if column.isVector {
		return fmt.Errorf("value type conflict")
	}
	var grnValue C.grngo_text
	if len(value) != 0 {
		grnValue.ptr = (*C.char)(unsafe.Pointer(&value[0]))
		grnValue.size = C.size_t(len(value))
	}
	if ok := C.grngo_column_set_text(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_text() failed")
	}
	return nil
}

// setBoolVector() assigns a Bool vector.
func (column *Column) setBoolVector(id uint32, value []bool) error {
	grnValue := make([]C.grn_bool, len(value))
	for i, v := range value {
		if v {
			grnValue[i] = C.GRN_TRUE
		}
	}
	var grnVector C.grngo_vector
	if len(grnValue) != 0 {
		grnVector.ptr = unsafe.Pointer(&grnValue[0])
		grnVector.size = C.size_t(len(grnValue))
	}
	if ok := C.grngo_column_set_bool_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_bool_vector() failed")
	}
	return nil
}

// setIntVector() assigns an Int vector.
func (column *Column) setIntVector(id uint32, value []int64) error {
	var grnVector C.grngo_vector
	if len(value) != 0 {
		grnVector.ptr = unsafe.Pointer(&value[0])
		grnVector.size = C.size_t(len(value))
	}
	ctx := column.table.db.ctx
	obj := column.obj
	var ok C.grn_bool
	switch column.valueType {
	case Int8:
		ok = C.grngo_column_set_int8_vector(ctx, obj, C.grn_id(id), &grnVector)
	case Int16:
		ok = C.grngo_column_set_int16_vector(ctx, obj, C.grn_id(id), &grnVector)
	case Int32:
		ok = C.grngo_column_set_int32_vector(ctx, obj, C.grn_id(id), &grnVector)
	case Int64:
		ok = C.grngo_column_set_int64_vector(ctx, obj, C.grn_id(id), &grnVector)
	case UInt8:
		ok = C.grngo_column_set_uint8_vector(ctx, obj, C.grn_id(id), &grnVector)
	case UInt16:
		ok = C.grngo_column_set_uint16_vector(ctx, obj, C.grn_id(id), &grnVector)
	case UInt32:
		ok = C.grngo_column_set_uint32_vector(ctx, obj, C.grn_id(id), &grnVector)
	case UInt64:
		ok = C.grngo_column_set_uint64_vector(ctx, obj, C.grn_id(id), &grnVector)
	default:
		return fmt.Errorf("value type conflict")
	}
	if ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_int*_vector() failed")
	}
	return nil
}

// setFloatVector() assigns a Float vector.
func (column *Column) setFloatVector(id uint32, value []float64) error {
	var grnVector C.grngo_vector
	if len(value) != 0 {
		grnVector.ptr = unsafe.Pointer(&value[0])
		grnVector.size = C.size_t(len(value))
	}
	if ok := C.grngo_column_set_float_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_float_vector() failed")
	}
	return nil
}

// setGeoPointVector() assigns a GeoPoint vector.
func (column *Column) setGeoPointVector(id uint32, value []GeoPoint) error {
	var grnVector C.grngo_vector
	if len(value) != 0 {
		grnVector.ptr = unsafe.Pointer(&value[0])
		grnVector.size = C.size_t(len(value))
	}
	if ok := C.grngo_column_set_geo_point_vector(column.table.db.ctx,
		column.obj, C.grn_builtin_type(column.valueType),
		C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_geo_point_vector() failed")
	}
	return nil
}

// setTextVector() assigns a Text vector.
func (column *Column) setTextVector(id uint32, value [][]byte) error {
	grnValue := make([]C.grngo_text, len(value))
	for i, v := range value {
		if len(v) != 0 {
			grnValue[i].ptr = (*C.char)(unsafe.Pointer(&v[0]))
			grnValue[i].size = C.size_t(len(v))
		}
	}
	var grnVector C.grngo_vector
	if len(grnValue) != 0 {
		grnVector.ptr = unsafe.Pointer(&grnValue[0])
		grnVector.size = C.size_t(len(grnValue))
	}
	if ok := C.grngo_column_set_text_vector(column.table.db.ctx,
		column.obj, C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return fmt.Errorf("grngo_column_set_text_vector() failed")
	}
	return nil
}

// SetValue() assigns a value.
func (column *Column) SetValue(id uint32, value interface{}) error {
	switch v := value.(type) {
	case bool:
		return column.setBool(id, v)
	case int64:
		return column.setInt(id, v)
	case float64:
		return column.setFloat(id, v)
	case GeoPoint:
		return column.setGeoPoint(id, v)
	case []byte:
		return column.setText(id, v)
	case []bool:
		return column.setBoolVector(id, v)
	case []int64:
		return column.setIntVector(id, v)
	case []float64:
		return column.setFloatVector(id, v)
	case []GeoPoint:
		return column.setGeoPointVector(id, v)
	case [][]byte:
		return column.setTextVector(id, v)
	default:
		return fmt.Errorf("unsupported value type: name = <%s>",
			reflect.TypeOf(value).Name())
	}
}

// getBool() gets a Bool value.
func (column *Column) getBool(id uint32) (interface{}, error) {
	var grnValue C.grn_bool
	if ok := C.grngo_column_get_bool(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_bool() failed")
	}
	return grnValue == C.GRN_TRUE, nil
}

// getInt() gets an Int value.
func (column *Column) getInt(id uint32) (interface{}, error) {
	var grnValue C.int64_t
	if ok := C.grngo_column_get_int(column.table.db.ctx, column.obj,
		C.grn_builtin_type(column.valueType),
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_int() failed")
	}
	return int64(grnValue), nil
}

// getFloat() gets a Float value.
func (column *Column) getFloat(id uint32) (interface{}, error) {
	var grnValue C.double
	if ok := C.grngo_column_get_float(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_float() failed")
	}
	return float64(grnValue), nil
}

// getGeoPoint() gets a GeoPoint value.
func (column *Column) getGeoPoint(id uint32) (interface{}, error) {
	var grnValue C.grn_geo_point
	if ok := C.grngo_column_get_geo_point(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_geo_point() failed")
	}
	return GeoPoint{int32(grnValue.latitude), int32(grnValue.longitude)}, nil
}

// getText() gets a Text value.
func (column *Column) getText(id uint32) (interface{}, error) {
	var grnValue C.grngo_text
	if ok := C.grngo_column_get_text(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_text() failed")
	}
	if grnValue.size == 0 {
		return make([]byte, 0), nil
	}
	value := make([]byte, int(grnValue.size))
	grnValue.ptr = (*C.char)(unsafe.Pointer(&value[0]))
	if ok := C.grngo_column_get_text(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_text() failed")
	}
	return value, nil
}

// getBoolVector() gets a BoolVector.
func (column *Column) getBoolVector(id uint32) (interface{}, error) {
	var grnVector C.grngo_vector
	if ok := C.grngo_column_get_bool_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_bool_vector() failed")
	}
	if grnVector.size == 0 {
		return make([]bool, 0), nil
	}
	grnValue := make([]C.grn_bool, int(grnVector.size))
	grnVector.ptr = unsafe.Pointer(&grnValue[0])
	if ok := C.grngo_column_get_bool_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_bool_vector() failed")
	}
	value := make([]bool, int(grnVector.size))
	for i, v := range grnValue {
		value[i] = (v == C.GRN_TRUE)
	}
	return value, nil
}

// getIntVector() gets a IntVector.
func (column *Column) getIntVector(id uint32) (interface{}, error) {
	var grnValue C.grngo_vector
	if ok := C.grngo_column_get_int_vector(column.table.db.ctx, column.obj,
		C.grn_builtin_type(column.valueType),
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_int_vector() failed")
	}
	if grnValue.size == 0 {
		return make([]int64, 0), nil
	}
	value := make([]int64, int(grnValue.size))
	grnValue.ptr = unsafe.Pointer(&value[0])
	if ok := C.grngo_column_get_int_vector(column.table.db.ctx, column.obj,
		C.grn_builtin_type(column.valueType),
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_int_vector() failed")
	}
	return value, nil
}

// getFloatVector() gets a FloatVector.
func (column *Column) getFloatVector(id uint32) (interface{}, error) {
	var grnValue C.grngo_vector
	if ok := C.grngo_column_get_float_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_float_vector() failed")
	}
	if grnValue.size == 0 {
		return make([]float64, 0), nil
	}
	value := make([]float64, int(grnValue.size))
	grnValue.ptr = unsafe.Pointer(&value[0])
	if ok := C.grngo_column_get_float_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_float_vector() failed")
	}
	return value, nil
}

// getGeoPointVector() gets a GeoPointVector.
func (column *Column) getGeoPointVector(id uint32) (interface{}, error) {
	var grnValue C.grngo_vector
	if ok := C.grngo_column_get_geo_point_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_geo_point_vector() failed")
	}
	if grnValue.size == 0 {
		return make([]GeoPoint, 0), nil
	}
	value := make([]GeoPoint, int(grnValue.size))
	grnValue.ptr = unsafe.Pointer(&value[0])
	if ok := C.grngo_column_get_geo_point_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnValue); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_geo_point_vector() failed")
	}
	return value, nil
}

// getTextVector() gets a TextVector.
func (column *Column) getTextVector(id uint32) (interface{}, error) {
	var grnVector C.grngo_vector
	if ok := C.grngo_column_get_text_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_text_vector() failed")
	}
	if grnVector.size == 0 {
		return make([][]byte, 0), nil
	}
	grnValues := make([]C.grngo_text, int(grnVector.size))
	grnVector.ptr = unsafe.Pointer(&grnValues[0])
	if ok := C.grngo_column_get_text_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_text_vector() failed")
	}
	value := make([][]byte, int(grnVector.size))
	for i, grnValue := range grnValues {
		if grnValue.size != 0 {
			value[i] = make([]byte, int(grnValue.size))
			grnValues[i].ptr = (*C.char)(unsafe.Pointer(&value[i][0]))
		}
	}
	if ok := C.grngo_column_get_text_vector(column.table.db.ctx, column.obj,
		C.grn_id(id), &grnVector); ok != C.GRN_TRUE {
		return nil, fmt.Errorf("grngo_column_get_text_vector() failed")
	}
	return value, nil
}

// GetValue() gets a value.
func (column *Column) GetValue(id uint32) (interface{}, error) {
	if !column.isVector {
		switch column.valueType {
		case Bool:
			return column.getBool(id)
		case Int8, Int16, Int32, Int64, UInt8, UInt16, UInt32, UInt64:
			return column.getInt(id)
		case Float:
			return column.getFloat(id)
		case ShortText, Text, LongText:
			return column.getText(id)
		case TokyoGeoPoint, WGS84GeoPoint:
			return column.getGeoPoint(id)
		}
	} else {
		switch column.valueType {
		case Bool:
			return column.getBoolVector(id)
		case Int8, Int16, Int32, Int64, UInt8, UInt16, UInt32, UInt64:
			return column.getIntVector(id)
		case Float:
			return column.getFloatVector(id)
		case ShortText, Text, LongText:
			return column.getTextVector(id)
		case TokyoGeoPoint, WGS84GeoPoint:
			return column.getGeoPointVector(id)
		}
	}
	return nil, fmt.Errorf("undefined value type: valueType = %d", column.valueType)
}
