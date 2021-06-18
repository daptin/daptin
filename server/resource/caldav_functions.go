package resource

import (
	"errors"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"time"
)

func(dr *DbResource) GetRpath(userId int64)(string,error){

	rPath := ""

	cal, _, err := dr.Cruds["calendar"].GetRowsByWhereClause("calendar", nil, goqu.Ex{"user_account_id": userId})
	if err != nil {
		return rPath, err
	}
	if len(cal) < 1 {
		return rPath, errors.New("calendar not found")
	}

	rPath = cal[0]["rPath"].(string)

	return rPath, err
}

func (dr *DbResource) DeleteCalendarEvent(UserId int64, rPath string) error {

	cal, err := dr.Cruds["calendar"].GetAllObjectsWithWhere("calendar",
		goqu.Ex{
			"user_account_id": UserId,
			"rPath": rPath,
		},
	)

	if err != nil || len(cal) == 0 {
		return errors.New("caldav resource does not exist")
	}

	query, args, err := statementbuilder.Squirrel.Delete("calendar").Where(goqu.Ex{"rPath": cal[0]["rPath"]}).ToSQL()
	if err != nil {
		return err
	}

	_, err = dr.db.Exec(query, args...)
	if err != nil {
		return err
	}


	return err

}

func(dr *DbResource) GetModTime(rPath string, userId int64)(time.Time,error){
	modified := time.Now()

	cal, _, err := dr.Cruds["calendar"].GetRowsByWhereClause("calendar", nil, goqu.Ex{"user_account_id": userId},goqu.Ex{"rPath": rPath})
	if err != nil {
		return modified, err
	}

	if len(cal) < 1 {
		return modified, errors.New("calendar not found")
	}

	modified = cal[0]["last_modified"].(time.Time)

	return modified, err
}

func(dr *DbResource) GetContent(rPath string, userId int64)(string,error){
	content := ""

	cal, _, err := dr.Cruds["calendar"].GetRowsByWhereClause("calendar", nil, goqu.Ex{"user_account_id": userId},goqu.Ex{"rPath": rPath})
	if err != nil {
		return content, err
	}

	if len(cal) < 1 {
		return content, errors.New("content not found")
	}

	content = cal[0]["content"].(string)

	return content, err
}

func (d *DbResource) UpdateResource(rPath, newContent string) error {

	cal, err := d.Cruds["calendar"].GetAllObjectsWithWhere("calendar",
		goqu.Ex{
			"rPath": rPath,
		},
	)
	if err != nil || len(cal) == 0 {
		return errors.New("calendar Event does not exist")
	}

	query, args, err := statementbuilder.Squirrel.
		Update("calendar").
		Set(goqu.Record{"content": newContent, "last_modified": time.Now()}).
		Where(goqu.Ex{"id": cal[0]["id"]}).ToSQL()
	if err != nil {
		return err
	}

	_, err = d.db.Exec(query, args...)

	return err

}

func (dr *DbResource) InsertResource(rPath, content string, userId int64) error{

	query, args, err := statementbuilder.Squirrel.Insert("calendar").
		Cols("rPath", "content", "user_account_id").
		Vals([]interface{}{rPath, content, userId}).
		ToSQL()

	if err != nil {
		return err
	}

	_, err = dr.db.Exec(query, args...)
	if err != nil {
		CheckErr(err, "Failed to Insert Calendar Resource: %v == %v", query, args)
		return err
	}

	return nil
	//CheckErr(err, "Failed to Insert Calendar Resource: %v == %v", query, args)
}