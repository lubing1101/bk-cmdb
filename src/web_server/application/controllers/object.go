/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except 
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and 
 * limitations under the License.
 */
 
package controllers

import (
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/core/cc/api"
	"configcenter/src/common/core/cc/wactions"
	"configcenter/src/common/types"

	"configcenter/src/web_server/application/logics"

	webCommon "configcenter/src/web_server/common"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	//"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tealeg/xlsx"
)

func init() {
	wactions.RegisterNewAction(wactions.Action{Verb: common.HTTPCreate, Path: "/object/owner/:bk_supplier_account/object/:bk_obj_id/import", Params: nil, Handler: ImportObject})
	wactions.RegisterNewAction(wactions.Action{Verb: common.HTTPSelectPost, Path: "/object/owner/:bk_supplier_account/object/:bk_obj_id/export", Params: nil, Handler: ExportObject})
}

var sortFields = []string{
	"bk_property_id",
	"bk_property_name",
	"bk_property_type",
	"option",
	"unit",
	"description",
	"placeholder",
	"editable",
	"isrequired",
	"isreadonly",
	"isonly",
}

var fieldType = map[string]string{
	"bk_property_id":   "文本",
	"bk_property_name": "文本",
	"bk_property_type": "文本",
	"option":           "文本",
	"unit":             "文本",
	"description":      "文本",
	"placeholder":      "文本",
	"editable":         "布尔",
	"isrequired":       "布尔",
	"isreadonly":       "布尔",
	"isonly":           "布尔",
}

var fields = map[string]string{
	"bk_property_id":   "英文名(必填)",
	"bk_property_name": "中文名(必填)",
	"bk_property_type": "数据类型(必填)",
	"option":           "数据配置",
	"unit":             "单位",
	"description":      "描述",
	"placeholder":      "提示",
	"editable":         "是否可编辑",
	"isrequired":       "是否必填",
	"isreadonly":       "是否只读",
	"isonly":           "是否唯一",
}

// ImportObject import object attribute
func ImportObject(c *gin.Context) {
	logics.SetProxyHeader(c)

	file, err := c.FormFile("file")
	if nil != err {
		msg := getReturnStr(CODE_ERROR_UPLOAD_FILE, "未找到上传文件", nil)
		c.String(http.StatusOK, string(msg))
		return
	}

	randNum := rand.Uint32()
	dir := webCommon.ResourcePath + "/import/"
	_, err = os.Stat(dir)
	if nil != err {
		os.MkdirAll(dir, os.ModeDir|os.ModePerm)
	}
	filePath := fmt.Sprintf("%s/importinsts-%d-%d.xlsx", dir, time.Now().UnixNano(), randNum)
	err = c.SaveUploadedFile(file, filePath)
	if nil != err {
		msg := getReturnStr(CODE_ERROR_UPLOAD_FILE, fmt.Sprintf("保存文件失败;error:%s", err.Error()), nil)
		c.String(http.StatusOK, string(msg))
		return
	}
	defer os.Remove(filePath) //delete file
	f, err := xlsx.OpenFile(filePath)
	if nil != err {
		msg := getReturnStr(CODE_ERROR_OPEN_FILE, fmt.Sprintf("保存上传文件;error:%s", err.Error()), nil)
		c.String(http.StatusOK, string(msg))
		return
	}

	attrItems, err := logics.GetImportInsts(f, "", c.Request.Header, 3)
	if 0 == len(attrItems) {
		msg := getReturnStr(CODE_ERROR_OPEN_FILE, "文件内容不能为空", nil)
		c.String(http.StatusOK, string(msg))
		return
	}

	blog.Debug("the object file content:%+v", attrItems)

	cc := api.NewAPIResource()
	apiSite, _ := cc.AddrSrv.GetServer(types.CC_MODULE_APISERVER)
	url := fmt.Sprintf("%s/api/%s/object/batch", apiSite, webCommon.API_VERSION)
	blog.Debug("batch insert insts, the url is %s", url)
	objID := c.Param(common.BKObjIDField)
	params := map[string]interface{}{
		objID: map[string]interface{}{
			"meta": nil,
			"attr": attrItems,
		},
	}

	blog.Debug("import the params(%+v)", params)

	reply, err := httpRequest(url, params, c.Request.Header)
	blog.Debug("return the result:", reply)
	if nil != err {
		c.String(http.StatusOK, err.Error())
	} else {
		c.String(http.StatusOK, reply)
	}

}

func setExcelSubTitle(row *xlsx.Row) *xlsx.Row {
	for _, key := range sortFields {
		cell := row.AddCell()
		cell.Value = key
	}
	return row
}

func setExcelTitle(row *xlsx.Row) *xlsx.Row {
	for _, key := range sortFields {
		cell := row.AddCell()
		cell.Value = fields[key]
		blog.Debug("key:%s value:%v", key, fields[key])
	}
	return row
}

func setExcelTitleType(row *xlsx.Row) *xlsx.Row {
	for _, key := range sortFields {
		cell := row.AddCell()
		cell.Value = fieldType[key]
		blog.Debug("key:%s value:%v", key, fields[key])
	}
	return row
}

func setExcelRow(row *xlsx.Row, item interface{}) *xlsx.Row {

	itemMap, ok := item.(map[string]interface{})
	if !ok {
		blog.Debug("failed to convert to map")
		return row
	}

	// key is the object filed, value is the object filed value
	for _, key := range sortFields {

		cell := row.AddCell()
		//cell.SetValue([]string{"v1", "v2"})
		keyVal, ok := itemMap[key]
		if !ok {
			blog.Warn("not fount the key(%s), skip it", key)
			continue
		}
		blog.Debug("key:%s value:%v", key, keyVal)

		switch t := keyVal.(type) {
		case bool:
			cell.SetBool(t)
		default:
			cell.SetValue(t)
		}
	}

	return row
}

// ExportObject export object
func ExportObject(c *gin.Context) {

	logics.SetProxyHeader(c)
	cc := api.NewAPIResource()

	ownerID := c.Param(common.BKOwnerIDField)
	objID := c.Param(common.BKObjIDField)

	apiSite, _ := cc.AddrSrv.GetServer(types.CC_MODULE_APISERVER)

	// get the all attribute of the object
	arrItems, err := logics.GetObjectData(ownerID, objID, apiSite, c.Request.Header)
	if nil != err {
		blog.Error(err.Error())
		c.String(http.StatusBadGateway, "获取实例数据失败, %s", err.Error())
		return
	}

	blog.Debug("the result:%+v", arrItems)

	// construct the excel file
	var file *xlsx.File
	var sheet *xlsx.Sheet

	file = xlsx.NewFile()

	sheet, err = file.AddSheet(objID)

	if err != nil {
		blog.Error(err.Error())
		c.String(http.StatusBadGateway, "创建EXCEL文件失败，%s", err.Error())
		return
	}

	// set the title
	setExcelTitle(sheet.AddRow())
	setExcelTitleType(sheet.AddRow())
	setExcelSubTitle(sheet.AddRow())

	// add the value
	for _, item := range arrItems {

		innerRow := item.(map[string]interface{})
		blog.Debug("object attribute data :%+v", innerRow)

		// set row value
		setExcelRow(sheet.AddRow(), innerRow)

	}

	dirFileName := fmt.Sprintf("%s/export", webCommon.ResourcePath)
	_, err = os.Stat(dirFileName)
	if nil != err {
		os.MkdirAll(dirFileName, os.ModeDir|os.ModePerm)
	}
	fileName := fmt.Sprintf("%d_%s.xlsx", time.Now().UnixNano(), objID)
	dirFileName = fmt.Sprintf("%s/%s", dirFileName, fileName)
	err = file.Save(dirFileName)
	if err != nil {
		blog.Error("ExportInst save file error:%s", err.Error())
		fmt.Printf(err.Error())
	}
	logics.AddDownExcelHttpHeader(c, fmt.Sprintf("inst_%s.xlsx", objID))
	c.File(dirFileName)

	os.Remove(dirFileName)

}
