package resource

import (
	"fmt"
	daptinid "github.com/daptin/daptin/server/id"

	//sq  "github.com/Masterminds/squirrel"
	//"github.com/jmoiron/sqlx"
	//log "github.com/sirupsen/logrus"
	//"database/sql"
	//"reflect"
	//uuid "github.com/google/uuid"
	//"github.com/artpar/reflect"
	//"time"
	"github.com/jmoiron/sqlx"
	"reflect"
)

type StatusResponse struct {
	Message string
}

/*
*

	using a map
*/
type mapStringScan struct {
	// cp are the column pointers
	cp []interface{}
	// row contains the final result
	row      map[string]interface{}
	colCount int
	colNames []string
}

func NewMapStringScan(columnNames []string) *mapStringScan {
	lenCN := len(columnNames)
	s := &mapStringScan{
		cp:       make([]interface{}, lenCN),
		row:      make(map[string]interface{}, lenCN),
		colCount: lenCN,
		colNames: columnNames,
	}
	for i := 0; i < lenCN; i++ {
		i2 := new(interface{})
		s.cp[i] = i2
	}
	return s
}

func ValueOf(x interface{}) interface{} {
	v := reflect.ValueOf(reflect.ValueOf(x).Elem().Interface())
	var finalValue interface{}
	switch v.Kind() {
	case reflect.Bool:
		//fmt.Printf("bool: %v\n", v.Bool())
		finalValue = v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int32, reflect.Int64:
		finalValue = v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint32, reflect.Uint64:
		//fmt.Printf("int: %v\n", v.Uint())
		finalValue = v.Uint()
	case reflect.Float32, reflect.Float64:
		//fmt.Printf("float: %v\n", v.Float())
		finalValue = v.Float()
	case reflect.String:
		//fmt.Printf("string: %v\n", v.String())
		finalValue = v.String()
	case reflect.Slice:
		//fmt.Printf("slice: len=%d, %v\n", v.Len(), v.Interface())
		finalValue = string(v.Interface().([]uint8))
	case reflect.Map:
		//fmt.Printf("map: %v\n", v.Interface())
		finalValue = v.Interface()
	case reflect.Chan:
		//fmt.Printf("chan %v\n", v.Interface())
		finalValue = v.Interface()
	default:
		finalValue = reflect.ValueOf(x).Elem().Interface()
		//fmt.Println(reflect.ValueOf(x).Elem().Interface())
	}

	return finalValue
}

func (s *mapStringScan) Update(rows *sqlx.Rows) error {

	if err := rows.Scan(s.cp...); err != nil {
		return err
	}

	for i := 0; i < s.colCount; i++ {
		rb := s.cp[i]
		if true {
			s.row[s.colNames[i]] = ValueOf(rb)
			if s.colNames[i] == "reference_id" || EndsWithCheck(s.colNames[i], "_reference_id") {
				s.row[s.colNames[i]] = daptinid.DaptinReferenceId([]byte(s.row[s.colNames[i]].(string)))
			}
			rb = nil // reset pointer to discard current value to avoid a bug
		} else {
			t := s.cp[i]
			return fmt.Errorf("Cannot convert index %d column [%s] to type *sql.RawBytes from [%v]", i, s.colNames[i], t)
		}
	}
	return nil
}

func (s *mapStringScan) Get() map[string]interface{} {
	return s.row
}
