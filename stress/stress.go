package stress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lock-free/gopcp"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"
)

func TestApi(apiConfig *ApiConfig) error {
	client := http.Client{
		Timeout: time.Duration(time.Duration(apiConfig.Timeout) * time.Second),
	}

	url := apiConfig.Scheme + "://" + apiConfig.Host + apiConfig.Path
	body, err := GetRequestBody(apiConfig)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(apiConfig.Method, url, bytes.NewBufferString(body))
	if err != nil {
		return err
	}

	for k, v := range apiConfig.Headers {
		request.Header.Set(k, v)
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	// check status code
	if !Contains(apiConfig.Expect.Status, resp.StatusCode) {
		return errors.New(fmt.Sprintf("Status check fail, http status code is %d, expected status are %v", resp.StatusCode, apiConfig.Expect.Status))
	}

	// TODO check header

	// check body
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if apiConfig.Expect.LogBody {
		fmt.Println(string(bodyBytes))
	}

	return CheckBody(bodyBytes, apiConfig.Expect)
}

func CheckBody(bodyBytes []byte, expect ApiExpect) error {
	body := string(bodyBytes)

	// check body
	switch expect.BodyExpectType {
	case EXPECT_EQUAL:
		if body != expect.BodyExp {
			return errors.New(fmt.Sprintf("Body check fail, body is %s", body))
		}

	case EXPECT_REG:
		matched, err := regexp.Match(expect.BodyExp, bodyBytes)
		if err != nil {
			return err
		}
		if !matched {
			return errors.New(fmt.Sprintf("Body check fail, body is %s", body))
		}

	case EXPECT_PCP:
		sandbox := gopcp.GetSandbox(map[string]*gopcp.BoxFunc{
			// return http body as string
			"getJson": gopcp.ToSandboxFun(func(args []interface{}, attachment interface{}, pcpServer *gopcp.PcpServer) (interface{}, error) {
				// return body, nil
				var v interface{}
				err := json.Unmarshal(bodyBytes, &v)
				return v, err
			}),
		})
		pcpServer := gopcp.NewPcpServer(sandbox)
		r, err := pcpServer.Execute(expect.BodyExp, nil)
		if err != nil {
			return err
		}

		matched, ok := r.(bool)
		if !ok {
			return errors.New(fmt.Sprintf("Body check fail, result of pcp is not boolean."))
		}

		if !matched {
			return errors.New(fmt.Sprintf("Body check fail, body is %s", body))
		}
	}

	return nil
}

func StressTestingApi(apiConfig ApiConfig) {
	log.Printf("[api stress] %v", apiConfig)

	coner := GetConer(apiConfig.MaxRunningReq)
	totalRun, errorCount, reqTotalTime := 0, 0, int64(0)
	var statMutex = &sync.Mutex{}

	t1 := time.Now().UnixNano() / int64(time.Millisecond)
	var wg sync.WaitGroup

	for i := 0; i < apiConfig.Duration; i++ {
		for j := 0; j < apiConfig.ReqPerSec; j++ {
			accepted := coner.Run(func() {
				reqStart := time.Now().UnixNano() / int64(time.Millisecond)
				err := TestApi(&apiConfig)
				reqEnd := time.Now().UnixNano() / int64(time.Millisecond)
				reqTime := reqEnd - reqStart

				// do some statistics
				statMutex.Lock()
				reqTotalTime += reqTime
				if err != nil {
					log.Printf("[Errored] %v", err)
					errorCount++
				}
				statMutex.Unlock()

				wg.Done()
			})

			if accepted {
				wg.Add(1)
				totalRun++
			}
		}

		// sleep 1 second
		time.Sleep(1 * time.Second)
	}

	wg.Wait()
	t2 := time.Now().UnixNano() / int64(time.Millisecond)
	// display statistics
	// TODO average http response time
	log.Printf("[api stress result] totalRun = %d, errored = %d, totalTime = %d s, avgReqTime = %d ms",
		totalRun, errorCount, (t2-t1)/1000, reqTotalTime/int64(totalRun))
}

func StressTesting(stressConfig StressConfig, host string, scheme string, only string) {
	for _, apiConfig := range stressConfig.Apis {
		if only == "" || only == apiConfig.Name {
			if host != "" {
				apiConfig.Host = host
			}

			if scheme != "" {
				apiConfig.Scheme = scheme
			}

			StressTestingApi(apiConfig)
		}
	}
}

type StressConfig struct {
	Apis []ApiConfig
}

type ApiConfig struct {
	Name string
	// request part
	Scheme  string // http, https
	Host    string // hostname
	Path    string //path
	Method  string // GET | POST | PUT | DELETE | HEAD | OPTION
	Headers map[string]string
	Body    interface{} // string || JSON object
	Timeout int         // seconds

	// qps controll part
	ReqPerSec     int // requests send per second
	Duration      int // last how long (seconds)
	MaxRunningReq int // max running requests

	// validation part
	Expect ApiExpect
}

func GetRequestBody(apiConfig *ApiConfig) (string, error) {
	switch v := apiConfig.Body.(type) {
	case string:
		return v, nil
	default:
		bs, err := json.Marshal(apiConfig.Body)
		return string(bs), err
	}
}

const EXPECT_EQUAL = "equal"
const EXPECT_REG = "reg" // regular expression
const EXPECT_PCP = "pcp" //pcp expression

type ApiExpect struct {
	Status         []int
	BodyExpectType string
	BodyExp        string
	LogBody        bool
}
