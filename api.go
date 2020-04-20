// Copyright 2020 Alexey Krivonogov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"time"

	"eonza/lib"
	"eonza/script"

	"github.com/labstack/echo/v4"
)

type TaskStatus struct {
	TaskID  uint32 `json:"taskid"`
	Status  int    `json:"status"`
	Message string `json:"msg,omitempty"`
	Time    int64  `json:"time,omitempty"`
}

type TaskInfo struct {
	ID         uint32 `json:"id"`
	Status     int    `json:"status"`
	Name       string `json:"name"`
	StartTime  string `json:"start"`
	FinishTime string `json:"finish"`
	UserID     uint32 `json:"userid"`
	Port       int    `json:"port"`
	Message    string `json:"message,omitempty"`
}

type TasksResponse struct {
	List  []TaskInfo `json:"list,omitempty"`
	Error string     `json:"error,omitempty"`
}

func jsonError(c echo.Context, err interface{}) error {
	return c.JSON(http.StatusOK, Response{Error: fmt.Sprint(err)})
}

func jsonSuccess(c echo.Context) error {
	return c.JSON(http.StatusOK, Response{Success: true})
}

func runHandle(c echo.Context) error {
	var (
		item *Script
		ok   bool
	)
	name := c.QueryParam(`name`)
	port, err := getPort()
	if err != nil {
		return jsonError(c, err)
	}
	if item, ok = scripts[name]; !ok {
		return jsonError(c, Lang(`erropen`, name))
	}
	if err = AddHistoryRun(c.(*Auth).User.ID, name); err != nil {
		return jsonError(c, err)
	}
	header := script.Header{
		Name:       name,
		Title:      item.Settings.Title,
		AssetsDir:  cfg.AssetsDir,
		LogDir:     cfg.Log.Dir,
		UserID:     c.(*Auth).User.ID,
		TaskID:     lib.RndNum(),
		ServerPort: cfg.HTTP.Port,
		HTTP: &lib.HTTPConfig{
			Port:  port,
			Open:  true,
			Theme: cfg.HTTP.Theme,
		},
	}
	if err := script.Encode(header); err != nil {
		return jsonError(c, err)
	}
	if err = NewTask(header); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, Response{Success: true})
}

func pingHandle(c echo.Context) error {
	return c.HTML(http.StatusOK, Success)
}

func taskStatusHandle(c echo.Context) error {
	var (
		taskStatus TaskStatus
		err        error
		finish     string
	)

	if err = c.Bind(&taskStatus); err != nil {
		return jsonError(c, err)
	}
	if taskStatus.Time != 0 {
		finish = time.Unix(taskStatus.Time, 0).Format(TimeFormat)
	}
	cmd := WsCmd{
		TaskID:  taskStatus.TaskID,
		Cmd:     WcStatus,
		Status:  taskStatus.Status,
		Message: taskStatus.Message,
		Time:    finish,
	}
	for id, client := range clients {
		err := client.Conn.WriteJSON(cmd)
		if err != nil {
			client.Conn.Close()
			delete(clients, id)
		}
	}
	if taskStatus.Status >= TaskFinished {
		if ptask, ok := tasks[taskStatus.TaskID]; ok {
			ptask.Status = taskStatus.Status
			ptask.Message = taskStatus.Message
			ptask.FinishTime = taskStatus.Time
			if ptask.Status >= TaskFinished {
				if err = SaveTrace(ptask); err != nil {
					return jsonError(c, err)
				}
			}
		}
	}
	return jsonSuccess(c)
}

func tasksHandle(c echo.Context) error {
	list := ListTasks()
	/*	for i := len(list)/2 - 1; i >= 0; i-- {
		opp := len(list) - 1 - i
		list[i], list[opp] = list[opp], list[i]
	}*/
	listInfo := make([]TaskInfo, len(list))
	for i, item := range list {
		var finish string
		if item.FinishTime > 0 {
			finish = time.Unix(item.FinishTime, 0).Format(TimeFormat)
		}
		listInfo[i] = TaskInfo{
			ID:         item.ID,
			Status:     item.Status,
			Name:       item.Name,
			StartTime:  time.Unix(item.StartTime, 0).Format(TimeFormat),
			FinishTime: finish,
			UserID:     item.UserID,
			Port:       item.Port,
			Message:    item.Message,
		}
	}
	return c.JSON(http.StatusOK, &TasksResponse{
		List: listInfo,
	})
}
