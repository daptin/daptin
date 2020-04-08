package server

import (
	"errors"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	"sort"
	"strings"
)

type DatabaseColumn struct {
}

type DatabaseTable struct {
	TableName string
	Columns   []DatabaseColumn
}

type DatabaseIndex struct {
}

type DatabaseSchema struct {
	Tables        []resource.TableInfo
	Relations     []api2go.TableRelation
	Indexes       map[string]string
	CompositeKeys [][]string
}

type PostgresColStructure struct {
	TableName      string `db:"table_name"`
	ColumnName     string `db:"column_nane"`
	ColumnType     string `db:"column_type"`
	ColumnFullType string `db:"column_full_type"`
	UdtName        string `db:"udt_name"`
	ColumnDefault  string `db:"column_default"`
	IsNullable     bool   `db:"is_nullable"`
	IsIdentity     bool   `db:"is_identity"`
	IsUnique       bool   `db:"is_unique"`
}

type PostgresRelationStructure struct {
	KeyName           string
	SourceTable       string
	SourceColumn      string
	DestinationTable  string
	DestinationColumn string
}

func GetCurrentDbSchema(connection *sqlx.DB, databaseName string) (*DatabaseSchema, error) {

	//tables := make([]DatabaseTable, 0)
	tableMap := make(map[string]*resource.TableInfo)
	relationsMap := make(map[string]*resource.TableRelation)

	switch connection.DriverName() {
	case "sqlite3":
	case "mysql":

	case "postgres":

		colQuery := `select
       c.table_name, c.column_name,
       ct.column_type,
       (
           case when c.character_maximum_length != 0
                     then
               (
                   ct.column_type || '(' || c.character_maximum_length || ')'
                   )
                else c.udt_name
               end
           ) as column_full_type,
       c.udt_name,
       e.data_type as array_type,
       c.domain_name,
       c.column_default,
       c.is_nullable = 'YES' as is_nullable,
       (case
          when (select
                       case
                         when column_name = 'is_identity' then (select c.is_identity = 'YES' as is_identity)
                         else
                           false
                           end as is_identity from information_schema.columns
                WHERE table_schema='information_schema' and table_name='columns' and column_name='is_identity') IS NULL then 'NO' else is_identity end) = 'YES' as is_identity,
       (select exists(
                 select 1
                 from information_schema.table_constraints tc
                        inner join information_schema.constraint_column_usage as ccu on tc.constraint_name = ccu.constraint_name
                 where tc.table_schema = 'public' and tc.constraint_type = 'UNIQUE' and ccu.constraint_schema = 'public' and ccu.table_name = c.table_name and ccu.column_name = c.column_name and
                       (select count(*) from information_schema.constraint_column_usage where constraint_schema = 'public' and constraint_name = tc.constraint_name) = 1
                   )) OR
       (select exists(
                 select 1
                 from pg_indexes pgix
                        inner join pg_class pgc on pgix.indexname = pgc.relname and pgc.relkind = 'i' and pgc.relnatts = 1
                        inner join pg_index pgi on pgi.indexrelid = pgc.oid
                        inner join pg_attribute pga on pga.attrelid = pgi.indrelid and pga.attnum = ANY(pgi.indkey)
                 where
                     pgix.schemaname = 'public' and pgix.tablename = c.table_name and pga.attname = c.column_name and pgi.indisunique = true
                   )) as is_unique
from information_schema.columns as c
       inner join pg_namespace as pgn on pgn.nspname = c.udt_schema
       left join pg_type pgt on c.data_type = 'USER-DEFINED' and pgn.oid = pgt.typnamespace and c.udt_name = pgt.typname
       left join information_schema.element_types e
         on ((c.table_catalog, c.table_schema, c.table_name, 'TABLE', c.dtd_identifier)
               = (e.object_catalog, e.object_schema, e.object_name, e.object_type, e.collection_type_identifier)),
     lateral (select
                     (
                         case when pgt.typtype = 'e'
                                   then
                             (
                             select 'enum.' || c.udt_name || '(''' || string_agg(labels.label, ''',''') || ''')'
                             from (
                                  select pg_enum.enumlabel as label
                                  from pg_enum
                                  where pg_enum.enumtypid =
                                        (
                                        select typelem
                                        from pg_type
                                        where pg_type.typtype = 'b' and pg_type.typname = ('_' || c.udt_name)
                                        limit 1
                                        )
                                  order by pg_enum.enumsortorder
                                  ) as labels
                             )
                              else c.data_type
                             end
                         ) as column_type
         ) ct
where c.table_schema = 'public'`

		rows, err := connection.Queryx(colQuery)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var postgresColStructure PostgresColStructure
			err = rows.Scan(&postgresColStructure)
			if err != nil {
				return nil, err
			}
			tableName := postgresColStructure.TableName
			tableInfo, ok := tableMap[tableName]
			if !ok {
				tableInfo = &resource.TableInfo{
					TableName: tableName,
					Columns:   []api2go.ColumnInfo{},
				}
			}

			colTypeParts := strings.Split(postgresColStructure.ColumnFullType, " ")
			tableInfo.Columns = append(tableInfo.Columns, api2go.ColumnInfo{
				ColumnName:   postgresColStructure.ColumnName,
				ColumnType:   DataTypeToColumnType(postgresColStructure.ColumnFullType),
				DataType:     colTypeParts[len(colTypeParts)-1],
				IsNullable:   postgresColStructure.IsNullable,
				IsUnique:     postgresColStructure.IsUnique,
				DefaultValue: postgresColStructure.ColumnDefault,
				IsPrimaryKey: postgresColStructure.IsIdentity,
			})

		}

		relationsQuery := `
select
		pgcon.conname,
		pgc.relname as source_table,
		pgasrc.attname as source_column,
		dstlookupname.relname as dest_table,
		pgadst.attname as dest_column
	from pg_namespace pgn
		inner join pg_class pgc on pgn.oid = pgc.relnamespace and pgc.relkind = 'r'
		inner join pg_constraint pgcon on pgn.oid = pgcon.connamespace and pgc.oid = pgcon.conrelid
		inner join pg_class dstlookupname on pgcon.confrelid = dstlookupname.oid
		inner join pg_attribute pgasrc on pgc.oid = pgasrc.attrelid and pgasrc.attnum = ANY(pgcon.conkey)
		inner join pg_attribute pgadst on pgcon.confrelid = pgadst.attrelid and pgadst.attnum = ANY(pgcon.confkey)
	where pgn.nspname = $2 and pgc.relname = $1 and pgcon.contype = 'f'
	order by pgcon.conname, source_table, source_column, dest_table, dest_column`

		relationsRes, err := connection.Queryx(relationsQuery, "public")
		if err != nil {
			return nil, err
		}

		for relationsRes.Next() {
			postgresRelationStructre := PostgresRelationStructure{}
			relationsRes.StructScan(&postgresRelationStructre)

			r := api2go.NewTableRelationWithNames(
				postgresRelationStructre.SourceColumn, postgresRelationStructre.SourceTable,
				"has_one",
				postgresRelationStructre.DestinationTable, postgresRelationStructre.DestinationColumn,
			)

			relationsMap[postgresRelationStructre.KeyName] = &resource.TableRelation{r, "cascade"}
		}

	default:
		return nil, errors.New(fmt.Sprintf("unknown database type [%v]", connection.DriverName()))
	}

	return &DatabaseSchema{
	}, nil
}

func DataTypeToColumnType(postgresDataType string) string {
	for _, colType := range resource.ColumnTypes {
		i := sort.SearchStrings(colType.DataTypes, postgresDataType)
		if i > -1 {
			return colType.Name
		}
	}
	return ""
}
