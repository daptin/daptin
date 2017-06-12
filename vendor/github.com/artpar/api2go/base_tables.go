package api2go

import (
  "fmt"
  "github.com/satori/go.uuid"
  log "github.com/Sirupsen/logrus"
  underscore "github.com/ahl5esoft/golang-underscore"
  "strings"
  "github.com/artpar/api2go/jsonapi"
  "errors"
)

type TableRelationInterface interface {
  GetSubjectName() string
  GetRelation() string
  GetObjectName() string
}

type TableRelation struct {
  Subject     string
  Object      string
  Relation    string
  SubjectName string
  ObjectName  string
}

func (tr *TableRelation) String() string {
  return fmt.Sprintf("[TableRelation] [%v][%v][%v]", tr.GetSubjectName(), tr.GetRelation(), tr.GetObjectName())
}

func (tr *TableRelation) GetSubjectName() string {
  if tr.SubjectName == "" {
    tr.SubjectName = tr.Subject + "_id"
  }
  return tr.SubjectName
}

func (tr *TableRelation) GetSubject() string {
  return tr.Subject
}

func (tr *TableRelation) GetJoinTableName() string {
  return tr.Subject + "_" + tr.GetSubjectName() + "_has_" + tr.Object + "_" + tr.GetObjectName()
}

func (tr *TableRelation) GetJoinString() string {

  if tr.Relation == "has_one" {
    return fmt.Sprintf(" %s on %s.%s = %s.%s ", tr.GetObject(), tr.GetSubject(), tr.GetObjectName(), tr.GetObject(), "id")
  } else if tr.Relation == "belongs_to" {
    return fmt.Sprintf(" %s on %s.%s = %s.%s ", tr.GetObject(), tr.GetSubject(), tr.GetObjectName(), tr.GetObject(), "id")
  } else if tr.Relation == "has_many" || tr.Relation == "has_many_and_belongs_to_many" {
    return fmt.Sprintf(" %s j1 on      j1.%s = %s.id             join %s  on  j1.%s = %s.%s ",
      tr.GetJoinTableName(), tr.SubjectName, tr.GetSubject(), tr.GetObject(), tr.GetObjectName(), tr.GetObject(), "id")
  } else {
    log.Errorf("Not implemented join: %v", tr)
  }

  return ""
}

func (tr *TableRelation) GetReverseJoinString() string {

  if tr.Relation == "has_one" {
    return fmt.Sprintf(" %s on %s.%s = %s.%s ", tr.GetSubject(), tr.GetSubject(), tr.GetObjectName(), tr.GetObject(), "id")
  } else if tr.Relation == "belongs_to" {
    return fmt.Sprintf(" %s on %s.%s = %s.%s ", tr.GetSubject(), tr.GetSubject(), tr.GetObjectName(), tr.GetObject(), "id")
  } else if tr.Relation == "has_many" {

    //select * from user join user_has_usergroup j1 on j1.user_id = user.id  join usergroup on j1.usergroup_id = usergroup.id
    return fmt.Sprintf(" %s j1 on j1.%s = %s.id join %s on j1.%s = %s.%s ",
      tr.GetJoinTableName(), tr.GetObjectName(), tr.GetObject(), tr.GetSubject(), tr.GetSubjectName(), tr.GetSubject(), "id")
  } else {
    log.Errorf("Not implemented join: %v", tr)
  }

  return ""
}

func (tr *TableRelation) GetRelation() string {
  return tr.Relation
}

func (tr *TableRelation) GetObjectName() string {
  if tr.ObjectName == "" {
    tr.ObjectName = tr.Object + "_id"
  }
  return tr.ObjectName
}

func (tr *TableRelation) GetObject() string {
  return tr.Object
}

func NewTableRelation(subject, relation, object string) TableRelation {
  return TableRelation{
    Subject:     subject,
    Relation:    relation,
    Object:      object,
    SubjectName: subject + "_id",
    ObjectName:  object + "_id",
  }
}

func NewTableRelationWithNames(subject, subjectName, relation, object, objectName string) *TableRelation {
  return &TableRelation{
    Subject:     subject,
    Relation:    relation,
    Object:      object,
    SubjectName: subjectName,
    ObjectName:  objectName,
  }
}

type Api2GoModel struct {
  typeName          string
  columns           []ColumnInfo
  columnMap         map[string]ColumnInfo
  defaultPermission int64
  relations         []TableRelation
  Data              map[string]interface{}
  Includes          []jsonapi.MarshalIdentifier
}

func (a *Api2GoModel) GetColumnMap() map[string]ColumnInfo {
  if a.columnMap != nil && len(a.columnMap) > 0 {
    return a.columnMap
  }

  m := make(map[string]ColumnInfo)

  for _, col := range a.columns {
    m[col.ColumnName] = col
  }
  a.columnMap = m
  return m
}

func (a *Api2GoModel) HasColumn(colName string) bool {
  for _, col := range a.columns {
    if col.ColumnName == colName {
      return true
    }
  }

  for _, rel := range a.relations {

    if rel.GetRelation() == "belongs_to" && rel.GetObjectName() == colName {
      return true
    }

  }
  return false
}

func (a *Api2GoModel) HasMany(colName string) bool {

  if a.typeName == "usergroup" {
    return false
  }

  for _, rel := range a.relations {
    if rel.GetRelation() == "has_many" && rel.GetObject() == colName {
      //log.Infof("Found %v relation: %v", colName, rel)
      return true
    }
  }
  return false
}

func (a *Api2GoModel) GetRelations() []TableRelation {
  return a.relations
}

type ColumnInfo struct {
  Name            string `db:"name"`
  ColumnName      string `db:"column_name"`
  ColumnType      string `db:"column_type"`
  IsPrimaryKey    bool   `db:"is_primary_key"`
  IsAutoIncrement bool   `db:"is_auto_increment"`
  IsIndexed       bool   `db:"is_indexed"`
  IsUnique        bool   `db:"is_unique"`
  IsNullable      bool   `db:"is_nullable"`
  Permission      uint64   `db:"permission"`
  IsForeignKey    bool   `db:"is_foreign_key"`
  ExcludeFromApi  bool   `db:"include_in_api"`
  ForeignKeyData  ForeignKeyData   `db:"foreign_key_data"`
  DataType        string   `db:"data_type"`
  DefaultValue    string   `db:"default_value"`
}

type ForeignKeyData struct {
  TableName  string
  ColumnName string
}

func (f *ForeignKeyData) Scan(src interface{}) error {
  strValue, ok := src.([]uint8)
  if !ok {
    return fmt.Errorf("metas field must be a string, got %T instead", src)
  }

  parts := strings.Split(string(strValue), "(")
  tableName := parts[0]
  columnName := strings.Split(parts[1], ")")[0]

  f.TableName = tableName
  f.ColumnName = columnName
  return nil
}

func (f ForeignKeyData) String() string {
  return fmt.Sprintf("%s(%s)", f.TableName, f.ColumnName)
}

func NewApi2GoModelWithData(name string, columns []ColumnInfo, defaultPermission int64, relations []TableRelation, m map[string]interface{}) *Api2GoModel {
  return &Api2GoModel{
    typeName:          name,
    columns:           columns,
    relations:         relations,
    Data:              m,
    defaultPermission: defaultPermission,
  }
}
func NewApi2GoModel(name string, columns []ColumnInfo, defaultPermission int64, relations []TableRelation) *Api2GoModel {
  //fmt.Printf("New columns: %v", columns)
  return &Api2GoModel{
    typeName:          name,
    defaultPermission: defaultPermission,
    relations:         relations,
    columns:           columns,
  }
}

func EndsWith(str string, endsWith string) (string, bool) {
  if len(endsWith) > len(str) {
    return "", false
  }

  if len(endsWith) == len(str) && endsWith != str {
    return "", false
  }

  suffix := str[len(str)-len(endsWith):]
  prefix := str[:len(str)-len(endsWith)]

  i := suffix == endsWith
  return prefix, i

}

func EndsWithCheck(str string, endsWith string) (bool) {
  if len(endsWith) > len(str) {
    return false
  }

  if len(endsWith) == len(str) && endsWith != str {
    return false
  }

  suffix := str[len(str)-len(endsWith):]
  i := suffix == endsWith
  return i

}

func (m *Api2GoModel) SetToOneReferenceID(name, ID string) error {

  m.Data[name] = ID
  return nil

  return errors.New("There is no to-one relationship with the name " + name)
}

func (m *Api2GoModel) SetToManyReferenceIDs(name string, IDs []string) error {

  for _, rel := range m.relations {
    log.Infof("Check relation: %v", rel.String())
    if rel.GetRelation() != "belongs_to" {

      if rel.GetObjectName() == name || rel.GetSubjectName() == name {
        var rows = make([]map[string]interface{}, 0)
        for _, id := range IDs {
          row := make(map[string]interface{})
          row[name] = id
          row[rel.GetSubjectName()] = m.Data["reference_id"]
          rows = append(rows, row)
        }
        m.Data[name] = rows
        return nil
      }
    }
  }

  return nil

}

func (m *Api2GoModel) AddToManyIDs(name string, IDs []string) error {

  new1 := errors.New("There is no to-manyrelationship with the name " + name)
  log.Errorf("ERROR: ", new1)
  return new1
}

func (m *Api2GoModel) DeleteToManyIDs(name string, IDs []string) error {
  new1 := errors.New("There is no to-manyrelationship with the name " + name)
  log.Errorf("ERROR: ", new1)
  return new1
}

func (m *Api2GoModel) GetReferencedStructs() []jsonapi.MarshalIdentifier {
  //log.Infof("References : %v", m.Includes)
  return m.Includes
}

func (m *Api2GoModel) GetReferencedIDs() []jsonapi.ReferenceID {

  references := make([]jsonapi.ReferenceID, 0)

  for _, rel := range m.relations {

    log.Infof("Checked relations [%v]: %v", m.typeName, rel)

    if rel.GetRelation() == "belongs_to" {
      if rel.GetSubject() == m.typeName {

        val, ok := m.Data[rel.GetObjectName()]
        if !ok || val == nil {
          continue
        }

        ref := jsonapi.ReferenceID{
          Type:         rel.GetObject(),
          Name:         rel.GetObjectName(),
          ID:           m.Data[rel.GetObjectName()].(string),
          Relationship: jsonapi.DefaultRelationship,
        }
        references = append(references, ref)
      }
    }

  }
  //
  //for _, col := range *m.columns {
  //
  //  if col.ColumnName == "reference_id" {
  //    continue
  //  }
  //
  //  pref, ok := EndsWith(col.ColumnName, "_id")
  //  log.Infof("Checked column [%v]: %v == %v", col.ColumnName, ok, pref)
  //  if ok {
  //    ref := jsonapi.ReferenceID{
  //      Type: pref,
  //      Name: pref,
  //      ID: m.Data[col.ColumnName].(string),
  //      Relationship: jsonapi.DefaultRelationship,
  //    }
  //    references = append(references, ref)
  //  }
  //}
  //log.Infof("Reference ids for %v: %v", m.typeName, references)
  return references

}

func (model *Api2GoModel) GetReferences() []jsonapi.Reference {

  references := make([]jsonapi.Reference, 0)
  //

  //log.Infof("Relations: %v", model.relations)
  for _, relation := range model.relations {

    //log.Infof("Check relation [%v] On [%v]", model.typeName, relation.String())
    ref := jsonapi.Reference{}

    if relation.GetSubject() == model.typeName {
      switch relation.GetRelation() {

      case "has_many":
        ref.Type = relation.GetObject()
        ref.Name = relation.GetObjectName()
        ref.Relationship = jsonapi.ToManyRelationship
      case "has_one":
        ref.Type = relation.GetObject()
        ref.Name = relation.GetObjectName()
        ref.Relationship = jsonapi.ToOneRelationship

      case "belongs_to":
        ref.Type = relation.GetObject()
        ref.Name = relation.GetObjectName()
        ref.Relationship = jsonapi.ToOneRelationship
      case "has_many_and_belongs_to_many":
        ref.Type = relation.GetObject()
        ref.Name = relation.GetObjectName()
        ref.Relationship = jsonapi.ToManyRelationship
      default:
        log.Errorf("Unknown type of relation: %v", relation.GetRelation())
      }

    } else {
      switch relation.GetRelation() {

      case "has_many":
        ref.Type = relation.GetSubject()
        ref.Name = relation.GetSubjectName()
        ref.Relationship = jsonapi.ToManyRelationship
      case "has_one":
        ref.Type = relation.GetSubject()
        ref.Name = relation.GetSubjectName()
        ref.Relationship = jsonapi.ToOneRelationship

      case "belongs_to":
        ref.Type = relation.GetSubject()
        ref.Name = relation.GetSubjectName()
        ref.Relationship = jsonapi.ToManyRelationship
      case "has_many_and_belongs_to_many":
        ref.Type = relation.GetSubject()
        ref.Name = relation.GetSubjectName()
        ref.Relationship = jsonapi.ToManyRelationship
      default:
        log.Errorf("Unknown type of relation: %v", relation.GetRelation())
      }
    }

    references = append(references, ref)
  }

  return references
}

func (m *Api2GoModel) GetAttributes() map[string]interface{} {
  attrs := make(map[string]interface{})
  colMap := m.GetColumnMap()

  //log.Infof("Column Map for [%v]: %v", colMap["reference_id"])
  for k, v := range m.Data {

    if colMap[k].IsForeignKey {
      continue
    }

    if colMap[k].ExcludeFromApi {
      continue
    }

    attrs[k] = v
  }
  return attrs
}

func (m *Api2GoModel) GetAllAsAttributes() map[string]interface{} {
  attrs := make(map[string]interface{})
  for k, v := range m.Data {
    attrs[k] = v
  }
  return attrs
}

func (m *Api2GoModel) InitializeObject(interface{}) {
  log.Infof("initialize object: %v", m)
  m.Data = make(map[string]interface{})
}

func (m *Api2GoModel) SetColumns(c []ColumnInfo) {
  m.columns = c

}

func (m *Api2GoModel) GetColumns() []ColumnInfo {
  return m.columns
}

func (m *Api2GoModel) GetColumnNames() []string {

  v := underscore.Map(m.columns, func(s ColumnInfo, _ int) string {
    //log.Infof("Columna name for [%v] == %v]", s.Name, s.ColumnName)
    return s.ColumnName
  })
  res, _ := v.([]string)

  return res
}

func (g Api2GoModel) GetDefaultPermission() int64 {
  //log.Infof("default permission for %v is %v", g.typeName, g.defaultPermission)
  return g.defaultPermission
}

func (g Api2GoModel) GetName() string {
  return g.typeName
}

func (g Api2GoModel) GetTableName() string {
  return g.typeName
}

func (g *Api2GoModel) GetID() string {
  return fmt.Sprintf("%v", g.Data["reference_id"])
}

func (g *Api2GoModel) SetAttributes(attrs map[string]interface{}) {
  log.Infof("set attributes: %v", attrs)
  if g.Data == nil {
    g.Data = attrs
    return
  }
  for k, v := range attrs {
    g.Data[k] = v
  }

}

func (g *Api2GoModel) SetID(str string) error {
  log.Infof("set id: %v", str)
  if g.Data == nil {
    g.Data = make(map[string]interface{})
  }
  g.Data["reference_id"] = str
  return nil
}

type HasId interface {
  GetId() interface{}
}

func (g *Api2GoModel) GetReferenceId() string {
  return fmt.Sprintf("%v", g.Data["reference_id"])
}

func (g *Api2GoModel) BeforeCreate() (err error) {
  g.Data["reference_id"] = uuid.NewV4().String()
  return nil
}
