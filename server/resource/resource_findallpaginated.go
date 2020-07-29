package resource

import (
	"fmt"
	"github.com/daptin/daptin/server/auth"
	"strconv"
	"strings"

	"encoding/base64"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/statementbuilder"
	log "github.com/sirupsen/logrus"
	"net/url"
)

func (dr *DbResource) GetTotalCount() uint64 {
	s, v, err := statementbuilder.Squirrel.Select("count(*)").From(dr.model.GetName()).ToSql()
	if err != nil {
		log.Errorf("Failed to generate count query for %v: %v", dr.model.GetName(), err)
		return 0
	}

	var count uint64
	err = dr.db.QueryRowx(s, v...).Scan(&count)
	CheckErr(err, "Failed to execute total count query [%s] [%v]", s, v)
	//log.Infof("Count: [%v] %v", dr.model.GetTableName(), count)
	return count
}

func (dr *DbResource) GetTotalCountBySelectBuilder(builder squirrel.SelectBuilder) uint64 {

	s, v, err := builder.ToSql()
	//log.Infof("Count query: %v == %v", s, v)
	if err != nil {
		log.Errorf("Failed to generate count query for %v: %v", dr.model.GetName(), err)
		return 0
	}

	var count uint64
	err = dr.db.QueryRowx(s, v...).Scan(&count)
	if err != nil {
		log.Errorf("Failed to execute count query [%v] %v", s, err)
	}
	//log.Infof("Count: [%v] %v", dr.model.GetTableName(), count)
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

// PaginatedFindAll(req Request) (totalCount uint, response Responder, err error)
func (dr *DbResource) PaginatedFindAllWithoutFilters(req api2go.Request) ([]map[string]interface{}, [][]map[string]interface{}, *PaginationData, error) {
	//log.Infof("Find all row by params: [%v]: %v", dr.model.GetName(), req.QueryParams)
	var err error

	user := req.PlainRequest.Context().Value("user")
	sessionUser := &auth.SessionUser{}

	if user != nil {
		sessionUser = user.(*auth.SessionUser)
	}

	adminId := dr.GetAdminReferenceId()
	isAdmin := adminId != "" && adminId == sessionUser.UserReferenceId

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
	} else if dr.tableInfo.DefaultOrder != "" {
		sortOrder = strings.Split(dr.tableInfo.DefaultOrder, ",")
	}

	var filters []string

	if len(req.QueryParams["filter"]) > 0 && len(queries) == 0 {
		filters = req.QueryParams["filter"]

		for i, q := range filters {
			unescaped, _ := url.QueryUnescape(q)
			filters[i] = unescaped
		}
	}

	//filters := []string{}

	//if len(req.QueryParams["filter"]) > 0 {
	//	filters = req.QueryParams["filter"]
	//}

	if pageNumber > 0 {
		pageNumber = pageNumber * pageSize
	}

	tableModel := dr.model
	//log.Infof("Get all resource type: %v\n", tableModel)

	cols := tableModel.GetColumns()
	finalCols := make([]string, 0)
	//log.Infof("Cols: %v", cols)

	prefix := dr.model.GetName() + "."
	if hasRequestedFields {

		for _, col := range cols {
			if !col.ExcludeFromApi && reqFieldMap[col.Name] && col.ColumnName != "permission" && col.ColumnName != "reference_id" {
				finalCols = append(finalCols, col.ColumnName)
			}
		}
	} else {
		for _, col := range cols {
			if col.ExcludeFromApi || col.ColumnName == "permission" || col.ColumnName == "reference_id" || col.ColumnName == "id" {
				continue
			}
			finalCols = append(finalCols, col.ColumnName)
		}
	}

	if _, ok := req.QueryParams["usergroup_id"]; ok && req.QueryParams["usergroupName"][0] == dr.model.GetName()+"_id" {
		isRelatedGroupRequest = true
		if relatedTableName == "" {
			relatedTableName = dr.model.GetTableName()
		}
	}

	idColumn := fmt.Sprintf("%s.id", tableModel.GetTableName())
	distinctIdColumn := fmt.Sprintf("distinct(%s.id)", tableModel.GetTableName())
	if isRelatedGroupRequest {
		//log.Infof("Switch permission to join table j1 instead of %v%v", prefix, "permission")
		if dr.model.GetName() == "usergroup" {
			finalCols = append(finalCols, fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.permission", relatedTableName, relatedTableName))
			finalCols = append(finalCols, fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.reference_id as reference_id", relatedTableName, relatedTableName))
			finalCols = append(finalCols, "usergroup.reference_id as relation_reference_id")
		} else {
			finalCols = append(finalCols, "usergroup_id.permission")
			finalCols = append(finalCols, fmt.Sprintf("%s.reference_id as relation_reference_id", relatedTableName))
			finalCols = append(finalCols, fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id.reference_id as reference_id", relatedTableName, relatedTableName))
			joinTableName := fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id", relatedTableName, relatedTableName)
			distinctIdColumn = fmt.Sprintf("distinct(%s.id)", joinTableName)
			idColumn = fmt.Sprintf("%s.id", joinTableName)
		}
		//		finalCols = append(finalCols, prefix+"reference_id as reference_id")
	} else {
		finalCols = append(finalCols, prefix+"permission")
		finalCols = append(finalCols, prefix+"reference_id")

	}

	idQueryCols := []string{distinctIdColumn}
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
		idQueryCols = append(idQueryCols, sort)
	}
	queryBuilder := statementbuilder.Squirrel.Select(idQueryCols...).From(tableModel.GetTableName())
	//queryBuilder = queryBuilder.From(tableModel.GetTableName())

	joinTableName := fmt.Sprintf("%s_%s_id_has_usergroup_usergroup_id", tableModel.GetTableName(), tableModel.GetTableName())
	if !isRelatedGroupRequest && tableModel.GetTableName() != "usergroup" {
		queryBuilder = queryBuilder.LeftJoin(
			fmt.Sprintf("%s %s on %s.id=%s.%s_id",
				joinTableName, joinTableName, tableModel.GetTableName(), joinTableName, tableModel.GetTableName(),
			))
	}

	var countQueryBuilder squirrel.SelectBuilder
	countQueryBuilder = statementbuilder.Squirrel.Select("count(*)").From(tableModel.GetTableName()).Offset(0).Limit(1)
	if req.QueryParams["page[after]"] != nil && len(req.QueryParams["page[after]"]) > 0 {
		id, err := dr.GetReferenceIdToId(dr.TableInfo().TableName, req.QueryParams["page[after]"][0])
		if err != nil {
			queryBuilder = queryBuilder.Where(squirrel.Gt{
				dr.TableInfo().TableName + ".id": id,
			}).Limit(pageSize)
		}
	} else if req.QueryParams["page[before]"] != nil && len(req.QueryParams["page[before]"]) > 0 {
		id, err := dr.GetReferenceIdToId(dr.TableInfo().TableName, req.QueryParams["page[before]"][0])
		if err != nil {
			queryBuilder = queryBuilder.Where(squirrel.Lt{
				dr.TableInfo().TableName + ".id": id,
			}).Limit(pageSize)
		}
	} else {
		queryBuilder = queryBuilder.Offset(pageNumber).Limit(pageSize)
	}
	joins := make([]string, 0)
	joinFilters := make([]interface{}, 0)

	infos := dr.model.GetColumns()

	// todo: fix search in findall operation. currently no way to do an " or " query
	if len(filters) > 0 {

		colsToAdd := make([]string, 0)
		wheres := make([]interface{}, 0)

		for _, col := range infos {
			if col.IsIndexed && col.ColumnType == "name" || col.ColumnType == "label" || col.ColumnType == "email" {
				colsToAdd = append(colsToAdd, col.ColumnName)
			}
		}

		if len(colsToAdd) > 0 {
			colString := make([]string, 0)
			for _, q := range filters {
				if len(q) < 1 {
					continue
				}

				for _, c := range colsToAdd {
					colString = append(colString, fmt.Sprintf("%v like ?", prefix+c))
					wheres = append(wheres, fmt.Sprint("%", q, "%"))
				}
			}
			if len(colString) > 0 {
				queryBuilder = queryBuilder.Where("( "+strings.Join(colString, " or ")+")", wheres...)
				countQueryBuilder = countQueryBuilder.Where("( "+strings.Join(colString, " or ")+")", wheres...)
			}
		}
	}

	queryBuilder = addFilters(queryBuilder, queries, prefix)

	//if len(groupings) > 0 && false {
	//	for _, groupBy := range groupings {
	//		queryBuilder = queryBuilder.GroupBy(fmt.Sprintf("%s %s", groupBy.ColumnName, groupBy.Order))
	//	}
	//}

	for _, rel := range dr.model.GetRelations() {

		if rel.GetSubject() == dr.model.GetName() {

			queries, ok := req.QueryParams[rel.GetObjectName()]
			if !ok {
				queries, ok = req.QueryParams[rel.GetObject()+"_id"]
			}
			if !ok || len(queries) < 1 {
				continue
			}
			log.Infof("Forward Relation %v", rel.String())

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

				ids, err := dr.GetSingleColumnValueByReferenceId(rel.GetObject(), []string{"id"}, "reference_id", queries)
				//log.Infof("Converted ids: %v", ids)
				if err != nil {
					log.Errorf("Failed to convert refids to ids [%v][%v]: %v", rel.GetObject(), queries, err)
					return nil, nil, nil, err
				}
				switch rel.Relation {
				case "has_one":
					if len(ids) < 1 {
						continue
					}
					queryBuilder = queryBuilder.Where(squirrel.Eq{rel.GetObjectName(): ids})
					countQueryBuilder = countQueryBuilder.Where(squirrel.Eq{rel.GetObjectName(): ids})
					break

				case "belongs_to":
					queryBuilder = queryBuilder.Where(squirrel.Eq{rel.GetObjectName(): ids})
					countQueryBuilder = countQueryBuilder.Where(squirrel.Eq{rel.GetObjectName(): ids})
					break

				case "has_many":
					wh := squirrel.Eq{}
					wh[rel.GetObjectName()+".id"] = ids
					queryBuilder = queryBuilder.Join(rel.GetJoinString()).Where(wh)
					countQueryBuilder = countQueryBuilder.Join(rel.GetJoinString()).Where(wh)
					joins = append(joins, rel.GetJoinString())
					joinFilters = append(joinFilters, wh)
				}
			}
		}
		if rel.GetObject() == dr.model.GetName() {

			subjectNameList, ok := req.QueryParams[rel.GetSubject()+"Name"]
			if !ok {
				continue
			}
			//log.Infof("Reverse Relation %v", rel.String())

			var subjectName string
			/**
			api2go give us two params for each relationship
			<entityName> -> the name of the column which is used to reference, usually <entity>_id but you name it something for special relations in the config
			*/
			subjectName = subjectNameList[0]
			if subjectName != rel.GetObjectName() {
				continue
			}

			switch rel.Relation {
			case "has_one":

				subjectId := req.QueryParams[rel.GetSubject()+"_id"]
				if len(subjectId) < 1 {
					continue
				}
				queryBuilder = queryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.GetSubjectName() + ".reference_id": subjectId})
				countQueryBuilder = countQueryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.GetSubjectName() + ".reference_id": subjectId})
				joins = append(joins, rel.GetReverseJoinString())
				joinFilters = append(joinFilters, squirrel.Eq{rel.GetSubjectName() + ".reference_id": subjectId})
				break

			case "belongs_to":

				queries, ok := req.QueryParams[rel.GetSubject()+"_id"]
				//log.Infof("%d Values as RefIds for relation [%v]", len(filters), rel.String())
				if !ok || len(queries) < 1 {
					continue
				}
				ids, err := dr.GetSingleColumnValueByReferenceId(rel.GetSubject(), []string{"id"}, "reference_id", queries)
				if err != nil {
					log.Errorf("Failed to convert [%v]refids to ids[%v]: %v", rel.GetSubject(), queries, err)
					return nil, nil, nil, err
				}

				queryBuilder = queryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.GetSubjectName() + ".id": ids})
				countQueryBuilder = countQueryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.GetSubjectName() + ".id": ids})
				joins = append(joins, rel.GetReverseJoinString())
				joinFilters = append(joinFilters, squirrel.Eq{rel.GetSubjectName() + ".id": ids})
				break
			case "has_many":
				subjectId := req.QueryParams[rel.GetSubject()+"_id"]
				if len(subjectId) < 1 {
					continue
				}
				//log.Infof("Has many [%v] : [%v] === %v", dr.model.GetName(), subjectId, req.QueryParams)
				queryBuilder = queryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.GetSubjectName() + ".reference_id": subjectId})
				countQueryBuilder = countQueryBuilder.Join(rel.GetReverseJoinString()).Where(squirrel.Eq{rel.GetSubjectName() + ".reference_id": subjectId})
				joins = append(joins, rel.GetReverseJoinString())
				joinFilters = append(joinFilters, squirrel.Eq{rel.GetSubjectName() + ".reference_id": subjectId})

			}

		}
	}

	orders := make([]string, 0)
	for _, so := range sortOrder {

		if len(so) < 1 {
			continue
		}
		//log.Infof("Sort order: %v", so)
		if so[0] == '-' {
			ord := prefix + so[1:] + " desc"
			// queryBuilder = queryBuilder.OrderBy(ord)
			// countQueryBuilder = countQueryBuilder.OrderBy(ord)
			orders = append(orders, ord)
		} else {
			if so[0] == '+' {
				ord := prefix + so[1:] + " asc"
				// queryBuilder = queryBuilder.OrderBy(ord)
				// countQueryBuilder = countQueryBuilder.OrderBy(ord)
				orders = append(orders, ord)
			} else {
				ord := prefix + so + " asc"
				if strings.ToLower(so) == "rand()" || strings.ToLower(so) == "random()" {
					ord = so
				}
				// queryBuilder = queryBuilder.OrderBy(ord)
				// countQueryBuilder = countQueryBuilder.OrderBy(ord)
				orders = append(orders, ord)
			}
		}
	}

	if !isAdmin && tableModel.GetTableName() != "usergroup" {
		queryBuilder = queryBuilder.Where(fmt.Sprintf("(((%s.permission & 2) = 2) or "+
			"((%s.permission & 32768) = 32768) or "+
			"(%s.user_account_id = ? and (%s.permission & 256) = 256))", tableModel.GetTableName(), joinTableName, tableModel.GetTableName(), tableModel.GetTableName()), sessionUser.UserId)
		countQueryBuilder = countQueryBuilder.Where(fmt.Sprintf("(((%s.permission & 2) = 2) or "+
		//"((%s.permission & 32768) = 32768) or "+
			"(%s.user_account_id = ? and (%s.permission & 256) = 256))", tableModel.GetTableName(), tableModel.GetTableName(), tableModel.GetTableName()), sessionUser.UserId)
	}

	idsListQuery, args, err := queryBuilder.OrderBy(orders...).ToSql()
	if err != nil {
		return nil, nil, nil, err
	}
	log.Printf("Id query: [%s]", idsListQuery)
	//log.Printf("Id query args: %v", args)
	stmt, err := dr.connection.Preparex(idsListQuery)
	if err != nil {
		log.Infof("Findall select query sql: %v == %v", idsListQuery, args)
		log.Errorf("Failed to prepare sql: %v", err)
		return nil, nil, nil, err
	}
	idsRow, err := stmt.Queryx(args...)
	if err != nil {
		log.Infof("Findall select query sql: %v == %v", idsListQuery, args)
		log.Errorf("Failed to prepare sql: %v", err)
		return nil, nil, nil, err
	}
	ids := make([]int64, 0)

	for idsRow.Next() {
		row := make(map[string]interface{})
		err = idsRow.MapScan(row)
		if err != nil {
			return nil, nil, nil, err
		}
		ids = append(ids, row["id"].(int64))
	}

	if len(languagePreferences) == 0 {

		for i, col := range finalCols {
			if strings.Index(col, ".") == -1 {
				finalCols[i] = prefix + col
			}
		}

		queryBuilder = statementbuilder.Squirrel.Select(finalCols...).From(tableModel.GetTableName()).Where(squirrel.Eq{
			idColumn: ids,
		}).OrderBy(orders...)
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
		for i, colName := range finalCols {
			if IsStandardColumn(colName) {
				finalCols[i] = prefix + colName
			} else {
				if strings.Index(colName, ".") == -1 {
					finalCols[i] = ifNullFunctionName + "(" + translateTableName + "." + colName + "," + prefix + colName + ") as " + colName
				} else {
					finalCols[i] = colName
				}
			}
		}

		queryBuilder = statementbuilder.Squirrel.Select(finalCols...).From(tableModel.GetTableName()).
			LeftJoin(translateTableName +
				" on " + translateTableName + ".translation_reference_id = " + tableModel.GetTableName() + ".id" +
				" and " + translateTableName + ".language_id = " + "'" + preferredLanguage + "'").Where(squirrel.Eq{
			idColumn: ids,
		}).OrderBy(orders...)

	}

	if len(joins) > 0 {
		for _, j := range joins {
			queryBuilder = queryBuilder.Join(j)
		}
		for _, w := range joinFilters {
			queryBuilder = queryBuilder.Where(w)
		}
	}

	sql1, args, err := queryBuilder.ToSql()
	//log.Printf("Query: %v == %v", sql1, args)

	if err != nil {
		log.Infof("Error: %v", err)
		return nil, nil, nil, err
	}

	stmt, err = dr.connection.Preparex(sql1)
	if err != nil {
		log.Infof("Findall select query sql: %v == %v", sql1, args)
		log.Errorf("Failed to prepare sql: %v", err)
		return nil, nil, nil, err
	}
	defer func() {
		err = stmt.Close()
		CheckErr(err, "Failed to close statement")
	}()
	rows, err := stmt.Queryx(args...)

	if err != nil {
		log.Infof("Error: %v", err)
		return nil, nil, nil, err
	}
	defer func() {
		err = rows.Close()
		CheckErr(err, "Failed to close rows")
	}()

	//log.Infof("Included relations: %v", includedRelations)
	results, includes, err := dr.ResultToArrayOfMap(rows, dr.model.GetColumnMap(), includedRelations)
	//log.Infof("Found: %d results", len(results))
	//log.Infof("Results: %v", results)

	total1 := dr.GetTotalCountBySelectBuilder(countQueryBuilder)

	if pageSize < 1 {
		pageSize = 10
	}

	paginationData := &PaginationData{
		PageNumber: pageNumber,
		PageSize:   pageSize,
		TotalCount: total1,
	}

	return results, includes, paginationData, err

}

func addFilters(queryBuilder squirrel.SelectBuilder, queries []Query, prefix string) squirrel.SelectBuilder {

	if len(queries) == 0 {
		return queryBuilder
	}

	for _, filterQuery := range queries {
		switch filterQuery.Operator {
		case "contains":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s like ?", prefix+filterQuery.ColumnName), "%"+fmt.Sprintf("%v", filterQuery.Value)+"%")
		case "not contains":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("(%s not like ? or %s is null)", prefix+filterQuery.ColumnName, prefix+filterQuery.ColumnName), "%"+fmt.Sprintf("%v", filterQuery.Value)+"%")
		case "is":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s = ?", prefix+filterQuery.ColumnName), filterQuery.Value)
		case "in":
			//queryBuilder = queryBuilder.Where(fmt.Sprintf("%s in (?)", prefix+filterQuery.ColumnName), filterQuery.Value)
			queryBuilder.Where(squirrel.Eq{prefix + filterQuery.ColumnName: filterQuery.Value.([]string)})
		case "is not":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s != ?", prefix+filterQuery.ColumnName), filterQuery.Value)
		case "before":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s < ?", prefix+filterQuery.ColumnName), filterQuery.Value)
		case "after":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s > ?", prefix+filterQuery.ColumnName), filterQuery.Value)
		case "more then":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s > ?", prefix+filterQuery.ColumnName), filterQuery.Value)
		case "any of":
			vals := strings.Split(fmt.Sprintf("%v", filterQuery.Value), ",")
			valsInterface := make([]interface{}, len(vals))
			for i, v := range vals {
				valsInterface[i] = v
			}
			questions := strings.Join(strings.Split(strings.Repeat("?", len(vals)), ""), ", ")
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s in (%s)", prefix+filterQuery.ColumnName, questions), valsInterface...)
		case "none of":
			vals := strings.Split(fmt.Sprintf("%v", filterQuery.Value), ",")
			valsInterface := make([]interface{}, len(vals))
			for i, v := range vals {
				valsInterface[i] = v
			}
			questions := strings.Join(strings.Split(strings.Repeat("?", len(vals)), ""), ", ")
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s not in (%s)", prefix+filterQuery.ColumnName, questions), valsInterface...)
		case "less then":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s < ?", prefix+filterQuery.ColumnName), filterQuery.Value)
		case "is empty":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s is null or %s = ''", prefix+filterQuery.ColumnName, prefix+filterQuery.ColumnName))
		case "is not empty":
			queryBuilder = queryBuilder.Where(fmt.Sprintf("%s is not null and %s != ''", prefix+filterQuery.ColumnName, prefix+filterQuery.ColumnName))
		}
	}

	return queryBuilder
}

func (dr *DbResource) FindAll(req api2go.Request) (response api2go.Responder, err error) {
	req.QueryParams["page[size]"] = []string{"1000"}
	_, responder, e := dr.PaginatedFindAll(req)
	return responder, e
}

func (dr *DbResource) PaginatedFindAll(req api2go.Request) (totalCount uint, response api2go.Responder, err error) {

	for _, bf := range dr.ms.BeforeFindAll {
		//log.Infof("Invoke BeforeFindAll [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())
		_, err := bf.InterceptBefore(dr, &req, []map[string]interface{}{})
		if err != nil {
			log.Infof("Error from BeforeFindAll middleware [%v]: %v", bf.String(), err)
			return 0, NewResponse(nil, err, 400, nil), err
		}
	}
	//log.Infof("Request [%v]: %v", dr.model.GetName(), req.QueryParams)

	results, includes, pagination, err := dr.PaginatedFindAllWithoutFilters(req)

	for _, bf := range dr.ms.AfterFindAll {
		//log.Infof("Invoke AfterFindAll [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

		results, err = bf.InterceptAfter(dr, &req, results)
		if err != nil {
			//log.Errorf("Error from findall paginated create middleware: %v", err)
			log.Errorf("Error from AfterFindAll[%v] middleware: %v", bf.String(), err)
		}
	}

	includesNew := make([][]map[string]interface{}, 0)
	for _, bf := range dr.ms.AfterFindAll {
		//log.Infof("Invoke AfterFindAll Includes [%v][%v] on FindAll Request", bf.String(), dr.model.GetName())

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

	//log.Infof("Offset, limit: %v, %v", pageNumber, pageSize)

	if pagination == nil {
		pagination = &PaginationData{
			PageNumber: 1,
			PageSize:   10,
		}
	}
	//log.Infof("Pagination :%v", pagination)

	return uint(pagination.TotalCount), NewResponse(nil, result, 200, &api2go.Pagination{
		Next:        map[string]string{"limit": fmt.Sprintf("%v", pagination.PageSize), "offset": fmt.Sprintf("%v", pagination.PageSize+pagination.PageNumber)},
		Prev:        map[string]string{"limit": fmt.Sprintf("%v", pagination.PageSize), "offset": fmt.Sprintf("%v", pagination.PageNumber-pagination.PageSize)},
		First:       map[string]string{},
		Last:        map[string]string{"limit": fmt.Sprintf("%v", pagination.PageSize), "offset": fmt.Sprintf("%v", pagination.TotalCount-pagination.PageSize)},
		Total:       pagination.TotalCount,
		PerPage:     pagination.PageSize,
		CurrentPage: 1 + (pagination.PageNumber / pagination.PageSize),
		LastPage:    1 + (pagination.TotalCount / pagination.PageSize),
		From:        pagination.PageNumber + 1,
		To:          pagination.PageSize,
	}), nil

}
