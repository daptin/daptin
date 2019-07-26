package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/apiblueprint"
	"github.com/daptin/daptin/server/resource"
	"github.com/disintegration/gift"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/russross/blackfriday.v2"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"strconv"
	"strings"
)

func CreateApiBlueprintHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) func(ctx *gin.Context) {
	return func(c *gin.Context) {
		c.String(200, "%s", apiblueprint.BuildApiBlueprint(initConfig, cruds))
	}
}

type ErrorResponse struct {
	Message string
}

func CreateDbAssetHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) func(*gin.Context) {
	return func(c *gin.Context) {
		var typeName = c.Param("typename")
		var resourceId = c.Param("resource_id")
		var columnNameWithExtension = c.Param("columnname")
		//var extension = c.Param("ext")

		var parts = strings.Split(columnNameWithExtension, ".")
		columnName := parts[0]
		//extension := parts[1]

		table, ok := cruds[typeName]

		if !ok || table == nil {
			c.AbortWithStatus(404)
			return
		}

		colInfo, ok := table.TableInfo().GetColumnByName(columnName)

		if !ok || colInfo == nil || (!colInfo.IsForeignKey && colInfo.ColumnType != "markdown") {
			c.AbortWithStatus(404)
			return
		}

		pr := &http.Request{
			Method: "GET",
		}

		pr = pr.WithContext(c.Request.Context())

		req := api2go.Request{
			PlainRequest: pr,
		}

		obj, err := cruds[typeName].FindOne(resourceId, req)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}

		row := obj.Result().(*api2go.Api2GoModel)
		colData := row.Data[columnName]
		if colData == nil {
			c.AbortWithStatus(404)
			return

		}

		if colInfo.IsForeignKey {

			files, ok := colData.([]map[string]interface{})

			if !ok || len(files) < 1 {
				c.AbortWithStatus(404)
				return
			}

			contentBytes, e := base64.StdEncoding.DecodeString(files[0]["contents"].(string))
			if e != nil {
				c.AbortWithStatus(500)
				return
			}

			filters := make([]gift.Filter, 0)
			for key, values := range c.Request.URL.Query() {
				param := gin.Param{
					Key:   key,
					Value: values[0],
				}

				valueFloat64, floatError := strconv.ParseFloat(param.Value, 32)
				valueFloat32 := float32(valueFloat64)

				switch param.Key {
				case "brightness":
					filters = append(filters, gift.Brightness(valueFloat32))
					break;
				case "colorBalance":
					vals := strings.Split(param.Value, ",")
					if len(vals) != 3 {
						continue
					}

					red, _ := strconv.ParseFloat(vals[0], 32)
					green, _ := strconv.ParseFloat(vals[1], 32)
					blue, _ := strconv.ParseFloat(vals[2], 32)
					filters = append(filters, gift.ColorBalance(float32(red), float32(green), float32(blue)))
					break;
				case "colorize":

					vals := strings.Split(param.Value, ",")
					if len(vals) != 3 {
						continue
					}

					hue, _ := strconv.ParseFloat(vals[0], 32)
					saturattion, _ := strconv.ParseFloat(vals[1], 32)
					percent, _ := strconv.ParseFloat(vals[2], 32)
					filters = append(filters, gift.ColorBalance(float32(hue), float32(saturattion), float32(percent)))
					break;
				case "colorspaceLinearToSRGB":
					break;
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {
						filters = append(filters, gift.ColorspaceLinearToSRGB())
					}
				case "colorspaceSRGBToLinear":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {
						filters = append(filters, gift.ColorspaceSRGBToLinear())
					}
					break;
				case "contrast":
					if floatError == nil {
						filters = append(filters, gift.Contrast(valueFloat32))
					}
					break;
				case "crop":

					vals := strings.Split(param.Value, ",")
					if len(vals) != 4 {
						continue
					}

					minX, _ := strconv.ParseInt(vals[0], 10, 32)
					minY, _ := strconv.ParseInt(vals[1], 10, 32)
					maxX, _ := strconv.ParseInt(vals[2], 10, 32)
					maxY, _ := strconv.ParseInt(vals[3], 10, 32)

					rect := image.Rectangle{
						Min: image.Point{
							X: int(minX),
							Y: int(minY),
						},
						Max: image.Point{
							X: int(maxX),
							Y: int(maxY),
						},
					}
					filters = append(filters, gift.Crop(rect))
					break;
				case "cropToSize":

					vals := strings.Split(param.Value, ",")
					if len(vals) != 3 {
						continue
					}
					height, _ := strconv.ParseInt(vals[0], 10, 32)
					weight, _ := strconv.ParseInt(vals[1], 10, 32)
					anchor := gift.CenterAnchor

					switch vals[2] {
					case "Center":
						anchor = gift.CenterAnchor
						break;
					case "TopLeft":
						anchor = gift.TopLeftAnchor
						break;
					case "Top":
						anchor = gift.TopAnchor
						break;
					case "TopRight":
						anchor = gift.TopRightAnchor
						break;
					case "Left":
						anchor = gift.LeftAnchor
						break;
					case "Right":
						anchor = gift.RightAnchor
						break;
					case "BottomLeft":
						anchor = gift.BottomLeftAnchor
						break;
					case "Bottom":
						anchor = gift.BottomAnchor
						break;
					case "BottomRight":
						anchor = gift.BottomRightAnchor
						break;
					}
					filters = append(filters, gift.CropToSize(int(height), int(weight), anchor))
					break;
				case "flipHorizontal":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {
						filters = append(filters, gift.FlipHorizontal())
					}
					break;
				case "flipVertical":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {
						filters = append(filters, gift.FlipVertical())
					}
					break;
				case "gamma":
					filters = append(filters, gift.Gamma(valueFloat32))
					break;
				case "gaussianBlur":
					filters = append(filters, gift.GaussianBlur(valueFloat32))
					break;
				case "grayscale":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {
						filters = append(filters, gift.Grayscale())
					}
					break;
				case "hue":
					filters = append(filters, gift.Hue(valueFloat32))
					break;
				case "invert":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {

						filters = append(filters, gift.Invert())
					}
					break;
				case "resize":
					vals := strings.Split(param.Value, ",")
					if len(vals) != 4 {
						continue
					}
					height, _ := strconv.ParseInt(vals[0], 10, 32)
					weight, _ := strconv.ParseInt(vals[1], 10, 32)
					resampling := gift.NearestNeighborResampling

					switch vals[2] {
					case "NearestNeighbor":
						resampling = gift.NearestNeighborResampling
						break;
					case "Box":
						resampling = gift.BoxResampling
						break;
					case "Linear":
						resampling = gift.LinearResampling
						break;
					case "Cubic":
						resampling = gift.CubicResampling
						break;
					case "Lanczos":
						resampling = gift.LanczosResampling
						break;
					}
					filters = append(filters, gift.Resize(int(height), int(weight), resampling))
					break;
				case "rotate":

					vals := strings.Split(param.Value, ",")
					if len(vals) != 3 {
						continue
					}
					angle, _ := strconv.ParseFloat(vals[0], 32)
					backgroundColor, _ := ParseHexColor(vals[1])
					interpolation := gift.NearestNeighborInterpolation

					switch vals[2] {
					case "NearestNeighbor":
						interpolation = gift.NearestNeighborInterpolation
						break;
					case "Linear":
						interpolation = gift.LinearInterpolation
						break;
					case "Cubic":
						interpolation = gift.CubicInterpolation
						break;
					}
					filters = append(filters, gift.Rotate(float32(angle), backgroundColor, interpolation))
					break;
				case "rotate180":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {

						filters = append(filters, gift.Rotate180())
					}
					break;
				case "rotate270":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {

						filters = append(filters, gift.Rotate270())
					}
					break;
				case "rotate90":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {

						filters = append(filters, gift.Rotate270())
					}
					break;
				case "saturation":
					filters = append(filters, gift.Saturation(valueFloat32))
					break;
				case "sepia":
					filters = append(filters, gift.Sepia(valueFloat32))
					break;
				case "sobel":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {

						filters = append(filters, gift.Sobel())
					}
					break;
				case "threshold":
					filters = append(filters, gift.Threshold(valueFloat32))

					break;
				case "transpose":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {

						filters = append(filters, gift.Transpose())
					}

					break;
				case "transverse":
					if strings.ToLower(param.Value) == "true" || param.Value == "1" {

						filters = append(filters, gift.Transverse())
					}
					break;

				}

			}

			f := gift.New(filters...)

			img, formatName, err := image.Decode(bytes.NewReader(contentBytes))
			log.Printf("Image format name: %v", formatName)
			if err != nil {
				c.AbortWithStatus(500)
				return
			}
			dst := image.NewNRGBA(f.Bounds(img.Bounds()))
			f.Draw(dst, img)

			c.Writer.Header().Set("Content-Type", "image/"+formatName)
			if formatName == "png" {
				err = png.Encode(c.Writer, dst)
			} else {
				err = jpeg.Encode(c.Writer, dst, nil)
			}
			if err != nil {
				log.Errorf("failed to write converted image :%v", err)
			}

			c.AbortWithStatus(200)

		} else if colInfo.ColumnType == "markdown" {

			outHtml := blackfriday.Run([]byte(colData.(string)))
			c.Writer.Header().Set("Content-Type", "text/html")
			c.Writer.Write(outHtml)
			c.AbortWithStatus(200)

		}

	}
}

func ParseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 4:
		_, err = fmt.Sscanf(s, "#%1x%1x%1x", &c.R, &c.G, &c.B)
		// Double the hex digits:
		c.R *= 17
		c.G *= 17
		c.B *= 17
	default:
		err = fmt.Errorf("invalid length, must be 7 or 4")

	}
	return
}

func CreateStatsHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) func(*gin.Context) {

	return func(c *gin.Context) {

		typeName := c.Param("typename")

		aggReq := resource.AggregationRequest{}

		aggReq.RootEntity = typeName
		aggReq.Filter = c.QueryArray("filter")
		aggReq.GroupBy = c.QueryArray("group")
		aggReq.Join = c.QueryArray("join")
		aggReq.ProjectColumn = c.QueryArray("column")
		aggReq.TimeSample = resource.TimeStamp(c.Query("timesample"))
		aggReq.TimeFrom = c.Query("timefrom")
		aggReq.TimeTo = c.Query("timeto")
		aggReq.Order = c.QueryArray("order")

		aggResponse, err := cruds[typeName].DataStats(aggReq)

		if err != nil {
			c.JSON(500, resource.NewDaptinError("Failed to query stats", "query failed"))
			return
		}

		c.JSON(200, aggResponse)

	}

}

func CreateReclineModelHandler() func(*gin.Context) {

	reclineColumnMap := make(map[string]string)

	for _, column := range resource.ColumnTypes {
		reclineColumnMap[column.Name] = column.ReclineType
	}

	return func(c *gin.Context) {
		c.JSON(200, reclineColumnMap)
	}

}

func CreateMetaHandler(initConfig *resource.CmsConfig) func(*gin.Context) {

	return func(context *gin.Context) {

		query := context.Query("query")

		switch query {
		case "column_types":
			context.JSON(200, resource.ColumnManager.ColumnMap)
		}
	}
}

func CreateJsModelHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) func(*gin.Context) {
	tableMap := make(map[string]resource.TableInfo)
	for _, table := range initConfig.Tables {

		//log.Infof("Default permission for [%v]: [%v]", table.TableName, table.Columns)

		tableMap[table.TableName] = table
	}

	streamMap := make(map[string]resource.StreamContract)
	for _, stream := range initConfig.Streams {
		streamMap[stream.StreamName] = stream
	}

	worlds, _, err := cruds["world"].GetRowsByWhereClause("world")
	if err != nil {
		log.Errorf("Failed to get worlds list")
	}

	worldToReferenceId := make(map[string]string)

	for _, world := range worlds {
		worldToReferenceId[world["table_name"].(string)] = world["reference_id"].(string)
	}

	return func(c *gin.Context) {
		typeName := strings.Split(c.Param("typename"), ".")[0]
		selectedTable, isTable := tableMap[typeName]

		if !isTable {
			log.Infof("%v is not a table", typeName)
			selectedStream, isStream := streamMap[typeName]

			if !isStream {
				c.AbortWithStatus(404)
				return

			} else {
				selectedTable = resource.TableInfo{}
				selectedTable.TableName = selectedStream.StreamName
				selectedTable.Columns = selectedStream.Columns
				selectedTable.Relations = make([]api2go.TableRelation, 0)

			}

		}

		cols := selectedTable.Columns

		//log.Infof("data: %v", selectedTable.Relations)
		actions, err := cruds["world"].GetActionsByType(typeName)

		if err != nil {
			log.Errorf("Failed to get actions by type: %v", err)
		}

		pr := &http.Request{
			Method: "GET",
		}

		pr = pr.WithContext(c.Request.Context())

		params := make(map[string][]string)
		req := api2go.Request{
			PlainRequest: pr,
			QueryParams:  params,
		}

		worldRefId := worldToReferenceId[typeName]

		params["worldName"] = []string{"smd_id"}
		params["world_id"] = []string{worldRefId}

		smdList := make([]map[string]interface{}, 0)

		_, result, err := cruds["smd"].PaginatedFindAll(req)

		if err != nil {
			log.Infof("Failed to get world SMD: %v", err)
		} else {
			models := result.Result().([]*api2go.Api2GoModel)
			for _, m := range models {
				if m.GetAttributes()["__type"].(string) == "smd" {
					smdList = append(smdList, m.GetAttributes())
				}
			}

		}

		res := map[string]interface{}{}

		for _, col := range cols {
			//log.Infof("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue, col.IsForeignKey, col.ForeignKeyData)
			if col.ExcludeFromApi {
				continue
			}

			if col.IsForeignKey && col.ForeignKeyData.DataSource == "self" {
				continue
			}

			res[col.ColumnName] = col
		}

		for _, rel := range selectedTable.Relations {
			//log.Infof("Relation [%v][%v]", selectedTable.TableName, rel.String())

			if rel.GetSubject() == selectedTable.TableName {
				r := "hasMany"
				if rel.GetRelation() == "belongs_to" || rel.GetRelation() == "has_one" {
					r = "hasOne"
				}
				res[rel.GetObjectName()] = NewJsonApiRelation(rel.GetObject(), rel.GetObjectName(), r, "entity")
			} else {
				if rel.GetRelation() == "belongs_to" {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), rel.GetSubjectName(), "hasMany", "entity")
				} else if rel.GetRelation() == "has_one" {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), rel.GetSubjectName(), "hasMany", "entity")
				} else {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), rel.GetSubjectName(), "hasMany", "entity")
				}
			}
		}

		for _, col := range cols {
			//log.Infof("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue)
			if col.ExcludeFromApi {
				continue
			}

			if !col.IsForeignKey || col.ForeignKeyData.DataSource == "self" {
				continue
			}

			//res[col.ColumnName] = NewJsonApiRelation(col.Name, col.ColumnName, "hasOne", col.ColumnType)
		}

		res["__type"] = api2go.ColumnInfo{
			Name:       "type",
			ColumnName: "__type",
			ColumnType: "hidden",
		}

		jsModel := JsModel{
			ColumnModel:           res,
			Actions:               actions,
			StateMachines:         smdList,
			IsStateMachineEnabled: selectedTable.IsStateTrackingEnabled,
		}

		//res["__type"] = "string"
		c.JSON(200, jsModel)
		if true {
			return
		}

		//j, _ := json.Marshal(res)

		//c.String(200, "jsonApi.define('%v', %v)", typeName, string(j))

	}
}

type JsModel struct {
	ColumnModel           map[string]interface{}
	Actions               []resource.Action
	StateMachines         []map[string]interface{}
	IsStateMachineEnabled bool
}

func NewJsonApiRelation(name string, relationName string, relationType string, columnType string) JsonApiRelation {

	return JsonApiRelation{
		Type:       name,
		JsonApi:    relationType,
		ColumnType: columnType,
		ColumnName: relationName,
	}

}

type JsonApiRelation struct {
	JsonApi    string `json:"jsonApi,omitempty"`
	ColumnType string `json:"columnType"`
	Type       string `json:"type,omitempty"`
	ColumnName string `json:"ColumnName"`
}
