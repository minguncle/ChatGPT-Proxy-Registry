package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type TypeStatus struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type APIKeyStatus struct {
	Index      int          `json:"index"`
	Key        string       `json:"key"`
	Usage      float64      `json:"usage"`
	Limit      float64      `json:"limit"`
	Remark     string       `json:"remark"`
	TypeStatus []TypeStatus `json:"type_status"`
	BanStatus  bool         `json:"ban_status"`
}

type SysStatus struct {
	ExecutorName string `json:"executor_name"`
	ExecutorAddr string `json:"executor_addr"`
}

type Status struct {
	APIStatus []APIKeyStatus `json:"api_status"`
	SysStatus SysStatus      `json:"sys_status"`
}

type ExecutorTypeEntry struct {
	Key  string `json:"key"`
	Type string `json:"type"`
	Addr string `json:"addr"`
	Name string `json:"name"`
}

type ActiveExecutor struct {
	Addr  string
	Name  string
	Alive bool
}

var (
	//executorsMutex sync.RWMutex
	executors = make(map[string]Status) // 执行器列表
)

var (
	//executorsByTypeMutex sync.RWMutex
	executorsByType  = make(map[string][]ExecutorTypeEntry) // 按类型整理的执行器列表
	lastSentPosition = 0
)

// 存储活跃的执行器
var activeExecutors = make(map[string]*ActiveExecutor)

func organizeExecutorsByType() {
	//executorsByTypeMutex.Lock()
	//defer executorsByTypeMutex.Unlock()

	// 清空现有数据
	executorsByType = make(map[string][]ExecutorTypeEntry)

	for _, status := range executors {
		for _, apiKeyStatus := range status.APIStatus {
			if apiKeyStatus.BanStatus {
				continue
			}
			for _, typeStatus := range apiKeyStatus.TypeStatus {
				if typeStatus.Status != "active" {
					continue
				}
				entry := ExecutorTypeEntry{
					Key:  apiKeyStatus.Key,
					Type: typeStatus.Type,
					Addr: status.SysStatus.ExecutorAddr,
					Name: status.SysStatus.ExecutorName,
				}
				executorsByType[typeStatus.Type] = append(executorsByType[typeStatus.Type], entry)
			}
		}
	}
}

func forwardHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/chat/completions") {
		http.Error(w, "Invalid endpoint", http.StatusBadRequest)
		return
	}

	// 提取请求中的模型类型
	var requestModel struct {
		Model string `json:"model"`
	}
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(requestBody, &requestModel)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 查找对应的执行器
	//executorsByTypeMutex.RLock()
	executorList, ok := executorsByType[requestModel.Model]
	if !ok || len(executorList) == 0 {
		executorList = executorsByType["default"]
		if len(executorList) == 0 {
			http.Error(w, "No available executor", http.StatusInternalServerError)
			return
		}
	}
	//executorsByTypeMutex.RUnlock()
	// 使用简单轮询进行负载均衡
	addr, key := getExecutorUrl(executorList)
	newRequest, err := http.NewRequest(r.Method, addr+r.URL.Path, bytes.NewBuffer(requestBody))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	//log.Printf("create request with body: [%s]", string(requestBody))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	newRequest.Header = r.Header
	var response *http.Response
	for i := 0; i < 3; i++ {
		// 发送新的请求
		client := &http.Client{
			Timeout: 120 * time.Second,
		}
		addr, key = getExecutorUrl(executorList)
		newRequest, err = http.NewRequest(r.Method, addr+r.URL.Path, bytes.NewBuffer(requestBody))
		newRequest.Header = r.Header
		newRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
		response, err = client.Do(newRequest)
		log.Printf("Forwarded request to: [%s] key[%s] model[%s] retry:[%d]/[%d] \n", addr, key, requestModel.Model, i+1, 3)
		if err == nil {
			break
		}
		defer response.Body.Close()
	}
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}

	// 将响应复制回原始响应
	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	// 返回 API 响应主体
	for {
		buff := make([]byte, 32)
		var n int
		n, err = response.Body.Read(buff)
		if err != nil {
			break
		}
		_, err = w.Write(buff[:n])
		if err != nil {
			break
		}
		w.(http.Flusher).Flush()
	}
}

func getExecutorUrl(executorList []ExecutorTypeEntry) (addr string, key string) {
	executor := executorList[lastSentPosition%len(executorList)]
	lastSentPosition++
	// 创建新的请求
	addr = executor.Addr
	key = executor.Key
	if !strings.HasPrefix(addr, "http://") {
		addr = "http://" + addr
	}
	return addr, key
}

// 定期检查执行器是否活跃
func checkExecutors() {
	for {
		time.Sleep(10 * time.Second)
		//executorsMutex.Lock()
		for name, executor := range activeExecutors {
			resp, err := http.Get("http://" + executor.Addr + "/ping")
			if err != nil || resp.StatusCode != http.StatusOK {
				executor.Alive = false
				delete(executors, name)
				delete(executorsByType, name)
				//log.Printf("Executor %s is not active, removing from list", name)
			} else {
				executor.Alive = true
			}
		}
		//executorsMutex.Unlock()
		organizeExecutorsByType()
	}
}

// 处理上报事件
func registerHandler(w http.ResponseWriter, r *http.Request) {
	var status Status
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// 加锁并更新执行器列表
	//executorsMutex.Lock()

	executorName := status.SysStatus.ExecutorName
	if _, exists := executors[executorName]; !exists {
		executors[executorName] = status
		activeExecutors[executorName] = &ActiveExecutor{
			Addr:  status.SysStatus.ExecutorAddr,
			Name:  executorName,
			Alive: true,
		}
		log.Printf("Added executor: %+v\n", status)
	} else {
		for i, apiStatus := range status.APIStatus {
			executors[executorName].APIStatus[i].TypeStatus = apiStatus.TypeStatus
		}
		//log.Printf("Updated executor: %+v\n", status)
	}
	//w.WriteHeader(http.StatusNoContent)
	_, err := w.Write([]byte("ok"))
	if err != nil {
		return
	}
	//executorsMutex.Unlock()
	organizeExecutorsByType()
}
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeFile(w, r, "index.html")
}

func main() {
	//go func() {
	//	for {
	//		time.Sleep(10 * time.Second)
	//		executors, _ := json.Marshal(executors)
	//		executorsByType, _ := json.Marshal(executorsByType)
	//		activeExecutors, _ := json.Marshal(activeExecutors)
	//		log.Println("执行器列表:" + string(executors))
	//		log.Println("类型路由列表:" + string(executorsByType))
	//		log.Println("实例维护列表:" + string(activeExecutors))
	//	}
	//}()
	//go func() {
	//	for {
	//		organizeExecutorsByType()
	//		time.Sleep(2 * time.Second)
	//	}
	//}()
	go checkExecutors()
	http.HandleFunc("/dashboard", dashboardHandler)
	http.HandleFunc("/v1/chat/completions", forwardHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/getExecutors", getExecutorsHandler)
	http.HandleFunc("/toggleExecutor", toggleExecutorHandler)
	http.HandleFunc("/toggleAPIKey", toggleAPIKeyHandler)
	log.Println("server start at port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
