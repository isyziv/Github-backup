package main

import (
	"encoding/json"
	"io/ioutil"
	"time"
)

type repojson struct {
	Repo string
	Time time.Time
}

func readJSONFile(filename string) (map[string]time.Time, error) {
	var data []repojson

	// 读取JSON文件内容
	fileData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// 解析JSON数据
	err = json.Unmarshal(fileData, &data)
	if err != nil {
		return nil, err
	}

	// 创建字典并填充数据
	result := make(map[string]time.Time)
	for _, item := range data {
		result[item.Repo] = item.Time
	}

	return result, nil
}

func writeJSONFile(filename string, repos map[string]time.Time) error {
	var dataList []repojson

	for repo, time := range repos {
		dataList = append(dataList, repojson{
			Repo: repo,
			Time: time,
		})
	}

	jsonData, err := json.Marshal(dataList)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return err
	}

	return nil
}
