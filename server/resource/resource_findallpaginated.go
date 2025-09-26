package resource

import (
	"context"
	"fmt"
	"github.com/buraksezer/olric"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"strconv"
	"strings"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"

	"encoding/base64"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/statementbuilder"
	log "github.com/sirupsen/logrus"
)

func (dbResource *DbResource) GetTotalCount() uint64 {
	s, v, err := statementbuilder.Squirrel.Select(goqu.L("count(*)")).Prepared(true).From(dbResource.model.GetName()).ToSQL()
	if err != nil {
		log.Errorf("Failed to generate count query for %v: %v", dbResource.model.GetName(), err)
		return 0
	}

	var count uint64

	start := time.Now()
	stmt1, err := dbResource.Connection().Preparex(s)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetTotalCount PrepareX: %v", duration)
	if err != nil {
		log.Errorf("[31] failed to prepare statment: %v", err)
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	start = time.Now()
	err = stmt1.QueryRowx(v...).Scan(&count)
	duration = time.Since(start)
	log.Tracef("[TIMING] GetTotalCount Scan: %v", duration)

	CheckErr(err, "Failed to execute total count query [%s] [%v]", s, v)
	//log.Printf("Count: [%v] %v", dbResource.model.GetTableName(), count)
	return count
}

func (dbResource *DbResource) GetTotalCountBySelectBuilder(builder *goqu.SelectDataset) uint64 {

	s, v, err := builder.ToSQL()
	//log.Printf("Count query: %v == %v", s, v)
	if err != nil {
		log.Errorf("Failed to generate count query for %v: %v", dbResource.model.GetName(), err)
		return 0
	}

	var count uint64

	start := time.Now()
	stmt1, err := dbResource.Connection().Preparex(s)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetTotalCountBySelectBuilder PrepareX: %v", duration)

	if err != nil {
		log.Errorf("[61] failed to prepare statment: %v", err)
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	start = time.Now()
	err = stmt1.QueryRowx(v...).Scan(&count)
	duration = time.Since(start)
	log.Tracef("[TIMING] GetTotalCountBySelectBuilder QueryRowx: %v", duration)

	if err != nil {
		log.Errorf("Failed to execute count query [%v] %v", s, err)
	}
	//log.Printf("Count: [%v] %v", dbResource.model.GetTableName(), count)
	return count
}

func GetTotalCountBySelectBuilderWithTransaction(builder *goqu.SelectDataset, transaction *sqlx.Tx) (uint64, error) {

	s, v, err := builder.ToSQL()
	//log.Printf("Count query: %v == %v", s, v)
	if err != nil {
		log.Errorf("Failed to generate count query: %v", err)
		return 0, nil
	}

	queryHash := GetMD5HashString(s + fmt.Sprintf("%v", v))
	cacheKey := fmt.Sprintf("count-%v", queryHash)
	if OlricCache != nil {
		cachedCount, err := OlricCache.Get(context.Background(), cacheKey)
		if err == nil {
			return cachedCount.Uint64()
		}
	}

	var count uint64

	start := time.Now()
	stmt1, err := transaction.Preparex(s)
	duration := time.Since(start)
	log.Tracef("[TIMING] GetTotalCountBySelectBuilder PrepareX: %v", duration)

	if err != nil {
		log.Errorf("[61] failed to prepare statment: %v", err)
		return 0, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	start = time.Now()
	err = stmt1.QueryRowx(v...).Scan(&count)
	duration = time.Since(start)
	log.Tracef("[TIMING] GetTotalCountBySelectBuilder QueryRowx: %v", duration)

	if err != nil {
		log.Errorf("Failed to execute count query [%v] %v", s, err)
		return 0, err
	}
	//log.Printf("Count: [%v] %v", dr.model.GetTableName(), count)

	if OlricCache != nil {
		OlricCache.Put(context.Background(), cacheKey, count, olric.EX(3*time.Second), olric.NX())
	}

	return count, nil
}

type PaginationData struct {
	PageNumber uint64
	PageSize   uint64
	TotalCount uint64
}

type Query struct {
	ColumnName   string              `json:"column"`
	Operator     string              `json:"operator"`
	Value        interface{}         `json:"value"`
	LogicalGroup string              `json:"logical_group,omitempty"` // For OR operations within groups
	FuzzyOptions *FuzzySearchOptions `json:"fuzzy_options,omitempty"` // Configuration for fuzzy search
}

type FuzzySearchOptions struct {
	Threshold    float64 `json:"threshold,omitempty"`     // Similarity threshold for PostgreSQL (0.0-1.0)
	MaxDistance  int     `json:"max_distance,omitempty"`  // Max Levenshtein distance
	SearchType   string  `json:"search_type,omitempty"`   // "trigram", "fulltext", "soundex", "partial"
	FallbackMode string  `json:"fallback_mode,omitempty"` // "strict", "partial", "soundex" for non-PostgreSQL
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
func (dbResource *DbResource) PaginatedFindAllWithoutFilters(req api2go.Request, transaction *sqlx.Tx) (
	[]map[string]interface{}, [][]map[string]interface{}, *PaginationData, bool, error) {
	log.Debugf("Find all row by params: [%v]: %v", dbResource.model.GetName(), req.QueryParams)
	var err error

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}

	start := time.Now()
	isAdmin := IsAdminWithTransaction(sessionUser, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] FindAllIsAdminCheck %v", duration)

	isRelatedGroupRequest := false // to switch permissions to the join table later in select query
	relatedTableName := ""
	if dbResource.model.GetName() == "usergroup" && len(req.QueryParams) > 2 {
		ok := false
		for key := range req.QueryParams {
			if relatedTableName, ok = EndsWith(key, "Name"); req.QueryParams[key][0] == "usergroup_id" && ok {
				isRelatedGroupRequest = true
				break
			}
		}
	}

	languagePreferences := make([]string, 0)
	if dbResource.tableInfo.TranslationsEnabled {
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
				return nil, nil, nil, false, fmt.Errorf("failed to unmarshal query as json: %v", err)
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
	} else if dbResource.tableInfo.DefaultOrder != "" && len(dbResource.tableInfo.DefaultOrder) > 2 {
		if dbResource.tableInfo.DefaultOrder[0] == '\'' || dbResource.tableInfo.DefaultOrder[0] == '"' {
			rep := strings.ReplaceAll(dbResource.tableInfo.DefaultOrder, "'", "\"")
			unquotedOrder, _ := strconv.Unquote(rep)
			dbResource.tableInfo.DefaultOrder = unquotedOrder
		}
		sortOrder = strings.Split(dbResource.tableInfo.DefaultOrder, ",")
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

	if len(req.QueryParams["filter[]"]) > 0 && len(queries) == 0 {
		filters = req.QueryParams["filter[]"]

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

	tableModel := dbResource.model
	//log.Printf("Get all resource type: %v\n", tableModel)

	cols := tableModel.GetColumns()
	finalCols := make([]column, 0)
	//log.Printf("Cols: %v", cols)

	prefix := dbResource.model.GetName() + "."
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

	if _, ok := req.QueryParams["usergroup_id"]; ok && req.QueryParams["usergroupName"][0] == dbResource.model.GetName()+"_id" {
		isRelatedGroupRequest = true
		if relatedTableName == "" {
			relatedTableName = dbResource.model.GetTableName()
		}
	}

	idColumn := fmt.Sprintf("%s.id", tableModel.GetTableName())
	distinctIdColumn := goqu.L(fmt.Sprintf("distinct(%s.id)", tableModel.GetTableName()))
	if isRelatedGroupRequest {
		//log.Printf("Switch permission to join table j1 instead of %v%v", prefix, "permission")
		if dbResource.model.GetName() == "usergroup" {
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
					originalvalue: goqu.I("usergroup.reference_id").As("relation_reference_id"),
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
	queryBuilder := statementbuilder.Squirrel.Select(idQueryCols...).Prepared(true).From(tableModel.GetTableName())
	//queryBuilder = queryBuilder.From(tableModel.GetTableName())
	var countQueryBuilder *goqu.SelectDataset
	countQueryBuilder = statementbuilder.Squirrel.
		Select(goqu.L(fmt.Sprintf("count(distinct(%v.id))", tableModel.GetTableName()))).Prepared(true).
		From(tableModel.GetTableName()).Offset(0).Limit(1)

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
		afterRefId := uuid.MustParse(req.QueryParams["page[after]"][0])
		id, err := GetReferenceIdToIdWithTransaction(dbResource.TableInfo().TableName, daptinid.DaptinReferenceId(afterRefId), transaction)
		if err != nil {
			queryBuilder = queryBuilder.Where(goqu.Ex{
				dbResource.TableInfo().TableName + ".id": goqu.Op{"gt": id},
			}).Limit(uint(pageSize))
		}
	} else if req.QueryParams["page[before]"] != nil && len(req.QueryParams["page[before]"]) > 0 {
		beforeRefId := uuid.MustParse(req.QueryParams["page[before]"][0])
		id, err := GetReferenceIdToIdWithTransaction(dbResource.TableInfo().TableName, daptinid.DaptinReferenceId(beforeRefId), transaction)
		if err != nil {
			queryBuilder = queryBuilder.Where(goqu.Ex{
				dbResource.TableInfo().TableName + ".id": goqu.Op{"lt": id},
			}).Limit(uint(pageSize))
		}
	} else {
		queryBuilder = queryBuilder.Offset(uint(pageNumber)).Limit(uint(pageSize))
	}
	joins := make([]join, 0)
	joinFilters := make([]goqu.Ex, 0)

	infos := dbResource.model.GetColumns()

	// todo: fix search in findall operation. currently no way to do an " or " query
	if len(filters) > 0 {

		colsToAdd := make([]string, 0)

		for _, col := range infos {
			if (col.IsIndexed || col.IsUnique) && (strings.Index(col.ColumnType, "name") > -1 || col.ColumnType == "label" || col.ColumnType == "email") {
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

	start = time.Now()
	queryBuilder, countQueryBuilder, err = dbResource.addFilters(queryBuilder, countQueryBuilder, queries, prefix, transaction)
	if err != nil {
		return nil, nil, nil, false, err
	}
	duration = time.Since(start)
	log.Tracef("[TIMING] FindAllAddFilters %v", duration)

	//if len(groupings) > 0 && false {
	//	for _, groupBy := range groupings {
	//		queryBuilder = queryBuilder.GroupBy(fmt.Sprintf("%s %s", groupBy.ColumnName, groupBy.Order))
	//	}
	//}

	// for relation api calls with has_one or belongs_to relations
	finalResponseIsSingleObject := false

	//joinTableFilterRegex, _ := regexp.Compile(`\(([^:]+[^&\)]+&?)+\)`)
	for _, rel := range dbResource.model.GetRelations() {

		if rel.GetSubject() == dbResource.model.GetName() {

			uuidStringQueries, ok := req.QueryParams[rel.GetObjectName()]
			if !ok {
				uuidStringQueries, ok = req.QueryParams[rel.GetObject()+"_id"]
			}
			if !ok || len(uuidStringQueries) < 1 || (len(uuidStringQueries) == 1 && uuidStringQueries[0] == "") {
				continue
			}

			joinTableFilters := make(map[daptinid.DaptinReferenceId]goqu.Ex)

			for i, query := range uuidStringQueries {
				if strings.Index(query, "@") > -1 {
					queryParts := strings.Split(query, "@")
					joinId := queryParts[0]
					joinQuery := strings.Join(queryParts[1:], "@")
					joinQuery = joinQuery[1 : len(joinQuery)-1]
					joinQueryParts := strings.Split(joinQuery, "&")
					joinWhere := goqu.Ex{}
					for _, joinQueryPart := range joinQueryParts {
						parts := strings.Split(joinQueryPart, ":")
						joinWhere[parts[0]] = strings.Split(parts[1], "|")
					}
					//matches := joinTableFilterRegex.FindAllStringSubmatch(joinQuery, -1)
					joinTableFilters[daptinid.DaptinReferenceId(uuid.MustParse(joinId))] = joinWhere
					uuidStringQueries[i] = joinId
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

				//ids, err := dbResource.GetSingleColumnValueByReferenceId(rel.GetObject(), []interface{}{"id"}, "reference_id", uuidStringQueries)

				if len(uuidStringQueries) == 0 || uuidStringQueries[0] == "" {
					log.Warnf("uuidStringQueries for %s is empty, skipping", rel.GetObjectName())
					continue
				}

				var uuidByteQueries []daptinid.DaptinReferenceId
				for _, str := range uuidStringQueries {
					u, er := uuid.Parse(str)
					if er != nil {
						return nil, nil, nil, false, fmt.Errorf("[602] failed to parse value as uuid: [%s] => %v", str, er)
					} else {
						uuidByteQueries = append(uuidByteQueries, daptinid.DaptinReferenceId(u))
					}
				}

				refIdsToIdMap, err := GetReferenceIdListToIdListWithTransaction(rel.GetObject(), uuidByteQueries, transaction)

				//log.Printf("Converted ids: %v", ids)
				if err != nil {
					log.Errorf("[612] Failed to convert refids to ids [%v][%v]: %v", rel.GetObject(), uuidStringQueries, err)
					return nil, nil, nil, false, err
				}

				if len(refIdsToIdMap) < 1 {
					log.Errorf("[576] Failed to convert refids to ids [%v][%v]: %v", rel.GetObject(), uuidStringQueries, err)
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
							k = k + 1
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
		if rel.GetObject() == dbResource.model.GetName() {

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
			if !ok || len(queries) < 1 || queries[0] == "" {
				continue
			}
			ids, err := GetSingleColumnValueByReferenceIdWithTransaction(rel.GetSubject(), []interface{}{"id"},
				"reference_id", queries, transaction)

			if len(ids) < 1 {
				return nil, nil, nil, false, fmt.Errorf("subject not resolved [%v][%v]", rel.GetSubject(), queries)
			}

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
				//log.Printf("Has many [%v] : [%v] === %v", dbResource.model.GetName(), subjectId, req.QueryParams)
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

		groupReferenceIds := make([]daptinid.DaptinReferenceId, 0)
		groupIds := make(map[daptinid.DaptinReferenceId]int64)
		groupIdIntList := make([]int64, 0)
		for _, group := range sessionUser.Groups {
			groupReferenceIds = append(groupReferenceIds, group.GroupReferenceId)
		}
		groupCount := len(groupReferenceIds)
		if groupCount > 0 {
			groupIds, err = GetReferenceIdListToIdListWithTransaction("usergroup", groupReferenceIds, transaction)
			CheckErr(err, "Failed to fetch group ids")
		}
		groupParameters := ""

		groupQueries := make([]goqu.Ex, 0)

		if groupCount > 0 {
			for _, intId := range groupIds {
				groupIdIntList = append(groupIdIntList, intId)
			}
			groupQueries = append(groupQueries, goqu.Ex{
				fmt.Sprintf("%s.usergroup_id", joinTableName): goqu.Op{
					"in": groupIdIntList,
				},
			})

			gCount := len(groupQueries)
			if gCount > 0 {
				gids := make([]string, 0)
				for _, gid := range groupIds {
					gids = append(gids, fmt.Sprintf("%d", gid))
				}
				groupParameters = strings.Join(gids, ",")
				groupParameters = fmt.Sprintf(" or ((%s.permission & 32768) = 32768 and "+"%s.usergroup_id in ("+groupParameters+")) ",
					joinTableName, joinTableName,
				)
			}
		}
		queryArgs := make([]interface{}, 0)
		for _, id := range groupIds {
			queryArgs = append(queryArgs, id)
		}
		queryArgs = append(queryArgs, sessionUser.UserId)

		queryBuilder = queryBuilder.Where(
			goqu.L(
				fmt.Sprintf("(((%s.permission & 2) = 2)"+
					groupParameters+" or "+
					"(%s.user_account_id = "+fmt.Sprintf("%d", sessionUser.UserId)+" and (%s.permission & 256) = 256))",
					tableModel.GetTableName(), tableModel.GetTableName(), tableModel.GetTableName())))

		countQueryBuilder = countQueryBuilder.Where(goqu.L(fmt.Sprintf("("+
			"((%s.permission & 2) = 2)  "+groupParameters+"  or "+
			"(%s.user_account_id = "+fmt.Sprintf("%d", sessionUser.UserId)+" and (%s.permission & 256) = 256))",
			tableModel.GetTableName(),
			tableModel.GetTableName(), tableModel.GetTableName()),
			queryArgs...))

	}

	idsListQuery, args, err := queryBuilder.Order(orders...).ToSQL()
	log.Tracef("[983] Id query: [%s]", err)
	if err != nil {
		return nil, nil, nil, false, err
	}
	log.Tracef("[984] Id query: [%s] => %v", idsListQuery, args)
	//log.Debugf("Id query args: %v", args)
	start = time.Now()
	stmt, err := transaction.Preparex(idsListQuery)
	duration = time.Since(start)
	log.Tracef("[TIMING] IdQuery Preparex: %v", duration)

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

	start = time.Now()
	idsRow, err := stmt.Queryx(args...)
	duration = time.Since(start)
	log.Tracef("[TIMING] IdQuery Queryx: %v", duration)

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
	_ = idsRow.Close()
	_ = stmt.Close()

	if len(languagePreferences) == 0 {

		for i, col := range finalCols {
			if strings.Index(col.reference, ".") == -1 {
				finalCols[i] = column{
					originalvalue: goqu.I(prefix + col.reference),
					reference:     prefix + col.reference,
				}
			}
		}

		queryBuilder = statementbuilder.Squirrel.Select(ColumnToInterfaceArray(finalCols)...).Prepared(true).
			From(tableModel.GetTableName()).Where(goqu.Ex{
			idColumn: ids,
		}).Order(orders...)

	} else {
		var preferredLanguage = languagePreferences[0]
		translateTableName := tableModel.GetTableName() + "_i18n"

		ifNullFunctionName := "IFNULL"
		if dbResource.Connection().DriverName() == "postgres" {
			ifNullFunctionName = "COALESCE"
		} else if dbResource.Connection().DriverName() == "mssql" {
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
			From(tableModel.GetTableName()).Prepared(true).
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

		start = time.Now()
		stmt, err = transaction.Preparex(sql1)
		duration = time.Since(start)
		log.Tracef("[TIMING] IdQuery Select Preparex: %v", duration)

		if err != nil {
			log.Printf("Findall select query sql 762: %v == %v", sql1, args)
			log.Errorf("Failed to prepare sql 763: %v", err)
			return nil, nil, nil, false, err
		}
		defer func() {
			err = stmt.Close()
			CheckErr(err, "Failed to close statement")
		}()
		start = time.Now()
		rows, err := stmt.Queryx(args...)
		duration = time.Since(start)
		log.Tracef("[TIMING] IdQuery Select QueryX: %v", duration)

		if err != nil {
			log.Printf("Error: %v", err)
			return nil, nil, nil, false, err
		}
		defer func() {
			err = rows.Close()
			CheckErr(err, "Failed to close rows")
		}()

		start = time.Now()
		responseArray, err := RowsToMap(rows, dbResource.model.GetName())
		err = stmt.Close()
		err = rows.Close()

		results, includes, err = dbResource.ResultToArrayOfMapWithTransaction(responseArray,
			dbResource.model.GetColumnMap(), includedRelations, transaction)
		if err != nil {
			return nil, nil, nil, false, err
		}
		duration = time.Since(start)
		log.Tracef("[TIMING] FindAll ResultToArray: %v", duration)

	}
	start = time.Now()

	total1, err = GetTotalCountBySelectBuilderWithTransaction(countQueryBuilder, transaction)
	if err != nil {
		return nil, nil, nil, false, err
	}

	duration = time.Since(start)
	log.Tracef("[TIMING] GetTotalCountBySelectBuilder: %v", duration)

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

func ValuesOf(mapItem map[daptinid.DaptinReferenceId]int64) []int64 {
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
	"ilike":        "iLike",
	"begins with":  "like",
	"ends with":    "like",
	"not contains": "notLike",
	"not like":     "notLike",
	"not ilike":    "notILike",
	"is":           "is",
	"eq":           "eq",
	"neq":          "neq",
	"in":           "in",
	"is not":       "isNot",
	"before":       "lt",
	"after":        "gt",
	"more than":    "gt",
	"any of":       "any of",
	"none of":      "none of",
	"less than":    "lt",
	"is empty":     "is nil",
	"is true":      "is true",
	"is false":     "is false",
	"fuzzy":        "fuzzy",     // Single term fuzzy match
	"fuzzy_any":    "fuzzy_any", // ANY keyword matches with fuzzy tolerance
	"fuzzy_all":    "fuzzy_all", // ALL keywords must match with fuzzy tolerance
}

// Helper function to generate partial matches for non-postgres databases
func generatePartialPatterns(keyword string) []string {
	patterns := []string{keyword}

	// For longer keywords, add prefix matching
	if len(keyword) > 4 {
		// Match words starting with the same prefix
		prefix := keyword[:len(keyword)-1]
		patterns = append(patterns, prefix)

		if len(keyword) > 5 {
			prefix = keyword[:len(keyword)-2]
			patterns = append(patterns, prefix)
		}
	}

	return patterns
}

// Process a single query filter and return the expression
func (dbResource *DbResource) processQueryFilter(filterQuery Query, prefix string, transaction *sqlx.Tx) (goqu.Expression, error) {
	columnName := filterQuery.ColumnName
	tableInfo := dbResource.tableInfo

	colInfo, ok := tableInfo.GetColumnByName(columnName)

	if !ok {
		log.Warnf("[1316] Table [%v] invalid column query [%v], skipping", dbResource.model.GetName(), columnName)
		return nil, fmt.Errorf("table [%v] invalid column query [%v]", dbResource.model.GetName(), columnName)
	}

	// Handle foreign key columns
	if colInfo.IsForeignKey {
		refernceValueString := filterQuery.Value
		var refUuid uuid.UUID
		var err error
		if refernceValueString != nil {
			asStr, isStr := refernceValueString.(string)
			if isStr {
				refUuid, err = uuid.Parse(asStr)
			} else {
				err = fmt.Errorf("reference value is not uuid")
			}
		} else {
			err = fmt.Errorf("reference value is nil")
		}

		valuesArray := []daptinid.DaptinReferenceId{}
		if err != nil {
			if !(refernceValueString == nil || refernceValueString == "null" || refernceValueString == "nil") {
				log.Errorf("invalid value type in foreign key column [%v] filter: %v", columnName, refernceValueString)
			}
		} else {
			valuesArray = append(valuesArray, daptinid.DaptinReferenceId(refUuid))
			valueIds, err := GetReferenceIdListToIdListWithTransaction(colInfo.ForeignKeyData.Namespace, valuesArray, transaction)
			if err != nil {
				log.Warnf("[1334] failed to lookup foreign key value: %v => %v", refernceValueString, err)
			} else {
				refernceValueString = valueIds
				refernceValueString, ok = valueIds[valuesArray[0]]
				if !ok {
					refernceValueString = valuesArray[0]
				}
				filterQuery.Value = refernceValueString
			}
		}
	}

	opValue, ok := OperatorMap[filterQuery.Operator]
	if !ok {
		opValue = filterQuery.Operator
	}

	var actualvalue interface{}
	query := goqu.I(prefix + filterQuery.ColumnName)

	actualvalue = filterQuery.Value

	if filterQuery.ColumnName == "reference_id" {
		i := daptinid.InterfaceToDIR(filterQuery.Value)
		actualvalue = i[:]
	}

	// Handle "is" and "not" operators
	if BeginsWith(opValue, "is") || BeginsWith(opValue, "not") {
		parts := strings.Split(opValue, " ")
		if len(parts) > 1 {
			switch parts[1] {
			case "true":
				actualvalue = true
			case "false":
				actualvalue = false
			case "empty", "null", "nil":
				actualvalue = nil
			}
		}
		if len(parts) == 2 {
			switch parts[0] {
			case "is":
				opValue = "#"
				switch actualvalue {
				case true:
					actualvalue = query.IsTrue()
				case false:
					actualvalue = query.IsFalse()
				case nil:
					actualvalue = query.IsNull()
				}
			case "not":
				opValue = "#"
				switch actualvalue {
				case true:
					actualvalue = query.IsNotTrue()
				case false:
					actualvalue = query.IsNotFalse()
				case nil:
					actualvalue = query.IsNotNull()
				}
			}
		} else {
			switch opValue {
			case "is":
				opValue = "="
			case "not":
				opValue = "neq"
			}
		}
	}

	// Build and return the expression
	if opValue == "=" {
		return goqu.Ex{prefix + filterQuery.ColumnName: actualvalue}, nil
	} else if opValue == "#" {
		return actualvalue.(goqu.Expression), nil
	} else {
		return goqu.Ex{
			prefix + filterQuery.ColumnName: goqu.Op{
				opValue: actualvalue,
			},
		}, nil
	}
}

// Process fuzzy search with database-specific implementations
func (dbResource *DbResource) processFuzzySearch(filterQuery Query, prefix string, transaction *sqlx.Tx) (goqu.Expression, error) {
	searchValue := fmt.Sprintf("%v", filterQuery.Value)
	keywords := strings.Fields(searchValue) // Split by whitespace

	if len(keywords) == 0 {
		return nil, nil
	}

	// Check if column name contains comma-separated columns for multi-column search
	columns := strings.Split(filterQuery.ColumnName, ",")
	for i := range columns {
		columns[i] = strings.TrimSpace(columns[i])
	}

	dbType := dbResource.Connection().DriverName()

	switch dbType {
	case "postgres":
		return dbResource.processFuzzySearchPostgres(filterQuery, keywords, columns, prefix)
	case "mysql":
		return dbResource.processFuzzySearchMySQL(filterQuery, keywords, columns, prefix)
	case "sqlite3", "sqlite":
		return dbResource.processFuzzySearchSQLite(filterQuery, keywords, columns, prefix)
	case "mssql", "sqlserver":
		return dbResource.processFuzzySearchMSSQL(filterQuery, keywords, columns, prefix)
	default:
		// Generic fallback using LIKE
		return dbResource.processFuzzySearchGeneric(filterQuery, keywords, columns, prefix)
	}
}

// PostgreSQL fuzzy search - using ILIKE for case-insensitive pattern matching (no extensions required)
func (dbResource *DbResource) processFuzzySearchPostgres(filterQuery Query, keywords []string, columns []string, prefix string) (goqu.Expression, error) {
	// AWS RDS PostgreSQL - use standard SQL features without extensions
	// Determine fallback mode - for PostgreSQL without extensions, use ILIKE
	fallbackMode := "ilike" // default to case-insensitive pattern matching
	if filterQuery.FuzzyOptions != nil && filterQuery.FuzzyOptions.FallbackMode != "" {
		fallbackMode = filterQuery.FuzzyOptions.FallbackMode
	}

	var allExpressions []goqu.Expression

	for _, keyword := range keywords {
		var columnExpressions []goqu.Expression

		// For each keyword, check ALL specified columns
		for _, col := range columns {
			switch fallbackMode {
			case "partial":
				// Generate partial patterns for prefix matching
				patterns := generatePartialPatterns(keyword)
				var patternExprs []goqu.Expression
				for _, pattern := range patterns {
					// Use ILIKE for case-insensitive matching in PostgreSQL
					patternExprs = append(patternExprs,
						goqu.Ex{prefix + col: goqu.Op{"iLike": fmt.Sprintf("%%%s%%", pattern)}})
				}
				if len(patternExprs) > 1 {
					columnExpressions = append(columnExpressions, goqu.Or(patternExprs...))
				} else if len(patternExprs) == 1 {
					columnExpressions = append(columnExpressions, patternExprs[0])
				}
			case "word_boundary":
				// Match whole words using PostgreSQL regex (~* for case-insensitive)
				columnExpressions = append(columnExpressions,
					goqu.L(fmt.Sprintf("%s ~* ?", prefix+col),
						fmt.Sprintf("\\y%s\\y", keyword))) // \y is word boundary in PostgreSQL
			case "prefix":
				// Match words starting with the keyword
				columnExpressions = append(columnExpressions,
					goqu.Ex{prefix + col: goqu.Op{"iLike": fmt.Sprintf("%s%%", keyword)}})
			case "suffix":
				// Match words ending with the keyword
				columnExpressions = append(columnExpressions,
					goqu.Ex{prefix + col: goqu.Op{"iLike": fmt.Sprintf("%%%s", keyword)}})
			default: // "ilike" or fallback
				// Simple ILIKE for case-insensitive substring matching
				columnExpressions = append(columnExpressions,
					goqu.Ex{prefix + col: goqu.Op{"iLike": fmt.Sprintf("%%%s%%", keyword)}})
			}
		}

		// OR between columns for same keyword
		if len(columnExpressions) == 1 {
			allExpressions = append(allExpressions, columnExpressions[0])
		} else if len(columnExpressions) > 1 {
			allExpressions = append(allExpressions, goqu.Or(columnExpressions...))
		}
	}

	// Combine based on operator
	if len(allExpressions) == 0 {
		return nil, nil
	}

	if filterQuery.Operator == "fuzzy_any" {
		// ANY keyword in ANY column
		return goqu.Or(allExpressions...), nil
	} else if filterQuery.Operator == "fuzzy_all" {
		// ALL keywords must appear (each can be in different columns)
		return goqu.And(allExpressions...), nil
	} else { // single fuzzy
		return allExpressions[0], nil
	}
}

// MySQL fuzzy search with fallbacks
func (dbResource *DbResource) processFuzzySearchMySQL(filterQuery Query, keywords []string, columns []string, prefix string) (goqu.Expression, error) {
	fallbackMode := "partial" // default
	if filterQuery.FuzzyOptions != nil && filterQuery.FuzzyOptions.FallbackMode != "" {
		fallbackMode = filterQuery.FuzzyOptions.FallbackMode
	}

	var allExpressions []goqu.Expression

	for _, keyword := range keywords {
		var columnExpressions []goqu.Expression

		for _, col := range columns {
			switch fallbackMode {
			case "soundex":
				// Use MySQL SOUNDEX for phonetic matching
				columnExpressions = append(columnExpressions,
					goqu.L(fmt.Sprintf("SOUNDEX(%s) = SOUNDEX(?)", prefix+col), keyword))
			case "partial":
				// Use partial matching
				patterns := generatePartialPatterns(keyword)
				var patternExprs []goqu.Expression
				for _, pattern := range patterns {
					patternExprs = append(patternExprs,
						goqu.Ex{prefix + col: goqu.Op{"like": fmt.Sprintf("%%%s%%", pattern)}})
				}
				columnExpressions = append(columnExpressions, goqu.Or(patternExprs...))
			default: // "strict"
				// Simple LIKE matching
				columnExpressions = append(columnExpressions,
					goqu.Ex{prefix + col: goqu.Op{"like": fmt.Sprintf("%%%s%%", keyword)}})
			}
		}

		if len(columnExpressions) == 1 {
			allExpressions = append(allExpressions, columnExpressions[0])
		} else {
			allExpressions = append(allExpressions, goqu.Or(columnExpressions...))
		}
	}

	if filterQuery.Operator == "fuzzy_any" {
		return goqu.Or(allExpressions...), nil
	} else if filterQuery.Operator == "fuzzy_all" {
		return goqu.And(allExpressions...), nil
	} else {
		return allExpressions[0], nil
	}
}

// SQLite fuzzy search with case-insensitive LIKE
func (dbResource *DbResource) processFuzzySearchSQLite(filterQuery Query, keywords []string, columns []string, prefix string) (goqu.Expression, error) {
	fallbackMode := "partial"
	if filterQuery.FuzzyOptions != nil && filterQuery.FuzzyOptions.FallbackMode != "" {
		fallbackMode = filterQuery.FuzzyOptions.FallbackMode
	}

	var allExpressions []goqu.Expression

	for _, keyword := range keywords {
		var columnExpressions []goqu.Expression

		for _, col := range columns {
			if fallbackMode == "partial" {
				// SQLite LIKE is case-insensitive by default
				patterns := generatePartialPatterns(strings.ToLower(keyword))
				var patternExprs []goqu.Expression
				for _, pattern := range patterns {
					patternExprs = append(patternExprs,
						goqu.L(fmt.Sprintf("LOWER(%s) LIKE ?", prefix+col),
							fmt.Sprintf("%%%s%%", strings.ToLower(pattern))))
				}
				columnExpressions = append(columnExpressions, goqu.Or(patternExprs...))
			} else {
				// Simple case-insensitive LIKE
				columnExpressions = append(columnExpressions,
					goqu.L(fmt.Sprintf("LOWER(%s) LIKE ?", prefix+col),
						fmt.Sprintf("%%%s%%", strings.ToLower(keyword))))
			}
		}

		if len(columnExpressions) == 1 {
			allExpressions = append(allExpressions, columnExpressions[0])
		} else {
			allExpressions = append(allExpressions, goqu.Or(columnExpressions...))
		}
	}

	if filterQuery.Operator == "fuzzy_any" {
		return goqu.Or(allExpressions...), nil
	} else if filterQuery.Operator == "fuzzy_all" {
		return goqu.And(allExpressions...), nil
	} else {
		return allExpressions[0], nil
	}
}

// MSSQL fuzzy search
func (dbResource *DbResource) processFuzzySearchMSSQL(filterQuery Query, keywords []string, columns []string, prefix string) (goqu.Expression, error) {
	fallbackMode := "partial"
	if filterQuery.FuzzyOptions != nil && filterQuery.FuzzyOptions.FallbackMode != "" {
		fallbackMode = filterQuery.FuzzyOptions.FallbackMode
	}

	var allExpressions []goqu.Expression

	for _, keyword := range keywords {
		var columnExpressions []goqu.Expression

		for _, col := range columns {
			if fallbackMode == "soundex" {
				// Use SOUNDEX DIFFERENCE for similarity
				columnExpressions = append(columnExpressions,
					goqu.L(fmt.Sprintf("DIFFERENCE(%s, ?) >= 3", prefix+col), keyword))
			} else {
				// Use LIKE with wildcards
				columnExpressions = append(columnExpressions,
					goqu.Ex{prefix + col: goqu.Op{"like": fmt.Sprintf("%%%s%%", keyword)}})
			}
		}

		if len(columnExpressions) == 1 {
			allExpressions = append(allExpressions, columnExpressions[0])
		} else {
			allExpressions = append(allExpressions, goqu.Or(columnExpressions...))
		}
	}

	if filterQuery.Operator == "fuzzy_any" {
		return goqu.Or(allExpressions...), nil
	} else if filterQuery.Operator == "fuzzy_all" {
		return goqu.And(allExpressions...), nil
	} else {
		return allExpressions[0], nil
	}
}

// Generic fuzzy search fallback
func (dbResource *DbResource) processFuzzySearchGeneric(filterQuery Query, keywords []string, columns []string, prefix string) (goqu.Expression, error) {
	var allExpressions []goqu.Expression

	for _, keyword := range keywords {
		var columnExpressions []goqu.Expression

		for _, col := range columns {
			// Simple LIKE matching as fallback
			columnExpressions = append(columnExpressions,
				goqu.Ex{prefix + col: goqu.Op{"like": fmt.Sprintf("%%%s%%", keyword)}})
		}

		if len(columnExpressions) == 1 {
			allExpressions = append(allExpressions, columnExpressions[0])
		} else {
			allExpressions = append(allExpressions, goqu.Or(columnExpressions...))
		}
	}

	if filterQuery.Operator == "fuzzy_any" {
		return goqu.Or(allExpressions...), nil
	} else if filterQuery.Operator == "fuzzy_all" {
		return goqu.And(allExpressions...), nil
	} else {
		return allExpressions[0], nil
	}
}

// New addFilters implementation with OR support and fuzzy search
func (dbResource *DbResource) addFilters(queryBuilder *goqu.SelectDataset, countQueryBuilder *goqu.SelectDataset,
	queries []Query, prefix string, transaction *sqlx.Tx) (*goqu.SelectDataset, *goqu.SelectDataset, error) {

	if len(queries) == 0 {
		return queryBuilder, countQueryBuilder, nil
	}

	// Group queries by LogicalGroup
	queryGroups := make(map[string][]Query)
	var ungroupedQueries []Query

	for _, q := range queries {
		// Handle fuzzy search operators first
		if q.Operator == "fuzzy_any" || q.Operator == "fuzzy_all" || q.Operator == "fuzzy" {
			// Process fuzzy search separately
			expr, err := dbResource.processFuzzySearch(q, prefix, transaction)
			if err != nil {
				return nil, nil, err
			}
			if expr != nil {
				queryBuilder = queryBuilder.Where(expr)
				countQueryBuilder = countQueryBuilder.Where(expr)
			}
		} else if q.LogicalGroup != "" {
			queryGroups[q.LogicalGroup] = append(queryGroups[q.LogicalGroup], q)
		} else {
			ungroupedQueries = append(ungroupedQueries, q)
		}
	}

	// Process ungrouped queries (maintain backward compatibility - these use AND)
	for _, filterQuery := range ungroupedQueries {
		expr, err := dbResource.processQueryFilter(filterQuery, prefix, transaction)
		if err != nil {
			return nil, nil, err
		}
		if expr != nil {
			queryBuilder = queryBuilder.Where(expr)
			countQueryBuilder = countQueryBuilder.Where(expr)
		}
	}

	// Process grouped queries (OR logic within groups)
	for _, groupQueries := range queryGroups {
		var orExpressions []goqu.Expression

		for _, filterQuery := range groupQueries {
			expr, err := dbResource.processQueryFilter(filterQuery, prefix, transaction)
			if err != nil {
				// Log error but continue with other filters
				log.Warnf("Error processing filter in group: %v", err)
				continue
			}
			if expr != nil {
				orExpressions = append(orExpressions, expr)
			}
		}

		// Apply OR'd expressions as a single WHERE clause
		if len(orExpressions) > 0 {
			queryBuilder = queryBuilder.Where(goqu.Or(orExpressions...))
			countQueryBuilder = countQueryBuilder.Where(goqu.Or(orExpressions...))
		}
	}

	return queryBuilder, countQueryBuilder, nil
}

func (dbResource *DbResource) FindAll(req api2go.Request) (response api2go.Responder, err error) {
	_, ok := req.QueryParams["page[size]"]
	if !ok {
		req.QueryParams["page[size]"] = []string{"1000"}
	}
	_, ok = req.QueryParams["page[number]"]
	if !ok {
		req.QueryParams["page[number]"] = []string{"1"}
	}
	_, responder, e := dbResource.PaginatedFindAll(req)
	return responder, e
}

func (dbResource *DbResource) PaginatedFindAll(req api2go.Request) (totalCount uint, response api2go.Responder, err error) {

	transaction, err := dbResource.Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [1434]")
		return 0, nil, err
	}
	defer transaction.Commit()

	for _, bf := range dbResource.ms.BeforeFindAll {
		//log.Printf("Invoke BeforeFindAll [%v][%v] on FindAll Request", databaseRequestInterceptor.String(), dbResource.model.GetName())
		start := time.Now()
		_, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{}, transaction)
		duration := time.Since(start)
		log.Tracef("[TIMING] FindBeforeFilter %v: %v", bf.String(), duration)

		if err != nil {
			log.Printf("Error from BeforeFindAll middleware [%v]: %v", bf.String(), err)
			transaction.Rollback()
			return 0, NewResponse(nil, err, 400, nil), err
		}
	}
	//log.Printf("Request [%v]: %v", dbResource.model.GetName(), req.QueryParams)

	start := time.Now()
	results, includes, pagination, finalResponseIsSingleObject, err := dbResource.PaginatedFindAllWithoutFilters(req, transaction)
	if err != nil {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "failed to rollback")
		return 0, nil, err
	}
	duration := time.Since(start)
	log.Tracef("[TIMING] FindAllWithoutFilters %v", duration)

	for _, bf := range dbResource.ms.AfterFindAll {
		//log.Printf("Invoke AfterFindAll [%v][%v] on FindAll Request", databaseRequestInterceptor.String(), dbResource.model.GetName())

		start := time.Now()
		results, err = bf.InterceptAfter(dbResource, &req, results, transaction)
		duration := time.Since(start)
		log.Tracef("[TIMING] FindAfterFilter %v: %v", bf.String(), duration)

		if err != nil {
			log.Errorf("Error from findall paginated create middleware: %v", err)
			rollbackErr := transaction.Rollback()
			CheckErr(rollbackErr, "failed to rollback")

			log.Errorf("[1500] Error from AfterFindAll[%v] middleware: %v", bf.String(), err)
			return 0, nil, err
		}
	}

	includesNew := make([][]map[string]interface{}, 0)
	includesNew = append(includesNew, includes...)
	for _, bf := range dbResource.ms.AfterFindAll {
		log.Tracef("Invoke AfterFindAll Includes [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())

		includesNewUpdated := make([][]map[string]interface{}, 0)
		for _, include := range includesNew {
			include, err = bf.InterceptAfter(dbResource, &req, include, transaction)
			if err != nil {
				rollbackErr := transaction.Rollback()
				CheckErr(rollbackErr, "failed to rollback")
				log.Errorf("[1514] Error from AfterFindAll[includes][%v] middleware: %v", bf.String(), err)
				return 0, nil, err
			}
			includesNewUpdated = append(includesNewUpdated, include)
		}
		includesNew = includesNewUpdated

	}

	log.Tracef("Commit transaction in PaginatedFindAll")
	commitErr := transaction.Commit()
	if commitErr != nil {
		CheckErr(commitErr, "Failed to commit")
		return 0, nil, commitErr
	}

	result := make([]api2go.Api2GoModel, 0)
	infos := dbResource.model.GetColumns()

	for i, res := range results {
		delete(res, "id")
		includes := includesNew[i]
		var a = api2go.NewApi2GoModelWithData(dbResource.model.GetTableName(),
			infos, dbResource.model.GetDefaultPermission(), dbResource.model.GetRelations(), res)

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
			model := api2go.NewApi2GoModelWithData(incType, dbResource.Cruds[incType].model.GetColumns(), int64(perm), dbResource.Cruds[incType].model.GetRelations(), include)

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

func (dbResource *DbResource) PaginatedFindAllWithTransaction(req api2go.Request, transaction *sqlx.Tx) (totalCount uint, response api2go.Responder, err error) {

	for _, bf := range dbResource.ms.BeforeFindAll {
		//log.Printf("Invoke BeforeFindAll [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())
		start := time.Now()
		_, err := bf.InterceptBefore(dbResource, &req, []map[string]interface{}{}, transaction)
		duration := time.Since(start)
		log.Tracef("[TIMING] FindBeforeFilter %v: %v", bf.String(), duration)

		if err != nil {
			log.Printf("Error from BeforeFindAll middleware [%v]: %v", bf.String(), err)
			return 0, NewResponse(nil, err, 400, nil), err
		}
	}
	//log.Printf("Request [%v]: %v", dbResource.model.GetName(), req.QueryParams)

	start := time.Now()
	results, includes, pagination, finalResponseIsSingleObject, err := dbResource.PaginatedFindAllWithoutFilters(req, transaction)
	duration := time.Since(start)
	log.Tracef("[TIMING] FindAllWithoutFilters %v", duration)

	for _, bf := range dbResource.ms.AfterFindAll {
		//log.Printf("Invoke AfterFindAll [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())

		start := time.Now()
		results, err = bf.InterceptAfter(dbResource, &req, results, transaction)
		duration := time.Since(start)
		log.Tracef("[TIMING] FindAfterFilter %v: %v", bf.String(), duration)

		if err != nil {
			//log.Errorf("Error from findall paginated create middleware: %v", err)
			log.Errorf("Error from AfterFindAll[%v] middleware: %v", bf.String(), err)
		}
	}

	includesNew := make([][]map[string]interface{}, 0)
	includesNew = append(includesNew, includes...)
	for _, bf := range dbResource.ms.AfterFindAll {
		log.Tracef("Invoke AfterFindAll Includes [%v][%v] on FindAll Request", bf.String(), dbResource.model.GetName())

		includesNewUpdated := make([][]map[string]interface{}, 0)
		for _, include := range includesNew {
			include, err = bf.InterceptAfter(dbResource, &req, include, transaction)
			if err != nil {
				rollbackErr := transaction.Rollback()
				CheckErr(rollbackErr, "failed to rollback")
				log.Errorf("[1514] Error from AfterFindAll[includes][%v] middleware: %v", bf.String(), err)
				return 0, nil, err
			}
			includesNewUpdated = append(includesNewUpdated, include)
		}
		includesNew = includesNewUpdated

	}

	result := make([]api2go.Api2GoModel, 0)
	infos := dbResource.model.GetColumns()

	for i, res := range results {
		delete(res, "id")
		includes := includesNew[i]
		var a = api2go.NewApi2GoModelWithData(dbResource.model.GetTableName(),
			infos, dbResource.model.GetDefaultPermission(), dbResource.model.GetRelations(), res)

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
			model := api2go.NewApi2GoModelWithData(incType, dbResource.Cruds[incType].model.GetColumns(), int64(perm), dbResource.Cruds[incType].model.GetRelations(), include)

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
