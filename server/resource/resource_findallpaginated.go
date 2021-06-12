package resource

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"strconv"
	"strings"

	"github.com/daptin/daptin/server/auth"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"

	"encoding/base64"

	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/statementbuilder"
	log "github.com/sirupsen/logrus"
)

func (dr *DbResource) GetTotalCount() uint64 {
	s, v, err := statementbuilder.Squirrel.Select(goqu.L("count(*)")).From(dr.model.GetName()).ToSQL()
	if err != nil {
		log.Errorf("Failed to generate count query for %v: %v", dr.model.GetName(), err)
		return 0
	}

	var count uint64

	stmt1, err := dr.connection.Preparex(s)
	if err != nil {
		log.Errorf("[31] failed to prepare statment: %v", err)
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&count)
	CheckErr(err, "Failed to execute total count query [%s] [%v]", s, v)
	//log.Printf("Count: [%v] %v", dr.model.GetTableName(), count)
	return count
}

func (dr *DbResource) GetTotalCountBySelectBuilder(builder *goqu.SelectDataset) uint64 {

	s, v, err := builder.ToSQL()
	//log.Printf("Count query: %v == %v", s, v)
	if err != nil {
		log.Errorf("Failed to generate count query for %v: %v", dr.model.GetName(), err)
		return 0
	}

	var count uint64

	stmt1, err := dr.connection.Preparex(s)
	if err != nil {
		log.Errorf("[61] failed to prepare statment: %v", err)
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&count)
	if err != nil {
		log.Errorf("Failed to execute count query [%v] %v", s, err)
	}
	//log.Printf("Count: [%v] %v", dr.model.GetTableName(), count)
	return count
}

type PaginationData struct {
	PageNumber uint64
	PageSize   uint64
	TotalCount uint64
}

type Query struct {
	ColumnName string      `json:"column"`
	Operator   string      `json:"operator"`
	Value      interface{} `json:"value"`
}

type Group struct {
	ColumnName string `json:"column"`
	Order      string `json:"order"`
}

type join struct {
	table     exp.Expression
	condition exp.JoinCondition
}

type column struct {
	originalvalue interface{}
	reference     string
}

// PaginatedFindAll(req Request) (totalCount uint, response Responder, err error)
func (dr *DbResource) PaginatedFindAllWithoutFilters(req api2go.Request) ([]map[string]interface{}, [][]map[string]interface{}, *PaginationData, bool, error) {
	//log.Printf("Find all row by params: [%v]: %v", dr.model.GetName(), req.QueryParams)
	var err error

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}

	isAdmin := dr.IsAdmin(sessionUser.UserReferenceId)

	isRelatedGroupRequest := false // to switch permissions to the join table later in select query
	relatedTableName := ""
	if dr.model.GetName() == "usergroup" && len(req.QueryParams) > 2 {
		ok := false
		for key := range req.QueryParams {
			if relatedTableName, ok = EndsWith(key, "Name"); req.QueryParams[key][0] == "usergroup_id" && ok {
				isRelatedGroupRequest = true
				break
			}
		}
	}

	languagePreferences := make([]string, 0)
	if dr.tableInfo.TranslationsEnabled {
		prefs := req.PlainRequest.Context().Value("language_preference")
		if prefs != nil {
			languagePreferences = prefs.([]string)
		}
	}

	pageNumber := uint64(0)
	if len(req.QueryParams["page[number]"]) > 0 {
		pageNumber, err = strconv.ParseUint(req.QueryParams["page[number]"][0], 10, 32)
		if err != nil {
			log.Errorf("Invalid parameter value: %v", req.QueryParams["page[number]"])
		}
		pageNumber--
	}

	query, ok := req.QueryParams["query"]
	queries := make([]Query, 0)
	if ok {
		if len(query) > 1 {
			//api2go will split the values on comma to give array of values
			//so we join it back to read it as json
			query[0] = strings.Join(query, ",")
		}
		if len(query) > 0 && len(query[0]) > 0 && query[0][0] == '[' {
			//log.Printf("Found query in request: %s", query[0])
			err = json.Unmarshal([]byte(query[0]), &queries)
			if CheckInfo(err, "Failed to unmarshal query as json, using as a filter instead") {
				req.QueryParams["filter"] = query
			}
			//log.Printf("Query: %v", queries)
		}
	}

	groups, ok := req.QueryParams["group"]
	groupings := make([]Group, 0)
	if ok {
		queryS, err := base64.StdEncoding.DecodeString(groups[0])
		log.Printf("Found groups in request: %s", queryS)
		if err == nil {
			err = json.Unmarshal(queryS, &groupings)
			log.Printf("Groupings: %v", groupings)
		}
		InfoErr(err, fmt.Sprintf("Failed to read groups from request: %v", query[0]))
	}

	reqFieldMap := make(map[string]bool)
	requestedFields, hasRequestedFields := req.QueryParams["fields"]
	if hasRequestedFields {
		for _, f := range requestedFields {

			fieldNames := strings.Split(f, ",")
			for _, name := range fieldNames {
				reqFieldMap[name] = true
			}
		}
		reqFieldMap[USER_ACCOUNT_ID_COLUMN] = true
	}

	pageSize := uint64(10)
	if len(req.QueryParams["page[size]"]) > 0 {
		pageSize, err = strconv.ParseUint(req.QueryParams["page[size]"][0], 10, 32)
		if err != nil {
			log.Errorf("Invalid parameter value: %v", req.QueryParams["page[size]"])
		}
	}

	includedRelations := make(map[string]bool, 0)
	if len(req.QueryParams["included_relations"]) > 0 {
		//included := req.QueryParams["included_relations"][0]
		//includedRelationsList := strings.Split(included, ",")
		for _, incl := range req.QueryParams["included_relations"] {
			includedRelations[incl] = true
		}

	} else {
		includedRelations = nil
	}

	if pageSize == 0 {
		pageSize = 1
	}

	var sortOrder []string
	if len(req.QueryParams["sort"]) > 0 {
		sortOrder = req.QueryParams["sort"]
	} else if dr.tableInfo.DefaultOrder != "" && len(dr.tableInfo.DefaultOrder) > 2 {
		if dr.tableInfo.DefaultOrder[0] == '\'' || dr.tableInfo.DefaultOrder[0] == '"' {
			rep := strings.ReplaceAll(dr.tableInfo.DefaultOrder, "'", "\"")
			unquotedOrder, _ := strconv.Unquote(rep)
			dr.tableInfo.DefaultOrder = unquotedOrder
		}
		sortOrder = strings.Split(dr.tableInfo.DefaultOrder, ",")
	} else {
		sortOrder = []string{"-created_at"}
	}

	var filters []string

	if len(req.QueryParams["filter"]) > 0 && len(queries) == 0 {
		filters = req.QueryParams["filter"]

		//for i, q := range filters {
		//	unescaped, _ := url.QueryUnescape(q)
		//	filters[i] = unescaped
		//}
	}

	//filters := []string{}

	//if len(req.QueryParams["filter"]) > 0 {
	//	filters = req.QueryParams["filter"]
	//}

	if pageNumber > 0 {
		pageNumber = pageNumber * pageSize
	}

	tableModel := dr.model
	//log.Printf("Get all resource type: %v\n", tableModel)

	cols := tableModel.GetColumns()
	finalCols := make([]column, 0)
	//log.Printf("Cols: %v", cols)

	prefix := dr.model.GetName() + "."
	if hasRequestedFields {

		for _, col := range cols {
			if !col.ExcludeFromApi && reqFieldMap[col.ColumnName] && col.ColumnName != "permission" && col.ColumnName != "reference_id" {
				finalCols = append(finalCols, column{
					originalvalue: goqu.C(col.ColumnName),
					reference:     col.ColumnName,
				})
			}
		}
	} else {
		for _, col := range cols {
			if col.ExcludeFromApi || col.ColumnName == "permission" || col.ColumnName == "reference_id" || col.ColumnName == "id" {
				continue
			}
			finalCols = append(finalCols, column{
				originalvalue: goqu.C(col.ColumnName),
				reference:     col.ColumnName,
			})
		}
	}

	if _, ok := req.QueryParams["usergroup_id"]; ok && req.QueryParams["usergroupName"][0] == dr.model.GetName()+"_id" {
		isRelatedGroupRequest = true
		if relatedTableName == "" {
			relatedTableName = dr.model.GetTableName()
		}
	}

	idColumn := fmt.Sprintf("%s.id", tableModel.GetTableName())
	distinctIdColumn := goqu.L(fmt.Sprintf("distinct(%s.id)", tableModel.GetTableName()))
	if isRelatedGroupRequest {
		//log.Printf("Switch permission to join table j1 instead of %v%v", prefix, "permission")
		if dr.model.GetName() == "usergroup" {
			finalCols = append(finalCols, column{
				originalvalue: goqu.I(fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.permission", relatedTableName, relatedTableName)),
				reference:     fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.permission", relatedTableName, relatedTableName),
			})
			finalCols = append(finalCols, column{
				originalvalue: goqu.I(fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.reference_id", relatedTableName, relatedTableName)).As("reference_id"),
				reference:     fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.reference_id", relatedTableName, relatedTableName),
			})
			finalCols = append(finalCols,
				column{
					originalvalue: goqu.I("usergroup.reference_id").As("reference_id"),
					reference:     "usergroup.reference_id as relation_reference_id",
				},
			)
		} else {

			finalCols = append(finalCols, column{
				originalvalue: goqu.I(fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.permission", relatedTableName, relatedTableName)),
				reference:     fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.permission", relatedTableName, relatedTableName),
			})

			finalCols = append(finalCols, column{
				originalvalue: goqu.I(fmt.Sprintf("%s.reference_id", relatedTableName)).As("relation_reference_id"),
				reference:     fmt.Sprintf("%s.reference_id", relatedTableName),
			})
			finalCols = append(finalCols, column{
				originalvalue: goqu.I(fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.reference_id", relatedTableName, relatedTableName)).As("reference_id"),
				reference:     fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.reference_id", relatedTableName, relatedTableName),
			})
			joinTableName := fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id", relatedTableName, relatedTableName)
			distinctIdColumn = goqu.L(fmt.Sprintf("distinct(%s.id)", joinTableName))
			idColumn = fmt.Sprintf("%s.id", joinTableName)
		}
		//		finalCols = append(finalCols, prefix+"reference_id as reference_id")
	} else {
		finalCols = append(finalCols, column{
			originalvalue: goqu.I(prefix + "permission"),
			reference:     prefix + "permission",
		})
		finalCols = append(finalCols, column{
			originalvalue: goqu.I(prefix + "reference_id"),
			reference:     prefix + "reference_id",
		})

	}

	idQueryCols := []interface{}{distinctIdColumn}

	for _, sort := range sortOrder {

		if len(sort) == 0 {
			continue
		}

		if sort[0] == '-' || sort[0] == '+' {
			sort = sort[1:]
		}

		if strings.Index(sort, "(") == -1 {
			sort = prefix + sort
		}
		idQueryCols = append(idQueryCols, goqu.I(sort).As(strings.ReplaceAll(sort, ".", "_")))
	}
	queryBuilder := statementbuilder.Squirrel.Select(idQueryCols...).From(tableModel.GetTableName())
	//queryBuilder = queryBuilder.From(tableModel.GetTableName())
	var countQueryBuilder *goqu.SelectDataset
	countQueryBuilder = statementbuilder.Squirrel.Select(goqu.L(fmt.Sprintf("count(distinct(%v.id))", tableModel.GetTableName()))).From(tableModel.GetTableName()).Offset(0).Limit(1)

	joinTableName := fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id", tableModel.GetTableName(), tableModel.GetTableName())
	if !isRelatedGroupRequest && tableModel.GetTableName() != "usergroup" {

		countQueryBuilder = countQueryBuilder.LeftJoin(
			goqu.T(joinTableName).As(joinTableName),
			goqu.On(goqu.Ex{
				fmt.Sprintf("%s.id", tableModel.GetTableName()): goqu.I(fmt.Sprintf("%s.%s_id", joinTableName, tableModel.GetTableName())),
			},
			),
		)

		queryBuilder = queryBuilder.LeftJoin(
			goqu.T(joinTableName).As(joinTableName),
			goqu.On(goqu.Ex{
				fmt.Sprintf("%s.id", tableModel.GetTableName()): goqu.I(fmt.Sprintf("%s.%s_id", joinTableName, tableModel.GetTableName())),
			},
			),
		)

	}

	if req.QueryParams["page[after]"] != nil && len(req.QueryParams["page[after]"]) > 0 {
		id, err := dr.GetReferenceIdToId(dr.TableInfo().TableName, req.QueryParams["page[after]"][0])
		if err != nil {
			queryBuilder = queryBuilder.Where(goqu.Ex{
				dr.TableInfo().TableName + ".id": goqu.Op{"gt": id},
			}).Limit(uint(pageSize))
		}
	} else if req.QueryParams["page[before]"] != nil && len(req.QueryParams["page[before]"]) > 0 {
		id, err := dr.GetReferenceIdToId(dr.TableInfo().TableName, req.QueryParams["page[before]"][0])
		if err != nil {
			queryBuilder = queryBuilder.Where(goqu.Ex{
				dr.TableInfo().TableName + ".id": goqu.Op{"lt": id},
			}).Limit(uint(pageSize))
		}
	} else {
		queryBuilder = queryBuilder.Offset(uint(pageNumber)).Limit(uint(pageSize))
	}
	joins := make([]join, 0)
	joinFilters := make([]goqu.Ex, 0)

	infos := dr.model.GetColumns()

	// todo: fix search in findall operation. currently no way to do an " or " query
	if len(filters) > 0 {

		colsToAdd := make([]string, 0)

		for _, col := range infos {
			if col.IsIndexed && (col.ColumnType == "name" || col.ColumnType == "label" || col.ColumnType == "email") {
				colsToAdd = append(colsToAdd, col.ColumnName)
			}
		}

		if len(colsToAdd) > 0 {

			queryExpressions := make([]goqu.Expression, 0)

			for _, q := range filters {

				if len(q) < 1 {
					continue
				}

				for _, c := range colsToAdd {

					query := goqu.Ex{
						prefix + c: goqu.Op{"like": fmt.Sprintf("%s%s%s", "%", q, "%")},
					}
					queryExpressions = append(queryExpressions, query)
				}
			}

			if len(queryExpressions) > 0 {
				queryBuilder = queryBuilder.Where(goqu.Or(queryExpressions...))
				countQueryBuilder = countQueryBuilder.Where(goqu.Or(queryExpressions...))
				//queryBuilder = queryBuilder.Where("( "+strings.Join(colString, " or ")+")", wheres...)
				//countQueryBuilder = countQueryBuilder.Where("( "+strings.Join(colString, " or ")+")", wheres...)
			}

		}
	}

	queryBuilder, countQueryBuilder = dr.addFilters(queryBuilder, countQueryBuilder, queries, prefix)

	//if len(groupings) > 0 && false {
	//	for _, groupBy := range groupings {
	//		queryBuilder = queryBuilder.GroupBy(fmt.Sprintf("%s %s", groupBy.ColumnName, groupBy.Order))
	//	}
	//}

	// for relation api calls with has_one or belongs_to relations
	finalResponseIsSingleObject := false

	//joinTableFilterRegex, _ := regexp.Compile(`\(([^:]+[^&\)]+&?)+\)`)
	for _, rel := range dr.model.GetRelations() {

		if rel.GetSubject() == dr.model.GetName() {

			queries, ok := req.QueryParams[rel.GetObjectName()]
			if !ok {
				queries, ok = req.QueryParams[rel.GetObject()+"_id"]
			}
			if !ok || len(queries) < 1 {
				continue
			}

			joinTableFilters := make(map[string]goqu.Ex)

			for i, query := range queries {
				if strings.Index(query, "@") > -1 {
					queryParts := strings.Split(query, "@")
					joinId := queryParts[0]
					joinQuery := queryParts[1]
					joinQuery = joinQuery[1 : len(joinQuery)-1]
					joinQueryParts := strings.Split(joinQuery, "&")
					joinWhere := goqu.Ex{}
					for _, joinQueryPart := range joinQueryParts {
						parts := strings.Split(joinQueryPart, ":")
						joinWhere[parts[0]] = parts[1]
					}
					//matches := joinTableFilterRegex.FindAllStringSubmatch(joinQuery, -1)
					joinTableFilters[joinId] = joinWhere
					queries[i] = joinId
				}
			}

			objectNameList, ok := req.QueryParams[rel.GetObject()+"Name"]

			var objectName string
			/**
			api2go give us two params for each relationship
			<entityName> -> the name of the column which is used to reference, usually <entity>_id but you name it something for special relations in the config
			*/
			if !ok {
				objectName = rel.GetObjectName()
				ok = true
			} else {
				objectName = objectNameList[0]
				if objectName != rel.GetSubjectName() {
					ok = false
				}
			}
			if ok {

				//ids, err := dr.GetSingleColumnValueByReferenceId(rel.GetObject(), []interface{}{"id"}, "reference_id", queries)

				refIdsToIdMap, err := dr.GetReferenceIdListToIdList(rel.GetObject(), queries)

				//log.Printf("Converted ids: %v", ids)
				if err != nil || len(refIdsToIdMap) < 1 {
					log.Errorf("Failed to convert refids to ids [%v][%v]: %v", rel.GetObject(), queries, err)
					return nil, nil, nil, false, err
				}

				intIdList := ValuesOf(refIdsToIdMap)
				switch rel.Relation {
				case "has_one":
					finalResponseIsSingleObject = false
					queryBuilder = queryBuilder.Where(goqu.Ex{rel.GetObjectName(): intIdList})
					countQueryBuilder = countQueryBuilder.Where(goqu.Ex{rel.GetObjectName(): intIdList})
					break

				case "belongs_to":
					finalResponseIsSingleObject = false
					queryBuilder = queryBuilder.Where(goqu.Ex{rel.GetObjectName(): intIdList})
					countQueryBuilder = countQueryBuilder.Where(goqu.Ex{rel.GetObjectName(): intIdList})
					break

				case "has_many":
					wh := goqu.Ex{}
					wh[rel.GetObjectName()+".id"] = intIdList

					if len(joinTableFilters) > 0 {

						k := 0
						for refId, joinFilter := range joinTableFilters {
							k = k+1
							intId := refIdsToIdMap[refId]
							joinTableAs := fmt.Sprintf("%v%v", rel.GetJoinTableName(), k)
							objectTableAs := fmt.Sprintf("%v%v", rel.GetObjectName(), k)

							joinTableJoinClause := goqu.Ex{
								fmt.Sprintf("%v.%v", joinTableAs, rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetSubject())),
							}

							for key, val := range joinFilter {
								joinTableJoinClause[fmt.Sprintf("%v.%v", joinTableAs, key)] = val
							}
							joinTableJoinClause[fmt.Sprintf("%v.%v", joinTableAs, rel.GetObjectName())] = intId


							objectTableJoinClause := goqu.Ex{
								fmt.Sprintf("%v.%v", joinTableAs, rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", objectTableAs)),
							}

							queryBuilder = queryBuilder.
								Join(
									goqu.T(rel.GetJoinTableName()).As(joinTableAs),
									goqu.On(joinTableJoinClause),
								).
								Join(
									goqu.T(rel.GetObject()).As(objectTableAs),
									goqu.On(objectTableJoinClause),
								)

							countQueryBuilder = countQueryBuilder.
								Join(
									goqu.T(rel.GetJoinTableName()).As(joinTableAs),
									goqu.On(joinTableJoinClause),
								).
								Join(
									goqu.T(rel.GetObject()).As(objectTableAs),
									goqu.On(objectTableJoinClause),
								)
						}

						//queryBuilder = queryBuilder.Where(wh)
						//countQueryBuilder = countQueryBuilder.Where(wh)

					} else {

						joinTableJoinClause := goqu.Ex{
							fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetSubject())),
						}

						objectTableJoinClause := goqu.Ex{
							fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetObjectName())),
						}

						queryBuilder = queryBuilder.
							Join(
								goqu.T(rel.GetJoinTableName()).As(rel.GetJoinTableName()),
								goqu.On(joinTableJoinClause),
							).
							Join(
								goqu.T(rel.GetObject()).As(rel.GetObjectName()),
								goqu.On(objectTableJoinClause),
							).Where(wh)

						countQueryBuilder = countQueryBuilder.
							Join(
								goqu.T(rel.GetJoinTableName()).As(rel.GetJoinTableName()),
								goqu.On(goqu.Ex{
									fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetSubject())),
								}),
							).
							Join(
								goqu.T(rel.GetObject()).As(rel.GetObjectName()),
								goqu.On(goqu.Ex{
									fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetObjectName())),
								}),
							).Where(wh)

					}


					joins = append(joins, GetJoins(rel)...)
					joinFilters = append(joinFilters, wh)

					if len(rel.Columns) > 0 {
						for _, col := range rel.Columns {
							joinColumn := fmt.Sprintf("%v.%v", rel.GetJoinTableName(), col.ColumnName)
							finalCols = append(finalCols, column{
								originalvalue: goqu.I(joinColumn),
								reference:     joinColumn,
							})
						}
					}

					createdAtJoinColumn := fmt.Sprintf("%v.%v", rel.GetJoinTableName(), "created_at")
					finalCols = append(finalCols, column{
						originalvalue: goqu.I(createdAtJoinColumn).As("relation_created_at"),
						reference:     createdAtJoinColumn,
					})

					updatedAtJoinColumn := fmt.Sprintf("%v.%v", rel.GetJoinTableName(), "updated_at")
					finalCols = append(finalCols, column{
						originalvalue: goqu.I(updatedAtJoinColumn).As("relation_updated_at"),
						reference:     updatedAtJoinColumn,
					})

				}
			}
		}
		if rel.GetObject() == dr.model.GetName() {

			subjectNameList, ok := req.QueryParams[rel.GetSubject()+"Name"]
			if !ok {
				continue
			}
			//log.Printf("Reverse Relation %v", rel.String())

			var subjectName string
			/**
			api2go give us two params for each relationship
			<entityName> -> the name of the column which is used to reference, usually <entity>_id but you name it something for special relations in the config
			*/
			subjectName = subjectNameList[0]
			if subjectName != rel.GetObjectName() {
				continue
			}

			queries, ok := req.QueryParams[rel.GetSubject()+"_id"]
			//log.Printf("%d Values as RefIds for relation [%v]", len(filters), rel.String())
			if !ok || len(queries) < 1 {
				continue
			}
			ids, err := dr.GetSingleColumnValueByReferenceId(rel.GetSubject(), []interface{}{"id"}, "reference_id", queries)

			switch rel.Relation {
			case "has_one":

				finalResponseIsSingleObject = true
				if len(ids) < 1 {
					continue
				}

				queryBuilder = queryBuilder.
					Join(
						goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
						goqu.On(goqu.Ex{
							fmt.Sprintf("%v.%v", rel.GetSubjectName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetObject())),
						}),
					).
					Where(goqu.Ex{rel.GetSubjectName() + ".id": ids})

				countQueryBuilder = countQueryBuilder.Join(
					goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
					goqu.On(goqu.Ex{
						fmt.Sprintf("%v.%v", rel.GetSubjectName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetObject())),
					}),
				).
					Where(goqu.Ex{rel.GetSubjectName() + ".id": ids})

				joins = append(joins, GetReverseJoins(rel)...)
				joinFilters = append(joinFilters, goqu.Ex{rel.GetSubjectName() + ".id": ids})
				break

			case "belongs_to":

				finalResponseIsSingleObject = true

				if err != nil || len(ids) < 1 {
					log.Errorf("Failed to convert [%v]refids to ids[%v]: %v", rel.GetSubject(), queries, err)
					return nil, nil, nil, false, err
				}

				queryBuilder = queryBuilder.
					Join(
						goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
						goqu.On(goqu.Ex{
							fmt.Sprintf("%v.%v", rel.GetSubjectName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetObject())),
						}),
					).
					Where(goqu.Ex{rel.GetSubjectName() + ".id": ids})
				countQueryBuilder = countQueryBuilder.Join(
					goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
					goqu.On(goqu.Ex{
						fmt.Sprintf("%v.%v", rel.GetSubjectName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetObject())),
					}),
				).Where(goqu.Ex{rel.GetSubjectName() + ".id": ids})
				joins = append(joins, GetReverseJoins(rel)...)
				joinFilters = append(joinFilters, goqu.Ex{rel.GetSubjectName() + ".id": ids})
				break
			case "has_many":
				//log.Printf("Has many [%v] : [%v] === %v", dr.model.GetName(), subjectId, req.QueryParams)
				queryBuilder = queryBuilder.
					Join(
						goqu.T(rel.GetJoinTableName()).As(rel.GetJoinTableName()),
						goqu.On(goqu.Ex{
							fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetObject())),
						}),
					).
					Join(
						goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
						goqu.On(goqu.Ex{
							fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetSubjectName())),
						}),
					).
					Where(goqu.Ex{rel.GetSubjectName() + ".id": ids})
				countQueryBuilder = countQueryBuilder.
					Join(
						goqu.T(rel.GetJoinTableName()).As(rel.GetJoinTableName()),
						goqu.On(goqu.Ex{
							fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetObject())),
						})).
					Join(
						goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
						goqu.On(goqu.Ex{
							fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.id", rel.GetSubjectName())),
						})).Where(goqu.Ex{rel.GetSubjectName() + ".id": ids})
				joins = append(joins, GetReverseJoins(rel)...)
				joinFilters = append(joinFilters, goqu.Ex{rel.GetSubjectName() + ".id": ids})

				if len(rel.Columns) > 0 {
					for _, col := range rel.Columns {
						joinColumn := fmt.Sprintf("%v.%v", rel.GetJoinTableName(), col.ColumnName)
						finalCols = append(finalCols, column{
							originalvalue: goqu.I(joinColumn),
							reference:     joinColumn,
						})
					}
				}

				createdAtJoinColumn := fmt.Sprintf("%v.%v", rel.GetJoinTableName(), "created_at")
				finalCols = append(finalCols, column{
					originalvalue: goqu.I(createdAtJoinColumn).As("relation_created_at"),
					reference:     createdAtJoinColumn,
				})

				updatedAtJoinColumn := fmt.Sprintf("%v.%v", rel.GetJoinTableName(), "updated_at")
				finalCols = append(finalCols, column{
					originalvalue: goqu.I(updatedAtJoinColumn).As("relation_updated_at"),
					reference:     updatedAtJoinColumn,
				})

			}

		}
	}

	orders := make([]exp.OrderedExpression, 0)
	for _, so := range sortOrder {

		if len(so) < 1 {
			continue
		}
		//log.Printf("Sort order: %v", so)
		if so[0] == '-' {
			//ord := prefix + so[1:] + " desc"
			// queryBuilder = queryBuilder.OrderBy(ord)
			// countQueryBuilder = countQueryBuilder.OrderBy(ord)
			orders = append(orders, goqu.I(prefix+so[1:]).Desc())
		} else {
			if so[0] == '+' {
				//ord := prefix + so[1:] + " asc"
				// queryBuilder = queryBuilder.OrderBy(ord)
				// countQueryBuilder = countQueryBuilder.OrderBy(ord)
				orders = append(orders, goqu.I(prefix+so[1:]).Asc())
			} else {
				ord := prefix + so
				if strings.ToLower(so) == "rand()" || strings.ToLower(so) == "random()" {
					ord = so
				}
				// queryBuilder = queryBuilder.OrderBy(ord)
				// countQueryBuilder = countQueryBuilder.OrderBy(ord)
				orders = append(orders, goqu.I(ord).Asc())
			}
		}
	}

	if !isAdmin && tableModel.GetTableName() != "usergroup" {

		groupReferenceIds := make([]string, 0)
		groupIds := make(map[string]int64)
		for _, group := range sessionUser.Groups {
			groupReferenceIds = append(groupReferenceIds, group.GroupReferenceId)
		}
		groupCount := len(groupReferenceIds)
		if groupCount > 0 {
			groupIds, err = dr.GetReferenceIdListToIdList("usergroup", groupReferenceIds)
			CheckErr(err, "Failed to fetch group ids")
		}
		groupParameters := ""

		groupQueries := make([]goqu.Ex, 0)

		if groupCount > 0 {
			groupQueries = append(groupQueries, goqu.Ex{
				fmt.Sprintf("%s.usergroup_id", joinTableName): goqu.Op{
					"in": groupIds,
				},
			})
			groupParameters = strings.Join(strings.Split(strings.Repeat("?", groupCount), ""), ",")
			groupParameters = fmt.Sprintf(" or ((%s.permission & 32768) = 32768 and "+"%s.usergroup_id in ("+groupParameters+")) ",
				joinTableName, joinTableName,
			)
		}
		queryArgs := make([]interface{}, 0)
		for _, id := range groupIds {
			queryArgs = append(queryArgs, id)
		}
		queryArgs = append(queryArgs, sessionUser.UserId)

		queryBuilder = queryBuilder.Where(goqu.L(fmt.Sprintf("(((%s.permission & 2) = 2)"+
			groupParameters+" ) or "+
			"(%s.user_account_id = ? and (%s.permission & 256) = 256)",
			tableModel.GetTableName(), tableModel.GetTableName(), tableModel.GetTableName(),
		), queryArgs...))

		countQueryBuilder = countQueryBuilder.Where(goqu.L(fmt.Sprintf("("+
			"((%s.permission & 2) = 2)  "+groupParameters+" ) or "+
			"(%s.user_account_id = ? and (%s.permission & 256) = 256)",
			tableModel.GetTableName(),
			tableModel.GetTableName(), tableModel.GetTableName()),
			queryArgs...))

	}

	idsListQuery, args, err := queryBuilder.Order(orders...).ToSQL()
	if err != nil {
		return nil, nil, nil, false, err
	}
	log.Infof("Id query: [%s]", idsListQuery)
	//log.Debugf("Id query args: %v", args)
	stmt, err := dr.connection.Preparex(idsListQuery)
	if err != nil {
		log.Errorf("Findall select query sql 738: %v == %v", idsListQuery, args)
		log.Errorf("Failed to prepare sql 674: %v", err)
		return nil, nil, nil, false, err
	}

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

	idsRow, err := stmt.Queryx(args...)
	if err != nil {
		log.Errorf("Findall select query sql 745: %v == %v", idsListQuery, args)
		log.Errorf("Failed to prepare sql 680: %v", err)
		return nil, nil, nil, false, err
	}
	ids := make([]int64, 0)

	for idsRow.Next() {
		row := make(map[string]interface{})
		err = idsRow.MapScan(row)
		if err != nil {
			return nil, nil, nil, false, err
		}
		ids = append(ids, row["id"].(int64))
	}
	idsRow.Close()

	if len(languagePreferences) == 0 {

		for i, col := range finalCols {
			if strings.Index(col.reference, ".") == -1 {
				finalCols[i] = column{
					originalvalue: goqu.I(prefix + col.reference),
					reference:     prefix + col.reference,
				}
			}
		}

		queryBuilder = statementbuilder.Squirrel.Select(ColumnToInterfaceArray(finalCols)...).From(tableModel.GetTableName()).Where(goqu.Ex{
			idColumn: ids,
		}).Order(orders...)

	} else {
		var preferredLanguage = languagePreferences[0]
		translateTableName := tableModel.GetTableName() + "_i18n"

		ifNullFunctionName := "IFNULL"
		if dr.connection.DriverName() == "postgres" {
			ifNullFunctionName = "COALESCE"
		} else if dr.connection.DriverName() == "mssql" {
			ifNullFunctionName = "ISNULL"
		}

		//translatedColumns := make([]string, 0)
		for i, columnValue := range finalCols {
			if IsStandardColumn(columnValue.reference) {
				finalCols[i] = column{
					originalvalue: goqu.I(prefix + columnValue.reference),
					reference:     columnValue.reference,
				}
			} else {
				if strings.Index(columnValue.reference, ".") == -1 {
					finalCols[i] = column{
						originalvalue: goqu.L(ifNullFunctionName + "(" + translateTableName + "." + columnValue.reference + "," + prefix + columnValue.reference + ") as " + columnValue.reference),
						reference:     columnValue.reference,
					}
				} else {
					finalCols[i] = columnValue
				}
			}
		}

		queryBuilder = statementbuilder.Squirrel.Select(ColumnToInterfaceArray(finalCols)...).
			From(tableModel.GetTableName()).
			LeftJoin(
				goqu.T(translateTableName),
				goqu.On(goqu.Ex{
					translateTableName + ".translation_reference_id": tableModel.GetTableName() + ".id",
					translateTableName + ".language_id":              "'" + preferredLanguage + "'",
				})).
			Where(goqu.Ex{
				idColumn: ids,
			}).Order(orders...)

	}

	if len(joins) > 0 {
		for _, j := range joins {
			queryBuilder = queryBuilder.Join(j.table, j.condition)
		}
		for _, w := range joinFilters {
			queryBuilder = queryBuilder.Where(w)
		}
	}

	results := make([]map[string]interface{}, 0)
	includes := make([][]map[string]interface{}, 0)
	total1 := uint64(0)
	if len(ids) > 0 {

		sql1, args, err := queryBuilder.ToSQL()
		//log.Printf("Query: %v == %v", sql1, args)

		if err != nil {
			log.Printf("Error: %v", err)
			return nil, nil, nil, false, err
		}

		stmt, err = dr.connection.Preparex(sql1)
		if err != nil {
			log.Printf("Findall select query sql 762: %v == %v", sql1, args)
			log.Errorf("Failed to prepare sql 763: %v", err)
			return nil, nil, nil, false, err
		}
		defer func() {
			err = stmt.Close()
			CheckErr(err, "Failed to close statement")
		}()
		rows, err := stmt.Queryx(args...)

		if err != nil {
			log.Printf("Error: %v", err)
			return nil, nil, nil, false, err
		}
		defer func() {
			err = rows.Close()
			CheckErr(err, "Failed to close rows")
		}()

		results, includes, err = dr.ResultToArrayOfMap(rows, dr.model.GetColumnMap(), includedRelations)

	}
	total1 = dr.GetTotalCountBySelectBuilder(countQueryBuilder)

	//log.Printf("Found: %d results", len(results))
	//log.Printf("Results: %v", results)

	if pageSize < 1 {
		pageSize = 10
	}

	paginationData := &PaginationData{
		PageNumber: pageNumber,
		PageSize:   pageSize,
		TotalCount: total1,
	}

	return results, includes, paginationData, finalResponseIsSingleObject, err

}

func ValuesOf(mapItem map[string]int64) []int64 {
	ret := make([]int64, 0)
	for _, item := range mapItem {
		ret = append(ret, item)
	}
	return ret

}

func GetJoins(rel api2go.TableRelation) []join {
	switch rel.Relation {
	case "belongs_to":
		return []join{
			{
				table: goqu.T(rel.GetObject()).As(rel.GetObjectName()),
				condition: goqu.On(goqu.Ex{
					fmt.Sprintf("%v.%v", rel.GetSubject(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetObjectName(), "id")),
				}),
			},
		}
	case "has_one":
		return []join{
			{
				table: goqu.T(rel.GetObject()).As(rel.GetObjectName()),
				condition: goqu.On(goqu.Ex{
					fmt.Sprintf("%v.%v", rel.GetSubject(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetObjectName(), "id")),
				}),
			},
		}

	case "has_many":
		fallthrough
	case "has_many_and_belongs_to_many":
		return []join{
			{
				table: goqu.T(rel.GetJoinTableName()).As(rel.GetJoinTableName()),
				condition: goqu.On(goqu.Ex{
					fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetSubject(), "id")),
				}),
			},
			{
				table: goqu.T(rel.GetObject()).As(rel.GetObjectName()),
				condition: goqu.On(goqu.Ex{
					fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetObjectName(), "id")),
				}),
			},
		}

	}
	return []join{}
}

func GetReverseJoins(rel api2go.TableRelation) []join {
	switch rel.Relation {
	case "belongs_to":
		return []join{
			{
				table: goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
				condition: goqu.On(goqu.Ex{
					fmt.Sprintf("%v.%v", rel.GetSubjectName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetObject(), "id")),
				}),
			},
		}
	case "has_one":
		return []join{
			{
				table: goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
				condition: goqu.On(goqu.Ex{
					fmt.Sprintf("%v.%v", rel.GetSubjectName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetObject(), "id")),
				}),
			},
		}

	case "has_many":
		fallthrough
	case "has_many_and_belongs_to_many":
		return []join{
			{
				table: goqu.T(rel.GetJoinTableName()).As(rel.GetJoinTableName()),
				condition: goqu.On(goqu.Ex{
					fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetObjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetObject(), "id")),
				}),
			},
			{
				table: goqu.T(rel.GetSubject()).As(rel.GetSubjectName()),
				condition: goqu.On(goqu.Ex{
					fmt.Sprintf("%v.%v", rel.GetJoinTableName(), rel.GetSubjectName()): goqu.I(fmt.Sprintf("%v.%v", rel.GetSubjectName(), "id")),
				}),
			},
		}

	}
	return []join{}
}

var OperatorMap = map[string]string{
	"contains":     "like",
	"like":         "like",
	"begins with":  "like",
	"ends with":    "like",
	"not contains": "notlike",
	"not like":     "notlike",
	"is":           "is",
	"in":           "in",
	"is not":       "isnot",
	"before":       "lt",
	"after":        "gt",
	"more then":    "gt",
	"any of":       "any of",
	"none of":      "none of",
	"less then":    "lt",
	"is empty":     "is nil",
	"is true":      "is true",
	"is false":     "is false",
}

func (dr *DbResource) addFilters(queryBuilder *goqu.SelectDataset, countQueryBuilder *goqu.SelectDataset,
	queries []Query, prefix string) (*goqu.SelectDataset, *goqu.SelectDataset) {

	if len(queries) == 0 {
		return queryBuilder, countQueryBuilder
	}

	for _, filterQuery := range queries {

		columnName := filterQuery.ColumnName
		tableInfo := dr.tableInfo

		colInfo, ok := tableInfo.GetColumnByName(columnName)

		if !ok {
			log.Printf("warn: invalid column [%v] in query, skipping", columnName)
			continue
		}

		if colInfo.IsForeignKey {

			values := filterQuery.Value

			valueString, isString := values.(string)
			valuesArray := []string{}
			if !isString {
				valuesArray, ok = values.([]string)
				if !ok {
					log.Printf("invalid value type in forign key column [%v] filter: %v", columnName, values)
				}
			} else {
				valuesArray = append(valuesArray, valueString)
			}

			valueIds := make(map[string]int64, len(valuesArray))

			valueIds, err := dr.GetReferenceIdListToIdList(colInfo.ForeignKeyData.Namespace, valuesArray)
			if err != nil {
				log.Printf("failed to lookup foreign key value: %v, skipping column filter", err)
				continue
			}

			values = valueIds
			if isString {
				values = valueIds[valuesArray[0]]
			}
			filterQuery.Value = values

		}

		opValue, ok := OperatorMap[filterQuery.Operator]
		var actualvalue interface{}
		query := goqu.I(prefix + filterQuery.ColumnName)

		actualvalue = filterQuery.Value
		if !ok {
			opValue = filterQuery.Operator
		}

		if BeginsWith(opValue, "is") {
			parts := strings.Split(opValue, " ")
			if len(parts) > 1 {
				switch parts[1] {
				case "true":
					actualvalue = true
				case "false":
					actualvalue = false
				case "empty":
					actualvalue = nil
				case "nil":
					actualvalue = nil
				}
			}
			switch parts[0] {
			case "is":
				opValue = "="
			case "not":
				query.IsNot(actualvalue)
			}
		}

		if opValue == "=" {

			queryBuilder = queryBuilder.Where(goqu.Ex{
				prefix + filterQuery.ColumnName: actualvalue,
			})

			countQueryBuilder = countQueryBuilder.Where(goqu.Ex{
				prefix + filterQuery.ColumnName: actualvalue,
			})

		} else {

			queryBuilder = queryBuilder.Where(goqu.Ex{
				prefix + filterQuery.ColumnName: goqu.Op{
					opValue: actualvalue,
				},
			})

			countQueryBuilder = countQueryBuilder.Where(goqu.Ex{
				prefix + filterQuery.ColumnName: goqu.Op{
					opValue: actualvalue,
				},
			})

		}

	}

	return queryBuilder, countQueryBuilder
}

func (dr *DbResource) FindAll(req api2go.Request) (response api2go.Responder, err error) {
	req.QueryParams["page[size]"] = []string{"1000"}
	_, responder, e := dr.PaginatedFindAll(req)
	return responder, e
}

func (dr *DbResource) PaginatedFindAll(req api2go.Request) (totalCount uint, response api2go.Responder, err error) {

	for _, bf := range dr.ms.BeforeFindAll {
		//log.Printf("Invoke BeforeFindAll [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())
		_, err := bf.InterceptBefore(dr, &req, []map[string]interface{}{})
		if err != nil {
			log.Printf("Error from BeforeFindAll middleware [%v]: %v", bf.String(), err)
			return 0, NewResponse(nil, err, 400, nil), err
		}
	}
	//log.Printf("Request [%v]: %v", dr.model.GetName(), req.QueryParams)

	results, includes, pagination, finalResponseIsSingleObject, err := dr.PaginatedFindAllWithoutFilters(req)

	for _, bf := range dr.ms.AfterFindAll {
		//log.Printf("Invoke AfterFindAll [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

		results, err = bf.InterceptAfter(dr, &req, results)
		if err != nil {
			//log.Errorf("Error from findall paginated create middleware: %v", err)
			log.Errorf("Error from AfterFindAll[%v] middleware: %v", bf.String(), err)
		}
	}

	includesNew := make([][]map[string]interface{}, 0)
	for _, bf := range dr.ms.AfterFindAll {
		//log.Printf("Invoke AfterFindAll Includes [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

		for _, include := range includes {
			include, err = bf.InterceptAfter(dr, &req, include)
			if err != nil {
				log.Errorf("Error from AfterFindAll[includes][%v] middleware: %v", bf.String(), err)
				continue
			}
			includesNew = append(includesNew, include)
		}

	}

	result := make([]*api2go.Api2GoModel, 0)
	infos := dr.model.GetColumns()

	for i, res := range results {
		delete(res, "id")
		includes := includesNew[i]
		var a = api2go.NewApi2GoModel(dr.model.GetTableName(), infos, dr.model.GetDefaultPermission(), dr.model.GetRelations())
		a.Data = res

		for _, include := range includes {
			delete(include, "id")
			if BeginsWith(include["__type"].(string), "file.") {
				continue
			}
			perm, ok := include["permission"].(int64)
			if !ok {
				log.Errorf("Failed to parse permission, skipping record: %v", err)
				continue
			}

			incType := include["__type"].(string)
			model := api2go.NewApi2GoModelWithData(incType, dr.Cruds[incType].model.GetColumns(), int64(perm), dr.Cruds[incType].model.GetRelations(), include)

			a.Includes = append(a.Includes, model)
		}

		result = append(result, a)
	}

	//log.Printf("Offset, limit: %v, %v", pageNumber, pageSize)

	if pagination == nil {
		pagination = &PaginationData{
			PageNumber: 1,
			PageSize:   10,
		}
	}
	//log.Printf("Pagination :%v", pagination)

	var resultObj interface{}
	resultObj = result
	if finalResponseIsSingleObject {
		if len(result) > 0 {
			resultObj = result[0]
		} else {
			resultObj = nil
		}
	}
	return uint(pagination.TotalCount), NewResponse(nil, resultObj, 200, &api2go.Pagination{
		//Next:        map[string]string{"limit": fmt.Sprintf("%v", pagination.PageSize), "offset": fmt.Sprintf("%v", pagination.PageSize+pagination.PageNumber)},
		//Prev:        map[string]string{"limit": fmt.Sprintf("%v", pagination.PageSize), "offset": fmt.Sprintf("%v", pagination.PageNumber-pagination.PageSize)},
		//First:       map[string]string{},
		//Last:        map[string]string{"limit": fmt.Sprintf("%v", pagination.PageSize), "offset": fmt.Sprintf("%v", pagination.TotalCount-pagination.PageSize)},
		Total:       pagination.TotalCount,
		PerPage:     pagination.PageSize,
		CurrentPage: 1 + (pagination.PageNumber / pagination.PageSize),
		LastPage:    1 + (pagination.TotalCount / pagination.PageSize),
		From:        pagination.PageNumber + 1,
		To:          pagination.PageSize,
	}), nil

}
