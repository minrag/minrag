// Copyright (c) 2025 minrag Authors.
//
// This file is part of minrag.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

// init 初始化函数
func init() {

	//初始化静态文件
	initStaticFS()

	// 异常页面
	h.GET("/error", funcError)

	// 默认首页
	h.GET("/", funcIndex)

	// 查看agent
	h.GET("/agent/:agentID", funcAgentPre)
	h.POST("/agent/sse", funcAgentSSE)
}

// funcIndex 模板首页
func funcIndex(ctx context.Context, c *app.RequestContext) {
	data := warpRequestMap(c)
	cHtml(c, http.StatusOK, "index.html", data)
}

// funcError 错误页面
func funcError(ctx context.Context, c *app.RequestContext) {
	cHtml(c, http.StatusOK, "error.html", nil)
}

// funcAgentPre 智能体
func funcAgentPre(ctx context.Context, c *app.RequestContext) {
	data := warpRequestMap(c)
	agentID := c.Param("agentID")
	data["agentID"] = agentID
	cHtml(c, http.StatusOK, "agent.html", data)
}

// funcAgent 智能体
func funcAgentSSE(ctx context.Context, c *app.RequestContext) {
	input := make(map[string]interface{}, 0)
	c.BindJSON(&input)

	// 设置响应头
	c.SetStatusCode(http.StatusOK)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	writer := resp.NewChunkedBodyWriter(&c.Response, c.GetWriter())
	c.Response.HijackWriter(writer)
	input["c"] = c

	agentIDObj, has := input["agentID"]
	if !has || agentIDObj == nil || agentIDObj.(string) == "" {
		c.WriteString(`data: agentID is empty\n\n`)
		c.WriteString(`data: [DONE]\n\n`)
		c.Flush()
		c.Abort()
		return
	}
	agentID := agentIDObj.(string)
	agent, err := findAgentByID(ctx, agentID)
	if err != nil {
		c.WriteString(`data: agent is empty\n\n`)
		c.WriteString(`data: [DONE]\n\n`)
		c.Flush()
		c.Abort()
		return
	}
	userId := c.GetString(tokenUserId)
	if userId != "" {
		input["roomID"] = userId + "_" + agentID
	} else {
		if agent.Status == 0 {
			c.WriteString(`data: agent is disable\n\n`)
			c.WriteString(`data: [DONE]\n\n`)
			c.Flush()
			c.Abort()
			return
		}
		roomIDObj, has := input["roomID"]
		if !has || roomIDObj.(string) == "" {
			c.WriteString(`data: roomID is empty\n\n`)
			c.WriteString(`data: [DONE]\n\n`)
			c.Flush()
			c.Abort()
			return
		}
		roomIDs := strings.Split(roomIDObj.(string), "_")
		if len(roomIDs) != 2 {
			c.WriteString(`data: roomID is error\n\n`)
			c.WriteString(`data: [DONE]\n\n`)
			c.Flush()
			c.Abort()
			return
		}
		timestampStr := roomIDs[0]
		if len(timestampStr) > 20 || !isNumeric(timestampStr) {
			c.WriteString(`data: roomID is error\n\n`)
			c.WriteString(`data: [DONE]\n\n`)
			c.Flush()
			c.Abort()
			return
		}

	}
	input["knowledgeBaseID"] = agent.KnowledgeBaseID
	pipeline := componentMap[agent.PipelineID]
	pipeline.Run(ctx, input)
	//choice := input["choice"]
	errObj := input[errorKey]
	if errObj != nil {
		c.WriteString(fmt.Sprintf("data: component run is error:%v\n\n", errObj))
		c.Flush()
		c.Abort()
		return
	}
	//fmt.Println(choice)
	//c.JSON(http.StatusOK, ResponseData{StatusCode: 1, Data: choice})
}

// warpRequestMap 包装请求参数为map
func warpRequestMap(c *app.RequestContext) map[string]interface{} {
	data := make(map[string]interface{}, 0)
	//设置用户角色,0是访客,1是管理员
	userType, ok := c.Get(userTypeKey)
	if ok {
		data[userTypeKey] = userType
	} else {
		data[userTypeKey] = 0
	}
	return data
}

func isNumeric(s string) bool {
	matched, _ := regexp.MatchString(`^\d+$`, s)
	return matched
}
