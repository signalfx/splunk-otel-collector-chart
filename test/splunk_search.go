// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func CheckEventsFromSplunk(searchQuery string, startTime string, endTimeOptional ...string) []any {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Splunk Search: checking events in Splunk --")
	user := GetConfigVariable("USER")
	password := GetConfigVariable("PASSWORD")
	baseURL := "https://" + GetConfigVariable("HOST") + ":" + GetConfigVariable("MANAGEMENT_PORT")
	endTime := "now"
	if len(endTimeOptional) > 0 {
		endTime = endTimeOptional[0]
	}
	// post search
	jobID := postSearchRequest(user, password, baseURL, searchQuery, startTime, endTime)
	// wait for search status done == true
	for i := 0; i < 20; i++ { // limit loop - not allowing infinite looping
		logger.Println("Checking Search Status ...")
		isDone := checkSearchJobStatusCode(user, password, baseURL, jobID)
		if isDone == true {
			break
		}
		time.Sleep(1 * time.Second)
	}
	// get events
	results := getSplunkSearchResults(user, password, baseURL, jobID)
	return results
}

func getSplunkSearchResults(user string, password string, baseURL string, jobID string) []any {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	eventURL := fmt.Sprintf("%s/services/search/jobs/%s/events?output_mode=json", baseURL, jobID)
	logger.Println("URL: " + eventURL)
	reqEvents, err := http.NewRequest(http.MethodGet, eventURL, nil)
	if err != nil {
		panic(err)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	reqEvents.SetBasicAuth(user, password)
	respEvents, err := client.Do(reqEvents)
	if err != nil {
		panic(err)
	}
	defer respEvents.Body.Close()
	logger.Println("Send Request: Get query status code: " + strconv.Itoa(respEvents.StatusCode))

	bodyEvents, err := io.ReadAll(respEvents.Body)
	if err != nil {
		panic(err)
	}

	var jsonResponseEvents map[string]any
	err = json.Unmarshal(bodyEvents, &jsonResponseEvents)
	if err != nil {
		panic(err)
	}

	// logger.Println("json Response Events --->")   # debug
	// logger.Println(jsonResponseEvents)			# debug
	results := jsonResponseEvents["results"].([]any)
	return results
}

func checkSearchJobStatusCode(user string, password string, baseURL string, jobID string) any {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	checkEventURL := baseURL + "/services/search/jobs/" + jobID + "?output_mode=json"
	logger.Println("URL: " + checkEventURL)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	checkReqEvents, err := http.NewRequest(http.MethodGet, checkEventURL, nil)
	if err != nil {
		panic(err)
	}
	checkReqEvents.SetBasicAuth(user, password)
	checkResp, err := client.Do(checkReqEvents)
	if err != nil {
		panic(err)
	}
	defer checkResp.Body.Close()
	logger.Println("Send Request: Check query status code: " + strconv.Itoa(checkResp.StatusCode))
	checkBody, err := io.ReadAll(checkResp.Body)
	if err != nil {
		panic(err)
	}
	var checkJSONResponse map[string]any
	err = json.Unmarshal(checkBody, &checkJSONResponse)
	if err != nil {
		panic(err)
	}
	// logger.Println(checkJSONResponse) // debug
	// Print isDone field from response
	isDone := checkJSONResponse["entry"].([]any)[0].(map[string]any)["content"].(map[string]any)["isDone"]
	logger.Printf("Is Splunk Search completed [isDone flag]: %v\n", isDone)
	return isDone
}

func postSearchRequest(user string, password string, baseURL string, searchQuery string, startTime string, endTime string) string {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	searchURL := fmt.Sprintf("%s/services/search/jobs?output_mode=json", baseURL)
	query := searchQuery
	logger.Println("Search query: " + query)
	data := url.Values{}
	data.Set("search", query)
	data.Set("earliest_time", startTime)
	data.Set("latest_time", endTime)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest(http.MethodPost, searchURL, strings.NewReader(data.Encode()))
	if err != nil {
		logger.Printf("Error while preparing POST request")
		panic(err)
	}
	req.SetBasicAuth(user, password)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("Error while executing Http POST request")
		panic(err)
	}
	defer resp.Body.Close()
	logger.Println("Send Request: Post query status code: " + strconv.Itoa(resp.StatusCode))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	var jsonResponse map[string]any
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		panic(err)
	}
	logger.Println(jsonResponse) // debug
	return jsonResponse["sid"].(string)
}
