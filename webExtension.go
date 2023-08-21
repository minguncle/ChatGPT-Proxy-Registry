package main

import (
	"encoding/json"
	"net/http"
)

func getExecutorsHandler(w http.ResponseWriter, r *http.Request) {
	//executorsMutex.RLock()
	//defer executorsMutex.RUnlock()

	// 转换为数组
	var executorsArray []Status
	for _, status := range executors {
		executorsArray = append(executorsArray, status)
	}

	response, err := json.Marshal(executorsArray)
	if err != nil {
		http.Error(w, "Failed to serialize data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(response)
}
func toggleExecutorHandler(w http.ResponseWriter, r *http.Request) {
	executorName := r.URL.Query().Get("executorName")
	status := r.URL.Query().Get("status")

	//executorsMutex.Lock()
	//defer executorsMutex.Unlock()

	executor, found := executors[executorName]
	if !found {
		http.Error(w, "Executor not found", http.StatusBadRequest)
		return
	}

	// 设置禁用或启用状态
	for i := range executor.APIStatus {
		for j := range executor.APIStatus[i].TypeStatus {
			executor.APIStatus[i].TypeStatus[j].Status = status
		}
	}

	executors[executorName] = executor
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusNoContent)
}
func toggleAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	executorName := r.URL.Query().Get("executorName")
	key := r.URL.Query().Get("key")
	status := r.URL.Query().Get("status")

	//executorsMutex.Lock()
	//defer executorsMutex.Unlock()

	executor, found := executors[executorName]
	if !found {
		http.Error(w, "Executor or key not found", http.StatusBadRequest)
		return
	}
	for i, apiKey := range executor.APIStatus {
		if apiKey.Key == key {
			executor.APIStatus[i].BanStatus = (status == "disable")
		}
	}

	executors[executorName] = executor
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusNoContent)
}
